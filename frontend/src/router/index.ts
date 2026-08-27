import { createRouter, createWebHistory } from 'vue-router'
import { CODES, requireAny, resetPermissions, setPermissions } from '@/permissions'
import { authApi } from '@/api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/HomeView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/docs/:id',
      name: 'doc',
      component: () => import('@/views/DocView.vue'),
      props: true,
    },
    {
      path: '/forbidden',
      name: 'forbidden',
      component: () => import('@/views/ForbiddenView.vue'),
    },
    {
      path: '/docs/:id/edit',
      name: 'doc-edit',
      meta: { requiresAuth: true, anyPermissions: [CODES.document_update] },
      // 懒加载：编辑器重依赖按路由拆分 chunk
      component: () => import('@/views/EditView.vue'),
      props: true,
    },
    {
      path: '/search',
      name: 'search',
      component: () => import('@/views/SearchView.vue'),
    },
    {
      path: '/trash',
      name: 'trash',
      meta: { requiresAuth: true, anyPermissions: [CODES.document_delete] },
      // 懒加载：回收站为低频页面
      component: () => import('@/views/TrashView.vue'),
    },
    {
      path: '/admin',
      name: 'admin',
      meta: {
        requiresAuth: true,
        anyPermissions: [CODES.settings_manage, CODES.user_list, CODES.dashboard_read, CODES.backup_manage],
      },
      component: () => import('@/views/AdminView.vue'),
    },
    {
      path: '/settings/tokens',
      name: 'tokens',
      meta: { requiresAuth: true, anyPermissions: [CODES.token_manage_own] },
      component: () => import('@/views/TokensView.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue'),
    },
  ],
})

// 启动时拉取 /users/me 填充权限码；未登录则清空。
router.beforeEach(async (to) => {
  let authenticated = false
  try {
    const me = await authApi.me()
    setPermissions(me.permissions)
    authenticated = true
  } catch {
    resetPermissions()
  }
  if (to.meta.requiresAuth && !authenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  const permissions = to.meta.anyPermissions as string[] | undefined
  if (permissions && (!authenticated || !requireAny(permissions))) {
    return { name: 'forbidden' }
  }
  return true
})

export default router
