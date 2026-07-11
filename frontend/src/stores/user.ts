import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { User } from '@/types'
import { api } from '@/services/api'

// Module-level cache prevents concurrent /me calls during simultaneous navigations.
let _sessionPromise: Promise<void> | null = null

export const useUserStore = defineStore('user', () => {
  const user = ref<User | null>(null)

  function ensureSession(): Promise<void> {
    if (!_sessionPromise) {
      _sessionPromise = api
        .getMe()
        .then((u) => {
          user.value = u
        })
        .catch(() => {
          user.value = null
        })
    }
    return _sessionPromise
  }

  function clearSession(): void {
    user.value = null
    _sessionPromise = null
  }

  function logout(): void {
    clearSession()
    window.location.href = '/api/auth/logout'
  }

  return { user, ensureSession, clearSession, logout }
})
