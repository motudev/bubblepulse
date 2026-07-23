<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const route = useRoute()

const STORAGE_KEY = 'bp_slack_installed'

const dismissed = ref(localStorage.getItem(STORAGE_KEY) === '1')

const show = computed(
  () => userStore.user?.role === 'ADMIN' && userStore.user?.slack_install_enabled === true && !dismissed.value
)

function dismiss(): void {
  localStorage.setItem(STORAGE_KEY, '1')
  dismissed.value = true
}

function handleKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') dismiss()
}

onMounted(() => {
  if (route.query.slack_installed === '1') {
    dismiss()
  }
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="slack-modal__backdrop" @click.self="dismiss">
      <div
        class="slack-modal__card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="slack-modal-title"
      >
        <button class="slack-modal__close" aria-label="Dismiss" @click="dismiss">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 16 16"
            aria-hidden="true"
            focusable="false"
          >
            <path
              fill="currentColor"
              d="M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.75.75 0 1 1 1.06 1.06L9.06 8l3.22 3.22a.75.75 0 1 1-1.06 1.06L8 9.06l-3.22 3.22a.75.75 0 0 1-1.06-1.06L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z"
            />
          </svg>
        </button>

        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 127 127"
          width="40"
          height="40"
          class="slack-modal__logo"
          aria-hidden="true"
          focusable="false"
        >
          <path
            fill="#E01E5A"
            d="M27.2 80c0 7.3-5.9 13.2-13.2 13.2C6.7 93.2.8 87.3.8 80c0-7.3 5.9-13.2 13.2-13.2h13.2V80zm6.6 0c0-7.3 5.9-13.2 13.2-13.2 7.3 0 13.2 5.9 13.2 13.2v33c0 7.3-5.9 13.2-13.2 13.2-7.3 0-13.2-5.9-13.2-13.2V80z"
          />
          <path
            fill="#36C5F0"
            d="M47 27c-7.3 0-13.2-5.9-13.2-13.2C33.8 6.5 39.7.6 47 .6c7.3 0 13.2 5.9 13.2 13.2V27H47zm0 6.7c7.3 0 13.2 5.9 13.2 13.2 0 7.3-5.9 13.2-13.2 13.2H13.9C6.6 60.1.7 54.2.7 46.9c0-7.3 5.9-13.2 13.2-13.2H47z"
          />
          <path
            fill="#2EB67D"
            d="M99.9 46.9c0-7.3 5.9-13.2 13.2-13.2 7.3 0 13.2 5.9 13.2 13.2 0 7.3-5.9 13.2-13.2 13.2H99.9V46.9zm-6.6 0c0 7.3-5.9 13.2-13.2 13.2-7.3 0-13.2-5.9-13.2-13.2V13.8C66.9 6.5 72.8.6 80.1.6c7.3 0 13.2 5.9 13.2 13.2v33.1z"
          />
          <path
            fill="#ECB22E"
            d="M80.1 99.8c7.3 0 13.2 5.9 13.2 13.2 0 7.3-5.9 13.2-13.2 13.2-7.3 0-13.2-5.9-13.2-13.2V99.8h13.2zm0-6.6c-7.3 0-13.2-5.9-13.2-13.2 0-7.3 5.9-13.2 13.2-13.2h33.1c7.3 0 13.2 5.9 13.2 13.2 0 7.3-5.9 13.2-13.2 13.2H80.1z"
          />
        </svg>

        <h2 id="slack-modal-title" class="slack-modal__title">
          Connect your Slack workspace
        </h2>

        <p class="slack-modal__body">
          BubblePulse collects daily updates via Slack DMs. Install the bot so your team can start sending check-ins.
        </p>

        <a href="/api/slack/install" class="slack-modal__cta">
          <img
            alt="Add to Slack"
            height="40"
            width="139"
            src="https://platform.slack-edge.com/img/add_to_slack.png"
            srcset="
              https://platform.slack-edge.com/img/add_to_slack.png    1x,
              https://platform.slack-edge.com/img/add_to_slack@2x.png 2x
            "
          />
        </a>

        <p class="slack-modal__hint">
          You can also do this later from
          <RouterLink to="/admin" class="slack-modal__hint-link">Admin settings</RouterLink>.
        </p>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.slack-modal__backdrop {
  position: fixed;
  inset: 0;
  background: rgba(8, 6, 22, 0.72);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
}

.slack-modal__card {
  position: relative;
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-panel), 0 0 0 1px rgba(255, 255, 255, 0.07);
  padding: var(--space-8);
  max-width: 420px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-4);
  text-align: center;
  animation: fadeSlideUp 0.65s ease forwards;
  animation-delay: 100ms;
  opacity: 0;
}

.slack-modal__close {
  position: absolute;
  top: var(--space-4);
  right: var(--space-4);
  background: none;
  border: none;
  cursor: pointer;
  color: var(--color-text-secondary);
  padding: var(--space-1);
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: var(--transition-fast);
}

.slack-modal__close:hover {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.slack-modal__close:active {
  background: rgba(255, 255, 255, 0.1);
}

.slack-modal__close:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}

.slack-modal__logo {
  margin-bottom: var(--space-2);
}

.slack-modal__title {
  font-family: var(--font-sans);
  font-size: var(--font-size-xl);
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0;
  line-height: 1.3;
}

.slack-modal__body {
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  margin: 0;
  line-height: 1.6;
}

.slack-modal__cta {
  display: inline-block;
  margin-top: var(--space-2);
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
}

.slack-modal__cta img {
  display: block;
  max-width: 100%;
}

.slack-modal__cta:hover {
  opacity: 0.85;
  transform: translateY(-1px);
}

.slack-modal__cta:active {
  opacity: 1;
  transform: translateY(0);
}

.slack-modal__cta:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 4px;
}

.slack-modal__hint {
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
  margin: 0;
}

.slack-modal__hint-link {
  color: var(--color-text-secondary);
  text-underline-offset: 2px;
  transition: color var(--transition-fast);
}

.slack-modal__hint-link:hover {
  color: var(--color-text-primary);
}

.slack-modal__hint-link:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
  border-radius: 2px;
}
</style>
