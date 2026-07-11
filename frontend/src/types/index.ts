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
