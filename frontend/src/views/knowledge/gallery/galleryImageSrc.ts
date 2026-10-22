import { applyStoredAuthHeaders } from '../../../utils/casSession.ts'
import {
  buildProtectedFileRequest,
  isProviderFileURL,
  resolveProtectedFileAccess,
  type ProtectedFileRequest,
} from '../../../utils/protectedFileAccess.ts'

const INTERNAL_MINIO_HOSTS = new Set(['localhost', '127.0.0.1', 'minio', 'host.docker.internal'])
const BUCKET_PROVIDER_SCHEMES = new Set(['minio', 'cos', 'tos', 's3', 'oss', 'ks3', 'obs'])

/**
 * The proxy request that loads one gallery image, or null when the browser can
 * render the URL as is.
 *
 * Stored images are storage handles (local://, minio://, cos://, s3://,
 * storage://<backend>/..., resource://) that no browser can open; they go
 * through the knowledge-base file proxy, which is also what authorises a
 * viewer of a KB shared from another tenant. The scheme list is the one
 * protectedFileAccess owns, so the gallery cannot drift from chat and wiki.
 *
 * Private object locators cannot be opened by the browser. Internal MinIO HTTP
 * URLs, and provider handles whose key is images/... or {tenant}/exports/...,
 * load through the gallery content endpoint. That endpoint authorizes the
 * exact URL against this knowledge base's image list. resource:// handles
 * stay on the knowledge-base file proxy: a document binding or a row in this
 * knowledge base's image list authorizes them. Public https URLs stay direct.
 */
export function galleryImageRequest(rawUrl: string, kbId: string): ProtectedFileRequest | null {
  if (!rawUrl) return null
  const access = resolveProtectedFileAccess({ mode: 'knowledgeBase', kbId })
  if (access.mode !== 'embed' && isGalleryContentObject(rawUrl)) {
    const id = kbId.trim()
    if (!id) return null
    const headers: Record<string, string> = {}
    try {
      applyStoredAuthHeaders(headers)
    } catch {
      // ignore localStorage read errors
    }
    return {
      url: `/api/v1/knowledge-bases/${encodeURIComponent(id)}/images/content?url=${encodeURIComponent(rawUrl.trim())}`,
      headers,
    }
  }
  if (!isProviderFileURL(rawUrl)) return null
  return buildProtectedFileRequest(rawUrl, access)
}

/** True when the gallery content endpoint, not the binding-checked file proxy, should load this URL. */
export function isGalleryContentObject(rawUrl: string): boolean {
  return isInternalMinIOHTTPURL(rawUrl) || galleryProviderObject(rawUrl.trim())
}

/** True when rawUrl is a private MinIO image object, not a public CDN URL. */
export function isInternalMinIOHTTPURL(rawUrl: string): boolean {
  let parsed: URL
  try {
    parsed = new URL(rawUrl.trim())
  } catch {
    return false
  }
  if (parsed.username || parsed.password) return false
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
  if (!INTERNAL_MINIO_HOSTS.has(parsed.hostname.toLowerCase()) || parsed.port !== '9000') return false
  let segments: string[]
  try {
    segments = parsed.pathname.split('/').filter(Boolean).map((part) => decodeURIComponent(part))
  } catch {
    return false
  }
  if (segments.length < 2) return false
  if (segments.some((part) => part === '.' || part === '..' || part.includes('\\'))) return false
  return galleryKeyAllowed(segments.slice(1).join('/'))
}

function galleryProviderObject(rawUrl: string): boolean {
  const match = /^storage:\/\/[0-9A-Za-z_-]+\/(.+)$/i.exec(rawUrl)
  const inner = match ? match[1] : rawUrl
  const separator = inner.indexOf('://')
  if (separator <= 0) return false
  const scheme = inner.slice(0, separator).toLowerCase()
  const rest = inner.slice(separator + 3)
  if (!rest || rest.includes('..')) return false
  if (scheme === 'local') return galleryKeyAllowed(rest)
  if (!BUCKET_PROVIDER_SCHEMES.has(scheme)) return false
  const slash = rest.indexOf('/')
  if (slash <= 0 || slash === rest.length - 1) return false
  return galleryKeyAllowed(rest.slice(slash + 1))
}

function galleryKeyAllowed(key: string): boolean {
  if (!key || key.includes('..') || key.includes('\\') || key.startsWith('/') || key.includes('//')) return false
  if (key.startsWith('images/')) return true
  const parts = key.split('/')
  for (let i = 0; i < parts.length; i += 1) {
    if (parts[i] !== 'exports') continue
    if (/^\d+$/.test(parts[i + 1] || '')) return true
    if (i > 0 && /^\d+$/.test(parts[i - 1] || '')) return true
  }
  return false
}
