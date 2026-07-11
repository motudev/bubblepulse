import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresGuest?: boolean
  }
}
import { useUserStore } from '@/stores/user'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'

const routes: RouteRecordRaw[] = [
  { path: '/',          component: LoginView,     meta: { requiresGuest: true } },
  { path: '/dashboard', component: DashboardView, meta: { requiresAuth: true } },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const userStore = useUserStore()
  await userStore.ensureSession()

  const requiresAuth  = to.matched.some((r) => r.meta.requiresAuth)
  const requiresGuest = to.matched.some((r) => r.meta.requiresGuest)

  if (requiresAuth  && !userStore.user) return '/'
  if (requiresGuest && userStore.user)  return '/dashboard'
})
