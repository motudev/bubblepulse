export type Role = 'ADMIN' | 'TEAM_EDITOR' | 'UPDATER'

export interface OrgInfo {
  id: string
  name: string
}

export interface User {
  id: number
  email: string
  name: string
  role: Role
  team_id: string | null
  org: OrgInfo | null
  slack_install_enabled: boolean
}

export interface Team {
  id: string
  name: string
}

export interface OrgUser {
  id: number
  name: string
  email: string
  role: Role
  team_id: string | null
}

export interface UserEntry {
  id: number
  name: string
  email: string
  update_text: string | null
  update_at: string | null
  topics: string[]
}

export interface DashboardResponse {
  users: UserEntry[]
  topics: string[]
  similarity_matrix: number[][]
}
