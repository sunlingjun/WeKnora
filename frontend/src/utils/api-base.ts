export function getApiBaseUrl(): string {
  // LocalHub plugin: respect vite BASE_URL for reverse proxy deployments
  const baseUrl = (import.meta.env.BASE_URL || '/').replace(/\/+$/, '');
  if (baseUrl && baseUrl !== '/') {
    return baseUrl;
  }

  // Docker / same-origin（Vite 环境变量均为字符串，须显式比较 'true'）
  if (import.meta.env.VITE_IS_DOCKER === 'true') {
    return '';
  }

  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) {
    return envUrl;
  }

  // NXIN default API base (CAS HTTPS cookie)
  if (typeof window !== 'undefined' && window.location.protocol === 'https:') {
    return 'https://zsk.t.nxin.com:8080';
  }
  return 'http://zsk.t.nxin.com:8080';
}
