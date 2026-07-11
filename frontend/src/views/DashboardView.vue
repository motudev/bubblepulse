<script setup lang="ts">
import { computed } from 'vue'
import BubbleMap from '@/components/BubbleMap.vue'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const greeting = computed(() => userStore.user ? `Welcome, ${userStore.user.name}` : "Today's Check-ins")

function handleLogout(): void {
  userStore.logout()
}
</script>

<template>
  <main class="dashboard">
    <header class="dashboard__header">
      <h1>{{ greeting }}</h1>
      <button class="dashboard__logout-btn" type="button" @click="handleLogout">
        Sign out
      </button>
    </header>
    <BubbleMap />
  </main>
</template>

<style scoped>
.dashboard {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.dashboard__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 2rem;
}

.dashboard__header h1 {
  font-size: 1.75rem;
  font-weight: 600;
}

.dashboard__logout-btn {
  background: transparent;
  border: none;
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  cursor: pointer;
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  transition:
    color var(--transition-fast),
    background var(--transition-fast);
}

.dashboard__logout-btn:hover {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.dashboard__logout-btn:active {
  background: rgba(255, 255, 255, 0.1);
}

.dashboard__logout-btn:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}
</style>
