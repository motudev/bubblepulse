import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { User } from '@/types'
import { api } from '@/services/api'
import { DEMO_ENABLED } from '@/demo'
import { DEMO_USER } from '@/demo/data'

// Module-level cache prevents concurrent /me calls during simultaneous navigations.
let _sessionPromise: Promise<void> | null = null

export const useUserStore = defineStore('user', () => {
  const user = ref<User | null>(null)

  function ensureSession(): Promise<void> {
    if (DEMO_ENABLED) {
      // In demo mode skip the /me API call. The user is injected by setDemoUser() on sign-in.
      if (!_sessionPromise) _sessionPromise = Promise.resolve()
      return _sessionPromise
    }
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

  function setDemoUser(): void {
    user.value = DEMO_USER
    _sessionPromise = Promise.resolve()
  }

  function clearSession(): void {
    user.value = null
    _sessionPromise = null
  }

  function logout(): void {
    clearSession()
    window.location.href = '/api/auth/logout'
  }

  return { user, ensureSession, clearSession, logout, setDemoUser }
})
