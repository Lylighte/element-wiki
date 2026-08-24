import { createRouter, createWebHistory } from 'vue-router'
import { resetPermissions, setPermissions } from '@/permissions'
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
      path: '/docs/:id/edit',
      name: 'doc-edit',
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
      // 懒加载：回收站为低频页面
      component: () => import('@/views/TrashView.vue'),
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('@/views/AdminView.vue'),
    },
    {
      path: '/settings/tokens',
      name: 'tokens',
      component: () => import('@/views/TokensView.vue'),
    },
  ],
})

// 启动时拉取 /users/me 填充权限码；未登录则清空。
router.beforeEach(async (_to, _from, next) => {
  try {
    const me = await authApi.me()
    setPermissions(me.permissions)
  } catch {
    resetPermissions()
  }
  next()
})

export default router
