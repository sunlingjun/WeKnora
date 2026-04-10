import { defineStore } from 'pinia'
import { ref } from 'vue'
import { validateCASSession } from '@/api/cas'
import { useAuthStore } from './auth'

export const useCASStore = defineStore('cas', () => {
  const isCASLoggedIn = ref<boolean>(false)

  // 判断是否为测试环境（统一的环境判断函数）
  const isTestEnvironment = (): boolean => {
    return import.meta.env.VITE_CAS_ENV === 'test' || window.location.hostname.includes('.t.')
  }

  // 跳转到 CAS 登录页面（提取为独立函数，便于复用）
  const redirectToCASLogin = () => {
    const isTestEnv = isTestEnvironment()
    const casLoginHost = isTestEnv ? 'cas.t.nxin.com' : 'cas.nxin.com'
    const currentUrl = encodeURIComponent(window.location.href)
    const casLoginUrl = `https://${casLoginHost}/cas/login/account?service=${currentUrl}&systemId=103`
    console.log('Redirecting to CAS login:', casLoginUrl)
    window.location.href = casLoginUrl
  }

  // 验证 CAS 会话
  // 简化逻辑：直接调用服务端验证 API，由服务端判断是否需要跳转
  // Cookie 会自动通过 withCredentials: true 携带到服务端
  const validateSession = async () => {
    console.log("开始 CAS 会话验证（直接调用服务端 API）...")
    
    // 防止重复跳转：如果当前已经在 CAS 登录页面，直接返回 false
    if (window.location.href.includes('cas.nxin.com') || window.location.href.includes('cas.t.nxin.com')) {
      console.log("当前已在 CAS 登录页面，避免重复跳转")
      return false
    }

    try {
      // 直接调用服务端验证 API，Cookie 会自动携带
      // 服务端会检查 Cookie 并返回相应的结果
      const response = await validateCASSession()
      console.log("CAS 验证返回：", response)
      
      if (response.code === 0 && response.data) {
        // 登录成功，更新 Auth Store（与原有登录逻辑一致）
        const authStore = useAuthStore()
        
        // 设置用户信息（格式与原有登录 API 一致）
        if (response.data.user) {
          authStore.setUser({
            id: response.data.user.id || '',
            username: response.data.user.username || '',
            email: response.data.user.email || '',
            avatar: response.data.user.avatar,
            tenant_id: String(response.data.tenant?.id || ''),
            can_access_all_tenants: response.data.user.can_access_all_tenants || false,
            created_at: response.data.user.created_at || new Date().toISOString(),
            updated_at: response.data.user.updated_at || new Date().toISOString()
          })
        }
        
        // 设置租户信息（格式与原有登录 API 一致）
        if (response.data.tenant) {
          authStore.setTenant({
            id: String(response.data.tenant.id || ''),
            name: response.data.tenant.name || '',
            api_key: response.data.tenant.api_key || '',
            owner_id: response.data.user?.id || '',
            created_at: response.data.tenant.created_at || new Date().toISOString(),
            updated_at: response.data.tenant.updated_at || new Date().toISOString()
          })
        }
        
        // 设置 Token（用于后续 API 调用）
        if (response.data.token) {
          authStore.setToken(response.data.token)
        }
        if (response.data.refresh_token) {
          authStore.setRefreshToken(response.data.refresh_token)
        }
        
        isCASLoggedIn.value = true
        console.log("CAS 验证成功，Token 已保存")
        return true
      } else {
        // 验证失败（code !== 0），跳转到 CAS 登录页面
        console.log("CAS validation failed with response code:", response.code, "msg:", response.msg)
        redirectToCASLogin()
        return false
      }
    } catch (error) {
      console.error('CAS session validation failed:', error)
      // 验证失败，跳转到 CAS 登录页面
      redirectToCASLogin()
      return false
    }
  }

  // 退出登录
  const logout = () => {
    // 清除本地状态
    isCASLoggedIn.value = false
    
    // 清除 Auth Store 中的信息
    const authStore = useAuthStore()
    authStore.logout()
    
    // 跳转到 CAS 退出页面
    // 环境变量配置：
    // VITE_APP_CAS: CAS 服务器地址（如：https://cas.nxin.com/ 或 https://cas.t.nxin.com/）
    // VITE_APP_APP: 应用地址（如：https://zsk.nxin.com/ 或 https://zsk.t.nxin.com/）
    const casBaseUrl = import.meta.env.VITE_APP_CAS || 'https://cas.t.nxin.com/'
    const appUrl = import.meta.env.VITE_APP_APP || window.location.origin + '/'
    const logoutUrl = `${casBaseUrl}cas/logout?service=${encodeURIComponent(appUrl)}`
    
    // 跳转到 CAS 退出页面
    window.location.href = logoutUrl
  }

  return {
    isCASLoggedIn,
    validateSession,
    logout,
  }
})
