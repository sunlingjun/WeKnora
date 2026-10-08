// src/utils/request.ts
import axios from "axios";
import { generateRandomString, MAX_FILE_SIZE_MB, MAX_SKILL_BUNDLE_SIZE_MB } from "./index";
import i18n from '@/i18n'
import { getApiBaseUrl } from './api-base';
import { applyStoredAuthHeaders, needsCasIdentityReconcile } from './casSession'
import { isSkillBundleUploadUrl } from './uploadLimit';
import { isTimeoutError, uploadTimeoutMs } from './requestTimeouts';
import {
  forceReloginRedirect,
  refreshAccessTokenShared,
} from './authRefresh';

export { forceReloginRedirect, refreshAccessTokenShared };

const t = (key: string) => i18n.global.t(key)

// API基础URL
// API基础URL（NXIN 域名逻辑见 getApiBaseUrl）
const BASE_URL = getApiBaseUrl();

/**
 * Response payload augmented with the HTTP status code.
 *
 * `$httpStatus` lets callers distinguish outcomes that share a success shape.
 * Defined as a non-enumerable property, so it stays invisible to object spread,
 * JSON.stringify and Object.keys and never leaks into downstream payloads.
 *
 * Guaranteed only for JSON responses (objects/arrays). Blob, string and SSE
 * stream responses do not carry it at runtime, so only read `$httpStatus`
 * when the payload is known to be an object.
 */
export type WithStatus<T> = T & {
  /** HTTP status code of the response. Non-enumerable. See {@link WithStatus}. */
  readonly $httpStatus: number
};

const HTTP_STATUS_KEY = '$httpStatus';

/**
 * Attach the non-enumerable `$httpStatus` property to a response payload
 * in place and return it. Primitives pass through untouched.
 * See {@link WithStatus} for where the property is guaranteed.
 */
function withHttpStatus<T>(data: T, status: number): T {
  if (data !== null && typeof data === 'object') {
    Object.defineProperty(data, HTTP_STATUS_KEY, {
      value: status,
      enumerable: false,
      configurable: true,
      writable: false,
    });
  }
  return data;
}

// 创建Axios实例
const instance = axios.create({
  baseURL: BASE_URL, // 使用配置的API基础URL
  timeout: 30000, // 请求超时时间
  headers: {
    "Content-Type": "application/json",
    "X-Request-ID": `${generateRandomString(12)}`,
  },
  withCredentials: true, // 允许携带 Cookie（用于 CAS 认证）
});

// 获取当前用户语言（用于 Accept-Language header）
export function getCurrentLanguage(): string {
  return i18n.global.locale?.value || localStorage.getItem('locale') || 'zh-CN'
}


instance.interceptors.request.use(
  (config) => {
    const existingAuth = config.headers?.Authorization ?? config.headers?.authorization;
    const isEmbedAuth = typeof existingAuth === 'string' && existingAuth.startsWith('Embed ');
    const isEmbedPath = typeof config.url === 'string' && config.url.includes('/api/v1/embed/');

    // 嵌入渠道使用 Embed token；勿用本地 JWT 覆盖（否则调试页会 401）。
    // NXIN：本页 CAS reconcile 完成前 applyStoredAuthHeaders 不会附带 leftover JWT。
    if (!isEmbedAuth) {
      applyStoredAuthHeaders(config.headers as Record<string, string>, { includeTenant: !isEmbedPath })
    }

    config.headers["Accept-Language"] = getCurrentLanguage();
    config.headers["X-Request-ID"] = `${generateRandomString(12)}`;
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Share-link endpoints (/auth/invitations/lookup, /auth/register-by-invite)
// are reachable by anonymous users opening an invite link. A 401 from these
// must surface to the page (e.g. expired token), not trigger the
// refresh-then-redirect-to-login flow (issue #1617). '/auth/register' already
// covers '/auth/register-by-invite' via substring match.
const PUBLIC_AUTH_PATHS = ['/auth/auto-setup', '/auth/login', '/auth/register', '/auth/oidc/', '/auth/invitations/lookup', '/api/v1/embed/', '/api/v1/cas/'];

function isPublicAuthRequest(url?: string): boolean {
  if (!url) return false;
  return PUBLIC_AUTH_PATHS.some(p => url.includes(p));
}

// 处理队列中的请求
const processQueue = (error: any, token: string | null = null) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) {
      reject(error);
    } else {
      resolve(token);
    }
  });
  
  failedQueue = [];
};

