<script setup lang="ts">
import { useRouter } from 'vue-router'
import DashboardPreview from '@/views/DashboardPreview.vue'
import { DEMO_ENABLED } from '@/demo'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

async function handleSlackLogin(): Promise<void> {
  if (DEMO_ENABLED) {
    userStore.setDemoUser()
    await router.push('/dashboard')
    return
  }
  window.location.href = '/api/auth/login'
}
</script>

<template>
  <div class="login">
    <!-- Full-bleed preview backdrop — non-interactive teaser of the real app -->
    <div class="login__backdrop" aria-hidden="true">
      <DashboardPreview />
    </div>

    <!-- Dark veil + blur so the card reads clearly over the graph -->
    <div class="login__veil" aria-hidden="true" />

    <!-- Centered login card -->
    <div class="login__card">
      <div class="login__brand">
        <div class="login__logo-mark" aria-hidden="true">
          <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" width="48" height="48">
            <circle cx="20" cy="20" r="18" fill="url(#bp-grad)" />
            <circle cx="13" cy="20" r="4" fill="rgba(255,255,255,0.9)" />
            <circle cx="27" cy="20" r="4" fill="rgba(255,255,255,0.9)" />
            <line x1="17" y1="20" x2="23" y2="20" stroke="rgba(255,255,255,0.7)" stroke-width="2" />
            <defs>
              <linearGradient id="bp-grad" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
                <stop stop-color="#6c63ff" />
                <stop offset="1" stop-color="#a29bfe" />
              </linearGradient>
            </defs>
          </svg>
        </div>
        <h1 class="login__title">BubblePulse</h1>
        <p class="login__tagline">See who's blocking who, before it blocks you.</p>
      </div>

      <div class="login__cta">
        <button class="login__slack-btn" type="button" @click="handleSlackLogin">
          <!-- Official Slack 4-colour logo -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 122.8 122.8"
            width="22"
            height="22"
            aria-hidden="true"
            focusable="false"
          >
            <path d="M25.8 77.6c0 7.1-5.8 12.9-12.9 12.9S0 84.7 0 77.6s5.8-12.9 12.9-12.9h12.9v12.9z" fill="#E01E5A"/>
            <path d="M32.3 77.6c0-7.1 5.8-12.9 12.9-12.9s12.9 5.8 12.9 12.9v32.3c0 7.1-5.8 12.9-12.9 12.9s-12.9-5.8-12.9-12.9V77.6z" fill="#E01E5A"/>
            <path d="M45.2 25.8c-7.1 0-12.9-5.8-12.9-12.9S38.1 0 45.2 0s12.9 5.8 12.9 12.9v12.9H45.2z" fill="#2EB67D"/>
            <path d="M45.2 32.3c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9H12.9C5.8 58.1 0 52.3 0 45.2s5.8-12.9 12.9-12.9h32.3z" fill="#2EB67D"/>
            <path d="M97 45.2c0-7.1 5.8-12.9 12.9-12.9s12.9 5.8 12.9 12.9-5.8 12.9-12.9 12.9H97V45.2z" fill="#ECB22E"/>
            <path d="M90.5 45.2c0 7.1-5.8 12.9-12.9 12.9s-12.9-5.8-12.9-12.9V12.9C64.7 5.8 70.5 0 77.6 0s12.9 5.8 12.9 12.9v32.3z" fill="#ECB22E"/>
            <path d="M77.6 97c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9-12.9-5.8-12.9-12.9V97h12.9z" fill="#36C5F0"/>
            <path d="M77.6 90.5c-7.1 0-12.9-5.8-12.9-12.9s5.8-12.9 12.9-12.9h32.3c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9H77.6z" fill="#36C5F0"/>
          </svg>
          Sign in with Slack
        </button>
      </div>

      <p class="login__footer">By signing in you agree to our Terms of Service.</p>
    </div>
  </div>
</template>

<style scoped>
.login {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

/* ── Backdrop: the full-bleed preview graph ── */
.login__backdrop {
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  /* Scale up very slightly so the graph fills edge-to-edge on all ratios */
  transform: scale(1.02);
  transform-origin: center;
}

/* ── Veil: dark tint + soft blur over the graph ── */
.login__veil {
  position: absolute;
  inset: 0;
  z-index: 1;
  background: rgba(8, 6, 22, 0.68);
  backdrop-filter: blur(3px);
  -webkit-backdrop-filter: blur(3px);
}

/* ── Card: glass panel floating in the center ── */
.login__card {
  position: relative;
  z-index: 2;
  width: 100%;
  max-width: 400px;
  margin: var(--space-8);
  padding: var(--space-12) var(--space-8);
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg), 0 0 0 1px rgba(255, 255, 255, 0.07);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-8);
  text-align: center;
  opacity: 0;
  animation: fadeSlideUp 0.65s ease forwards;
  animation-delay: 100ms;
}

/* ── Brand ── */
.login__brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
}

.login__logo-mark {
  filter: drop-shadow(0 4px 16px rgba(108, 99, 255, 0.5));
}

.login__title {
  font-size: var(--font-size-3xl);
  font-weight: 900;
  color: var(--color-text-primary);
  letter-spacing: -0.02em;
  line-height: 1.1;
}

.login__tagline {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  max-width: 26ch;
  line-height: 1.5;
}

/* ── CTA ── */
.login__cta {
  width: 100%;
  display: flex;
  justify-content: center;
}

.login__slack-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--space-3);
  background: var(--color-btn-slack-bg);
  color: var(--color-btn-slack-text);
  border: none;
  border-radius: var(--radius-md);
  padding: var(--space-3) var(--space-6);
  font-family: var(--font-sans);
  font-size: var(--font-size-base);
  font-weight: 700;
  cursor: pointer;
  box-shadow: var(--shadow-btn);
  transition:
    box-shadow var(--transition-base),
    transform var(--transition-fast);
  white-space: nowrap;
}

.login__slack-btn:hover {
  box-shadow: var(--shadow-btn-hover);
  transform: translateY(-2px);
}

.login__slack-btn:active {
  transform: translateY(0);
  box-shadow: var(--shadow-btn);
}

.login__slack-btn:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 3px;
}

/* ── Footer ── */
.login__footer {
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
}
</style>
