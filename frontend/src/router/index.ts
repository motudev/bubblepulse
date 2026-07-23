import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import type { Role } from '@/types'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresGuest?: boolean
    roles?: Role[]
  }
}
import { useUserStore } from '@/stores/user'
import { useConfigStore } from '@/stores/config'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import AdminSettings from '@/views/AdminSettings.vue'

const routes: RouteRecordRaw[] = [
  { path: '/',          component: LoginView,     meta: { requiresGuest: true } },
  { path: '/dashboard', component: DashboardView, meta: { requiresAuth: true } },
  {
    path: '/admin',
    component: AdminSettings,
    meta: { requiresAuth: true, roles: ['ADMIN', 'TEAM_EDITOR'] },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const userStore = useUserStore()
  const configStore = useConfigStore()
  await Promise.all([userStore.ensureSession(), configStore.fetchConfig()])

  const requiresAuth  = to.matched.some((r) => r.meta.requiresAuth)
  const requiresGuest = to.matched.some((r) => r.meta.requiresGuest)
  const allowedRoles  = to.matched.flatMap((r) => r.meta.roles ?? [])

  if (requiresAuth  && !userStore.user) return '/'
  if (requiresGuest && userStore.user)  return '/dashboard'
  if (allowedRoles.length > 0 && (!userStore.user || !allowedRoles.includes(userStore.user.role))) {
    return '/dashboard'
  }
})
