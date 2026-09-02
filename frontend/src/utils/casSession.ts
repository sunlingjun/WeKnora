/**
 * NXIN CAS cookies (_cas_sid / _cas_uid) are HttpOnly, so JS cannot
 * compare them to a stored JWT binding. Identity is reconciled once
 * per full page load via /api/v1/cas/validate; until that finishes,
 * leftover Bearer tokens must not be sent.
 */

// Early cookie-to-JWT binding keys. No longer written; logout still
// removes leftovers from older builds.
export const STALE_CAS_BINDING_SID_KEY = 'weknora_cas_bound_sid'
export const STALE_CAS_BINDING_UID_KEY = 'weknora_cas_bound_uid'

export function isNxinCASHost(hostname: string): boolean {
  return hostname.includes('.nxin.com')
}

export function shouldAttachStoredJWT(hostname: string, reconciledThisLoad: boolean): boolean {
  if (!isNxinCASHost(hostname)) return true
  return reconciledThisLoad
}

export function needsCasIdentityReconcileFor(hostname: string, reconciledThisLoad: boolean): boolean {
  if (!isNxinCASHost(hostname)) return false
  return !reconciledThisLoad
}

function browserHostname(): string {
  if (typeof window === 'undefined') return ''
  return window.location.hostname
}

let casIdentityReconciledThisLoad = false

export function markCasIdentityReconciled() {
  casIdentityReconciledThisLoad = true
}

export function resetCasIdentityReconcileForTests() {
  casIdentityReconciledThisLoad = false
}

export function clearStaleCasCookieBinding() {
  if (typeof localStorage === 'undefined') return
  localStorage.removeItem(STALE_CAS_BINDING_SID_KEY)
  localStorage.removeItem(STALE_CAS_BINDING_UID_KEY)
}

export function currentShouldAttachStoredJWT(): boolean {
  return shouldAttachStoredJWT(browserHostname(), casIdentityReconciledThisLoad)
}

export function needsCasIdentityReconcile(): boolean {
  if (typeof window === 'undefined') return false
  return needsCasIdentityReconcileFor(browserHostname(), casIdentityReconciledThisLoad)
}

/** Attach leftover JWT / X-Tenant-ID only after this load's CAS reconcile. */
export function applyStoredAuthHeaders(headers: Record<string, string>, opts?: { includeTenant?: boolean }) {
  if (!currentShouldAttachStoredJWT()) return
  if (typeof localStorage === 'undefined') return
  const token = (localStorage.getItem('weknora_token') || '').trim()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  if (opts?.includeTenant === false) return
  const selectedTenantId = (localStorage.getItem('weknora_selected_tenant_id') || '').trim()
  if (selectedTenantId) {
    headers['X-Tenant-ID'] = selectedTenantId
  }
}
