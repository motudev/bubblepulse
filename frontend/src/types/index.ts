export interface User {
  id: number
  email: string
  name: string
}

export interface DashboardEntry {
  id: number
  name: string
  email: string
  update_text: string | null
  update_at: string | null
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
