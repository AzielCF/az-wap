import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { isPublic: true }
    },
    {
      path: '/workspaces',
      name: 'workspaces',
      component: () => import('../views/WorkspacesView.vue'),
    },
    {
      path: '/workspaces/:id',
      name: 'workspace-detail',
      component: () => import('../views/WorkspaceDetailView.vue'),
    },
    {
      path: '/health',
      name: 'health',
      component: () => import('../views/HealthView.vue'),
    },
    {
      path: '/monitoring',
      name: 'monitoring',
      component: () => import('../views/MonitoringView.vue'),
    },
    {
      path: '/bots',
      name: 'bots',
      component: () => import('../views/BotsView.vue'),
    },
    {
      path: '/mcp',
      name: 'mcp',
      component: () => import('../views/MCPView.vue'),
    },
    {
      path: '/credentials',
      name: 'credentials',
      component: () => import('../views/CredentialsView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
    },
  ],
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('api_token')
  if (!token && !to.meta.isPublic) {
    next({ name: 'login' })
  } else {
    next()
  }
})

export default router