function isEmbedPage(): boolean {
  if (typeof window === 'undefined') return false;
  return window.location.pathname.startsWith('/embed/');
}

function redirectToLogin() {
  if (typeof window === 'undefined') return;
  if (window.location.pathname === '/login') return;
  // Enterprise CAS invite landing must stay on-page (retry / CAS), never
  // fall through to the local login/register form.
  if (window.location.pathname === '/join-cas') return;
  // Embed 渠道用 Embed token 鉴权，匿名访问不应被踢到登录页
  if (isEmbedPage()) return;
  // NXIN：401 回首页，由路由守卫跳 CAS；本机/非 nxin 主机落自有 /login
  if (window.location.hostname.includes('.nxin.com')) {
    window.location.href = '/';
    return;
  }
  window.location.href = '/login';
}

instance.interceptors.response.use(
  (response) => {
    // 根据业务状态码处理逻辑
    const { status, data } = response;
    if (status >= 200 && status < 300) {
      return withHttpStatus(data, status);
    } else {
      return Promise.reject(withHttpStatus(data, status));
    }
  },
  async (error: any) => {
    const originalRequest = error.config;
    
    if (!error.response) {
      // A timeout and an unreachable server both arrive without a response, but
      // telling someone whose upload timed out to "check your connection" sends
      // them after the wrong problem.
      return Promise.reject({
        message: t(isTimeoutError(error) ? 'error.requestTimeout' : 'error.networkError'),
      });
    }

    // 文件下载失败时服务端仍返回 JSON；先还原错误信息，避免被 Blob 隐藏。
    // 不依赖 Content-Type：网关可能把错误改成 text/plain 或空类型。
    if (typeof Blob !== 'undefined' && error.response.data instanceof Blob) {
      try {
        const text = (await error.response.data.text()).trim();
        if (text.startsWith('{') || text.startsWith('[') || error.response.data.type.includes('json')) {
          error.response.data = JSON.parse(text);
        }
      } catch {
        // 非法 JSON 继续使用原有错误处理。
      }
    }
    
    // 公开接口（auto-setup / login / register / oidc）的 401 不走 refresh 逻辑，直接返回错误
    if ((error.response.status === 401 || error.response.status === 403) && isPublicAuthRequest(originalRequest?.url)) {
      const { status, data } = error.response;
      const msg = typeof data === 'object'
        ? (typeof data?.error === 'string' ? data.error : (data?.error?.message || data?.message))
        : data;
      return Promise.reject(withHttpStatus({ status, message: msg || t('error.invalidCredentials') }, status));
    }

    // Embed 调试页/挂件：无 JWT 时直接拒绝，勿走 refresh → /login
    if (error.response.status === 401 && isEmbedPage()) {
      const { status, data } = error.response;
      const msg = typeof data === 'object'
        ? (typeof data?.error === 'string' ? data.error : (data?.error?.message || data?.message))
        : data;
      return Promise.reject(withHttpStatus({ status, message: msg || t('error.invalidCredentials') }, status));
    }

    // 如果是401错误且不是刷新token的请求，尝试刷新token
    if (error.response.status === 401 && !originalRequest._retry && !originalRequest.url?.includes('/auth/refresh')) {
      // NXIN: this page load has not finished CAS reconcile. Refreshing the
      // leftover JWT would keep the previous user. Clear the session and
      // send the browser through CAS again.
      if (needsCasIdentityReconcile()) {
        try {
          const { useAuthStore } = await import('../stores/auth')
          useAuthStore().logout()
        } catch {
          localStorage.removeItem('weknora_token')
          localStorage.removeItem('weknora_refresh_token')
          localStorage.removeItem('weknora_user')
          localStorage.removeItem('weknora_tenant')
        }
        redirectToLogin();
        return Promise.reject({ message: t('error.pleaseRelogin') });
      }
      if (isRefreshing) {
        // 如果正在刷新token，将请求加入队列
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        }).then(token => {
          originalRequest.headers['Authorization'] = 'Bearer ' + token;
          return instance(originalRequest);
        }).catch(err => {
          return Promise.reject(err);
        });
      }
      
      originalRequest._retry = true;
      try {
        const token = await refreshAccessTokenShared({
          messages: {
            pleaseRelogin: t('error.pleaseRelogin'),
            tokenRefreshFailed: t('error.tokenRefreshFailed'),
          },
        });
        originalRequest.headers['Authorization'] = 'Bearer ' + token;
        return instance(originalRequest);
      } catch (refreshError) {
        // refreshAccessTokenShared already cleared credentials and redirected.
        return Promise.reject(refreshError);
      }
    }
    
    // 处理 Nginx 413 Request Entity Too Large
    const ERR_ENTITY_TOO_LARGE = 413;
    if (error.response.status === ERR_ENTITY_TOO_LARGE) {
      const skillUpload = isSkillBundleUploadUrl(error.config?.url)
      return Promise.reject(withHttpStatus({
        status: ERR_ENTITY_TOO_LARGE,
        message: skillUpload
          ? i18n.global.t('settings.sandbox.skillBundleTooLarge', { size: MAX_SKILL_BUNDLE_SIZE_MB })
          : i18n.global.t('error.fileSizeExceeded', { size: MAX_FILE_SIZE_MB }),
        success: false
      }, ERR_ENTITY_TOO_LARGE));
    }

    const { status, data } = error.response;
    // 将HTTP状态码一并抛出，方便上层判断401等场景
    // 后端返回格式: { success: false, error: { code, message, details } }
    // 提取 error.message 作为顶层 message，方便前端使用 error?.message 获取
    let errorMessage: string | undefined;
    if (typeof data === 'object') {
      if (typeof data?.error === 'string') {
        errorMessage = data.error;
      } else if (data?.error?.message) {
        errorMessage = data.error.message;
      } else {
        errorMessage = data?.message;
      }
    } else if (typeof data === 'string') {
      errorMessage = data;
    }
    return Promise.reject(withHttpStatus({
      status,
      message: errorMessage,
      ...(typeof data === 'object' ? data : {}) 
    }, status));
  }
);

