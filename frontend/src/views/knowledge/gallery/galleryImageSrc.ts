import { applyStoredAuthHeaders } from '../../../utils/casSession.ts'
import {
  buildProtectedFileRequest,
  isProviderFileURL,
  resolveProtectedFileAccess,
  type ProtectedFileRequest,
} from '../../../utils/protectedFileAccess.ts'

const INTERNAL_MINIO_HOSTS = new Set(['localhost', '127.0.0.1', 'minio', 'host.docker.internal'])

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
 * A private MinIO HTTP URL (http://localhost:9000/<bucket>/images/...) is the
 * same class of stored object: the browser would request the viewer's own
 * machine. Those load through the gallery content endpoint, which reads the
 * object with the server's credentials after checking the URL belongs to the
 * knowledge base. Public https URLs stay direct.
 */
export function galleryImageRequest(rawUrl: string, kbId: string): ProtectedFileRequest | null {
  if (!rawUrl) return null
  const access = resolveProtectedFileAccess({ mode: 'knowledgeBase', kbId })
  if (isProviderFileURL(rawUrl)) return buildProtectedFileRequest(rawUrl, access)
  if (access.mode === 'embed' || !isInternalMinIOHTTPURL(rawUrl)) return null
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
  const key = segments.slice(1).join('/')
  return key.startsWith('images/') && !key.includes('..')
}
