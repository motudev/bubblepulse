import { ref, watch, onMounted, onUnmounted } from 'vue'
import type { Ref } from 'vue'
import type { DashboardResponse } from '@/types'
import { buildDataForTeam, UPDATE_TEXTS } from '@/demo/data'

/** Provides a reactive DashboardResponse that switches on teamId and ticks live updates every 3–6 s. */
export function useDemoDashboard(activeTeamId: Ref<string | null>): Ref<DashboardResponse> {
  const data = ref<DashboardResponse>(buildDataForTeam(activeTeamId.value))

  watch(activeTeamId, (id) => {
    data.value = buildDataForTeam(id)
  })

  let timer: ReturnType<typeof setTimeout> | null = null

  function scheduleNext(): void {
    const delay = 3000 + Math.random() * 3000
    timer = setTimeout(() => {
      const users = data.value.users
      const targetIdx = Math.floor(Math.random() * users.length)
      const text = UPDATE_TEXTS[Math.floor(Math.random() * UPDATE_TEXTS.length)]
      data.value = {
        ...data.value,
        users: users.map((u, i) =>
          i === targetIdx
            ? { ...u, update_text: text, update_at: new Date().toISOString() }
            : u
        ),
      }
      scheduleNext()
    }, delay)
  }

  onMounted(() => scheduleNext())
  onUnmounted(() => {
    if (timer !== null) clearTimeout(timer)
  })

  return data as Ref<DashboardResponse>
}
