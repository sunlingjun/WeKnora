export function getApiBaseUrl(): string {
  // Docker / 同源部署：使用相对路径
  if (import.meta.env.VITE_IS_DOCKER) {
    return '';
  }
  // 显式配置优先
  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) {
    return envUrl;
  }
  // NXIN 环境默认 API 基址（支持 CAS HTTPS Cookie）
  if (typeof window !== 'undefined' && window.location.protocol === 'https:') {
    return 'https://zsk.t.nxin.com:8080';
  }
  return 'http://zsk.t.nxin.com:8080';
}
