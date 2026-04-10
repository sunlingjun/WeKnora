import { get } from '@/utils/request'

export interface CASUserInfo {
  idStr: string
  id: string
  unionId: string
  loginName: string
  realName: string
  nickName: string
  email: string
  mobilePhone: string
  image: string
  gender: number
  province: number
  city: number
  country: number
  areaName: string
  nationalIdentifier: string
  integrity: number
  grade: number
  authStatus: Record<string, any>
  phoneSigned: boolean
}

export interface CASValidateResponse {
  code: number
  data?: {
    user: any
    tenant: any
    token: string
    refresh_token: string
  }
  msg: string
}

// 验证 CAS 会话
// 注意：_cas_sid 和 _cas_uid 通过 Cookie 自动传递，无需手动设置
// withCredentials 已在 axios 实例中配置
export function validateCASSession(): Promise<CASValidateResponse> {
  return get('/api/v1/cas/validate') as unknown as Promise<CASValidateResponse>
}

// 退出 CAS 登录
export function logoutCAS(): void {
  // 环境变量配置：
  // VITE_APP_CAS: CAS 服务器地址（如：https://cas.nxin.com/ 或 https://cas.t.nxin.com/）
  // VITE_APP_APP: 应用地址（如：https://zsk.nxin.com/ 或 https://zsk.t.nxin.com/）
  const casBaseUrl = import.meta.env.VITE_APP_CAS || 'https://cas.t.nxin.com/'
  const appUrl = import.meta.env.VITE_APP_APP || window.location.origin + '/'
  const logoutUrl = `${casBaseUrl}cas/logout?service=${encodeURIComponent(appUrl)}`
  
  // 跳转到 CAS 退出页面
  window.location.href = logoutUrl
}