export function get<T = any>(url: string, config?: any): Promise<WithStatus<T>> {
  return instance.get<T>(url, config) as unknown as Promise<WithStatus<T>>;
}

export async function getDown(url: string): Promise<Blob> {
  const res = await instance.get<Blob>(url, {
    responseType: "blob",
  }) as unknown as Blob;
  return res
}

export function postUpload(
  url: string,
  data = {},
  onUploadProgress?: (progressEvent: any) => void,
  config: any = {},
): Promise<WithStatus<any>> {
  return instance.post(url, data, {
    // Uploads are bounded by transfer time, not by the 30s default that suits
    // JSON calls. Derive the budget from the payload so a deployment raising
    // MAX_FILE_SIZE_MB doesn't silently abort its own uploads; an explicit
    // `config.timeout` still wins.
    timeout: uploadTimeoutMs(data),
    ...config,
    headers: {
      "Content-Type": "multipart/form-data",
      "X-Request-ID": `${generateRandomString(12)}`,
      ...(config.headers || {}),
    },
    onUploadProgress: onUploadProgress || config.onUploadProgress,
  }) as unknown as Promise<any>;
}

export function postChat<T = any>(url: string, data = {}): Promise<T> {
  // SSE stream: body is a string, so no `$httpStatus` is attached (see WithStatus).
  return instance.post(url, data, {
    headers: {
      "Content-Type": "text/event-stream;charset=utf-8",
      "X-Request-ID": `${generateRandomString(12)}`,
    },
  }) as unknown as Promise<T>;
}

export function post<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.post<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function put<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.put<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function patch<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.patch<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function del<T = any>(url: string, data?: any): Promise<WithStatus<T>> {
  return instance.delete<T>(url, { data }) as unknown as Promise<WithStatus<T>>;
}
