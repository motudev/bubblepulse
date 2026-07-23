import type { User, DashboardResponse, Team, OrgUser, OrgInfo, Role } from '@/types'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

// skipUnauthorized=true suppresses the redirect-to-login behaviour for endpoints
// where a 401 is an expected (non-error) response (e.g. session probe on mount).
async function request<T>(path: string, init?: RequestInit, skipUnauthorized = false): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    if (res.status === 401 && !skipUnauthorized) {
      window.location.replace('/')
      throw new Error('Unauthorized')
    }
    throw new Error(`API error ${res.status}: ${await res.text()}`)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export interface UpdateUserPayload {
  team_id?: string | null
  role?: Role
}

export interface AppConfig {
  slack_oidc: boolean
}

export const api = {
  getHealth: (): Promise<{ status: string }> => request('/api/v1/health'),
  getConfig: (): Promise<AppConfig> => request<AppConfig>('/api/v1/config', undefined, true),
  getMe: (): Promise<User> => request<User>('/api/v1/me', undefined, true),
  getDashboard: (teamId?: string | null): Promise<DashboardResponse> =>
    request<DashboardResponse>(
      teamId ? `/api/dashboard?team_id=${encodeURIComponent(teamId)}` : '/api/dashboard'
    ),

  getTeams: (): Promise<Team[]> => request<Team[]>('/api/v1/teams'),
  createTeam: (name: string): Promise<Team> =>
    request<Team>('/api/v1/teams', { method: 'POST', body: JSON.stringify({ name }) }),
  updateTeam: (id: string, name: string): Promise<Team> =>
    request<Team>(`/api/v1/teams/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) }),
  deleteTeam: (id: string): Promise<void> =>
    request<void>(`/api/v1/teams/${id}`, { method: 'DELETE' }),

  getOrgUsers: (): Promise<OrgUser[]> => request<OrgUser[]>('/api/v1/users'),
  updateUser: (id: number, payload: UpdateUserPayload): Promise<OrgUser> =>
    request<OrgUser>(`/api/v1/users/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),

  renameOrg: (name: string): Promise<OrgInfo> =>
    request<OrgInfo>('/api/v1/org', { method: 'PATCH', body: JSON.stringify({ name }) }),
}
