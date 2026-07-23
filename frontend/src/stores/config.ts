import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/services/api'

export const useConfigStore = defineStore('config', () => {
  const slackOidc = ref(false)
  let fetched = false

  async function fetchConfig(): Promise<void> {
    if (fetched) return
    fetched = true
    try {
      const data = await api.getConfig()
      slackOidc.value = data.slack_oidc
    } catch {
      // network failure — default to generic (non-Slack) UI
    }
  }

  return { slackOidc, fetchConfig }
})
