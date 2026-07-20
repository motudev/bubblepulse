<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useScopeStore } from '@/stores/scope'

const userStore = useUserStore()
const scopeStore = useScopeStore()

const canManage = computed(() => {
  const role = userStore.user?.role
  return role === 'ADMIN' || role === 'TEAM_EDITOR'
})

const orgLabel = computed(() => {
  const name = userStore.user?.org?.name
  return name && name.trim() !== '' ? name : 'BubblePulse'
})

const greeting = computed(() =>
  userStore.user ? `Welcome, ${userStore.user.name}` : "Today's Check-ins",
)

function handleLogout(): void {
  userStore.logout()
}
</script>

<template>
  <nav class="app-nav" aria-label="Main navigation">
    <RouterLink class="app-nav__brand" to="/dashboard">
      <span class="app-nav__brand-dot" aria-hidden="true"></span>
      {{ orgLabel }}
    </RouterLink>

    <div class="app-nav__center">
      <span class="app-nav__greeting">{{ greeting }}</span>
      <div
        v-if="scopeStore.hasTeam"
        class="app-nav__scope"
        role="group"
        aria-label="Dashboard scope"
      >
        <button
          class="app-nav__scope-btn"
          :class="{ 'app-nav__scope-btn--active': scopeStore.scope === 'org' }"
          type="button"
          :aria-pressed="scopeStore.scope === 'org'"
          @click="scopeStore.setScope('org')"
        >
          Entire Organization
        </button>
        <button
          class="app-nav__scope-btn"
          :class="{ 'app-nav__scope-btn--active': scopeStore.scope === 'team' }"
          type="button"
          :aria-pressed="scopeStore.scope === 'team'"
          @click="scopeStore.setScope('team')"
        >
          My Team
        </button>
      </div>
    </div>

    <div class="app-nav__actions">
      <RouterLink v-if="canManage" class="app-nav__link" to="/admin">Admin</RouterLink>
      <span class="app-nav__user">{{ userStore.user?.name }}</span>
      <button class="app-nav__logout" type="button" @click="handleLogout">Sign out</button>
    </div>
  </nav>
</template>

<style scoped>
.app-nav {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: var(--space-6);
  padding: var(--space-3) var(--space-8);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.app-nav__brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 900;
  color: var(--color-text-primary);
  text-decoration: none;
}

.app-nav__brand-dot {
  width: 10px;
  height: 10px;
  border-radius: var(--radius-full);
  background: var(--color-brand);
  box-shadow: 0 0 8px var(--color-brand);
}

.app-nav__center {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
}

.app-nav__greeting {
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  color: var(--color-text-secondary);
}

.app-nav__scope {
  display: flex;
  padding: 2px;
  border-radius: var(--radius-full);
  background: rgba(255, 255, 255, 0.06);
}

.app-nav__scope-btn {
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-full);
  cursor: pointer;
  transition:
    color var(--transition-fast),
    background var(--transition-fast);
}

.app-nav__scope-btn--active {
  color: var(--color-text-primary);
  background: var(--color-brand);
}

.app-nav__scope-btn:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}

.app-nav__actions {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  justify-self: end;
}

.app-nav__link {
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  text-decoration: none;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  transition:
    color var(--transition-fast),
    background var(--transition-fast);
}

.app-nav__link:hover,
.app-nav__link.router-link-active {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.app-nav__user {
  color: var(--color-text-muted);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
}

.app-nav__logout {
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

.app-nav__logout:hover {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.app-nav__logout:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}
</style>
