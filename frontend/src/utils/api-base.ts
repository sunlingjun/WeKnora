/** Strip quotes / trailing slashes from Vite env string values. */
export function normalizeEnvUrl(raw: unknown): string {
  if (raw === undefined || raw === null) return ''
  let value = String(raw).trim()
  if (
    (value.startsWith('"') && value.endsWith('"')) ||
    (value.startsWith("'") && value.endsWith("'"))
  ) {
    value = value.slice(1, -1).trim()
  }
  return value.replace(/\/+$/, '')
}

/**
 * Resolve axios / fetch API base URL.
 *
 * Priority:
 * 1. Vite BASE_URL when deployed under a sub-path (LocalHub)
 * 2. VITE_IS_DOCKER=true → same-origin empty base (local vite proxy / nginx 反代)
 * 3. VITE_API_URL or VITE_APP_BASE_API（.env.development / .env.production）
 * 4. same-origin empty base（不再硬编码 host:8080）
 */
export function getApiBaseUrl(): string {
  const baseUrl = (import.meta.env.BASE_URL || '/').replace(/\/+$/, '')
  if (baseUrl && baseUrl !== '/') {
    return baseUrl
  }

  // Docker / reverse-proxy same-origin（Vite 环境变量均为字符串，须显式比较 'true'）
  if (import.meta.env.VITE_IS_DOCKER === 'true') {
    return ''
  }

  const fromEnv = normalizeEnvUrl(
    import.meta.env.VITE_API_URL || import.meta.env.VITE_APP_BASE_API,
  )
  if (fromEnv) {
    return fromEnv
  }

  return ''
}
