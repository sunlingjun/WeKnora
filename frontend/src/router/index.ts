import { createRouter, createWebHistory } from 'vue-router'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { useAuthStore } from '@/stores/auth'
import { useCASStore } from '@/stores/cas'
import { validateToken } from '@/api/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      redirect: "/platform/knowledge-bases",
    },
    {
      path: "/login",
      name: "login",
      component: () => import("../views/auth/Login.vue"),
      meta: { requiresAuth: false, requiresInit: false }
    },
    {
      path: "/join",
      name: "joinOrganization",
      // 重定向到组织列表页，并将 code 参数转换为 invite_code
      redirect: (to) => {
        const code = to.query.code as string
        return {
          path: '/platform/organizations',
          query: code ? { invite_code: code } : {}
        }
      },
      meta: { requiresInit: true, requiresAuth: true }
    },
    {
      path: "/knowledgeBase",
      name: "home",
      component: () => import("../views/knowledge/KnowledgeBase.vue"),
      meta: { requiresInit: true, requiresAuth: true }
    },
    {
      path: "/platform",
      name: "Platform",
      redirect: "/platform/knowledge-bases",
      component: () => import("../views/platform/index.vue"),
      meta: { requiresInit: true, requiresAuth: true },
      children: [
        {
          path: "tenant",
          redirect: "/platform/settings"
        },
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/settings/Settings.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "knowledge-bases",
          name: "knowledgeBaseList",
          component: () => import("../views/knowledge/KnowledgeBaseList.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "knowledge-bases/:kbId",
          name: "knowledgeBaseDetail",
          component: () => import("../views/knowledge/KnowledgeBase.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "knowledge-search",
          name: "knowledgeSearch",
          component: () => import("../views/knowledge/KnowledgeSearch.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "knowledge-bases/:kbId/members",
          name: "knowledgeBaseMembers",
          component: () => import("../views/knowledge/settings/KnowledgeBaseMembers.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "shared-knowledge-bases",
          name: "sharedKnowledgeBaseSquare",
          component: () => import("../views/knowledge/SharedKnowledgeBaseSquare.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "agents",
          name: "agentList",
          component: () => import("../views/agent/AgentList.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "creatChat",
          name: "globalCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "knowledge-bases/:kbId/creatChat",
          name: "kbCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "chat/:chatid",
          name: "chat",
          component: () => import("../views/chat/index.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
        {
          path: "organizations",
          name: "organizationList",
          component: () => import("../views/organization/OrganizationList.vue"),
          meta: { requiresInit: true, requiresAuth: true }
        },
      ],
    },
  ],
});

// 路由守卫：检查认证状态和系统初始化状态
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  const casStore = useCASStore()
  
  // 如果访问的是登录页面或初始化页面，直接放行（不触发 CAS 验证）
  if (to.meta.requiresAuth === false || to.meta.requiresInit === false) {
    // 如果已登录用户访问登录页面，重定向到知识库列表页面
    if (to.path === '/login' && authStore.isLoggedIn) {
      next('/platform/knowledge-bases')
      return
    }
    // 登录页直接放行，不触发 CAS 验证，避免死循环
    next()
    return
  }

  // 检查用户认证状态（仅对需要认证的路由）
  if (to.meta.requiresAuth !== false) {
    if (!authStore.isLoggedIn) {
      // 未登录，尝试 CAS 验证
      // 注意：如果正在跳转到 CAS 登录页面，不要重复触发
      if (window.location.href.includes('cas.nxin.com') || window.location.href.includes('cas.t.nxin.com')) {
        // 如果当前正在 CAS 登录页面，阻止导航并等待跳转完成
        next(false)
        return
      }
      
      const casValid = await casStore.validateSession()
      if (!casValid) {
        // CAS 验证失败或未登录，validateSession 内部会跳转到 CAS 登录页面
        // 阻止当前导航，因为页面即将跳转到 CAS 登录页面
        next(false)
        return
      }
      // CAS 验证成功，继续路由
      next()
      return
    }

    // 已登录，验证Token有效性（可选，如果Token过期可以重新验证CAS）
    // try {
    //   const { valid } = await validateToken()
    //   if (!valid) {
    //     // Token无效，尝试重新验证CAS
    //     const casValid = await casStore.validateSession()
    //     if (!casValid) {
    //       next(false)
    //       return
    //     }
    //   }
    // } catch (error) {
    //   console.error('Token验证失败:', error)
    //   // Token验证失败，尝试重新验证CAS
    //   const casValid = await casStore.validateSession()
    //   if (!casValid) {
    //     next(false)
    //     return
    //   }
    // }
  }

  next()
});

export default router
