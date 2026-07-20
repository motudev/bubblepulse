import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { useUserStore } from '@/stores/user'

export type DashboardScope = 'org' | 'team'

/**
 * Holds the dashboard viewing scope: the entire organization or the current
 * user's team. Defaults to the user's team when they have one.
 */
export const useScopeStore = defineStore('scope', () => {
  const userStore = useUserStore()
  const scope = ref<DashboardScope>('org')

  // Default to "My Team" once the session resolves with a team assignment;
  // fall back to the org view whenever the user loses their team.
  watch(
    () => userStore.user?.team_id ?? null,
    (teamId) => {
      scope.value = teamId ? 'team' : 'org'
    },
    { immediate: true }
  )

  const hasTeam = computed(() => Boolean(userStore.user?.team_id))

  /** The team_id the dashboard should be filtered by; null = whole organization. */
  const activeTeamId = computed<string | null>(() =>
    scope.value === 'team' ? (userStore.user?.team_id ?? null) : null
  )

  function setScope(next: DashboardScope): void {
    if (next === 'team' && !hasTeam.value) return
    scope.value = next
  }

  return { scope, hasTeam, activeTeamId, setScope }
})
