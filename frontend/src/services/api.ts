import type { User } from '@/types'

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
  return res.json() as Promise<T>
}

export const api = {
  getHealth: (): Promise<{ status: string }> => request('/api/v1/health'),
  getMe: (): Promise<User> => request<User>('/api/v1/me', undefined, true),
}
