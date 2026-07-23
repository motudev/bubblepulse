import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import { useScopeStore } from './scope'
import { useUserStore } from './user'
import type { User } from '@/types'

// The user store imports the API service; stub it so no request machinery is
// touched — these tests drive the store state directly.
vi.mock('@/services/api', () => ({
  api: {
    getMe: vi.fn().mockRejectedValue(new Error('unauthenticated')),
  },
}))

function makeUser(teamId: string | null): User {
  return {
    id: 1,
    email: 'alice@a.test',
    name: 'Alice',
    role: 'UPDATER',
    team_id: teamId,
    org: { id: 'org-1', name: 'Acme' },
    slack_install_enabled: false,
  }
}

describe('scope store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('defaults to org scope when no session is resolved', () => {
    const scope = useScopeStore()
    expect(scope.scope).toBe('org')
    expect(scope.hasTeam).toBe(false)
    expect(scope.activeTeamId).toBeNull()
  })

  it('defaults to team scope once the user resolves with a team', async () => {
    const userStore = useUserStore()
    const scope = useScopeStore()

    userStore.user = makeUser('team-1')
    await nextTick()

    expect(scope.scope).toBe('team')
    expect(scope.activeTeamId).toBe('team-1')
  })

  it('stays on org scope for a user without a team', async () => {
    const userStore = useUserStore()
    const scope = useScopeStore()

    userStore.user = makeUser(null)
    await nextTick()

    expect(scope.scope).toBe('org')
    expect(scope.activeTeamId).toBeNull()
  })

  it('rejects switching to team scope when the user has no team', async () => {
    const userStore = useUserStore()
    const scope = useScopeStore()
    userStore.user = makeUser(null)
    await nextTick()

    scope.setScope('team')

    expect(scope.scope).toBe('org')
    expect(scope.activeTeamId).toBeNull()
  })

  it('switches between org and team scope for a user with a team', async () => {
    const userStore = useUserStore()
    const scope = useScopeStore()
    userStore.user = makeUser('team-1')
    await nextTick()

    scope.setScope('org')
    expect(scope.scope).toBe('org')
    expect(scope.activeTeamId).toBeNull()

    scope.setScope('team')
    expect(scope.scope).toBe('team')
    expect(scope.activeTeamId).toBe('team-1')
  })

  it('falls back to org scope when the user loses their team', async () => {
    const userStore = useUserStore()
    const scope = useScopeStore()
    userStore.user = makeUser('team-1')
    await nextTick()
    expect(scope.scope).toBe('team')

    userStore.user = makeUser(null)
    await nextTick()

    expect(scope.scope).toBe('org')
    expect(scope.activeTeamId).toBeNull()
  })
})
