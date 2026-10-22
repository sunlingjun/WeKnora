package utils

import (
	"net/url"
	"strings"
)

// builtinMinIOHosts are the hosts a private MinIO writes into image records
// when its public endpoint points at the compose network or the developer's
// machine. They are never a public CDN.
var builtinMinIOHosts = map[string]struct{}{
	"localhost":            {},
	"127.0.0.1":            {},
	"minio":                {},
	"host.docker.internal": {},
}

// ParseInternalMinIOHTTPURL reports the bucket and object key of an HTTP URL
// that addresses this deployment's MinIO. Public https URLs return ok=false so
// callers keep serving them directly. endpoints are extra MINIO_ENDPOINT /
// MINIO_PUBLIC_ENDPOINT values (host:port or full URL).
func ParseInternalMinIOHTTPURL(raw string, endpoints ...string) (bucket, key string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return "", "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", false
	}
	if !internalMinIOHost(parsed, endpoints) {
		return "", "", false
	}
	path, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil {
		return "", "", false
	}
	slash := strings.IndexByte(path, '/')
	if slash <= 0 || slash >= len(path)-1 {
		return "", "", false
	}
	bucket, key = path[:slash], path[slash+1:]
	if !safeMinIOName(bucket) || !galleryObjectKeyAllowed(key) {
		return "", "", false
	}
	return bucket, key, true
}

// GalleryImageStoragePath is the storage locator for one gallery image.
// Private MinIO HTTP URLs become minio://bucket/key. Provider handles
// (minio://, local://, cos://, s3://, and the same family, including a
// storage:// backend wrapper) pass through when the object key is an
// extracted image (images/...) or a KB export ({tenant}/exports/... or
// exports/{tenant}/...). Exports must belong to ownerTenant. resource://
// stays on the binding-checked file proxy.
func GalleryImageStoragePath(raw, configuredBucket string, ownerTenant uint64, endpoints ...string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 || strings.Contains(raw, "..") {
		return "", false
	}
	if bucket, key, ok := ParseInternalMinIOHTTPURL(raw, endpoints...); ok {
		if !minioBucketAllowed(bucket, configuredBucket) {
			return "", false
		}
		path := "minio://" + bucket + "/" + key
		if !galleryPathTenantOK(path, key, ownerTenant) {
			return "", false
		}
		return path, true
	}
	path, key, scheme, ok := providerGalleryPath(raw)
	if !ok {
		return "", false
	}
	if scheme == "minio" {
		bucket, _, _ := strings.Cut(strings.TrimPrefix(unwrapStorageBackendPath(path), "minio://"), "/")
		if !minioBucketAllowed(bucket, configuredBucket) {
			return "", false
		}
	}
	if !galleryPathTenantOK(path, key, ownerTenant) {
		return "", false
	}
	return path, true
}

func providerGalleryPath(raw string) (path, key, scheme string, ok bool) {
	inner := unwrapStorageBackendPath(raw)
	scheme, rest, found := strings.Cut(inner, "://")
	if !found || rest == "" {
		return "", "", "", false
	}
	scheme = strings.ToLower(scheme)
	switch scheme {
	case "local":
		if !galleryObjectKeyAllowed(rest) {
			return "", "", "", false
		}
		return raw, rest, scheme, true
	case "minio", "cos", "tos", "s3", "oss", "ks3", "obs":
		bucket, objectKey, found := strings.Cut(rest, "/")
		if !found || !safeMinIOName(bucket) || !galleryObjectKeyAllowed(objectKey) {
			return "", "", "", false
		}
		return raw, objectKey, scheme, true
	default:
		return "", "", "", false
	}
}

func galleryPathTenantOK(path, key string, ownerTenant uint64) bool {
	if strings.HasPrefix(key, "images/") && !keyHasExportsScope(key) {
		return true
	}
	if ownerTenant == 0 {
		return false
	}
	return ValidateKBScopedStoragePath(path, ownerTenant) == nil
}

func galleryObjectKeyAllowed(key string) bool {
	if !safeMinIOName(key) {
		return false
	}
	if strings.HasPrefix(key, "images/") {
		return true
	}
	return keyHasExportsScope(key)
}

func keyHasExportsScope(key string) bool {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		if part != kbScopedExportsSegment {
			continue
		}
		if i+1 < len(parts) && allDigits(parts[i+1]) {
			return true
		}
		if i > 0 && allDigits(parts[i-1]) {
			return true
		}
	}
	return false
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func minioBucketAllowed(bucket, configured string) bool {
	configured = strings.TrimSpace(configured)
	return configured != "" && bucket == configured
}

func internalMinIOHost(parsed *url.URL, endpoints []string) bool {
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if _, builtin := builtinMinIOHosts[host]; builtin && port == "9000" {
		return true
	}
	for _, endpoint := range endpoints {
		endpointHost, endpointPort, matched := minioEndpointHostPort(endpoint)
		if matched && host == endpointHost && port == endpointPort {
			return true
		}
	}
	return false
}

func minioEndpointHostPort(endpoint string) (host, port string, ok bool) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", false
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		return "", "", false
	}
	port = parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(parsed.Hostname()), port, true
}

func safeMinIOName(value string) bool {
	if value == "" || strings.Contains(value, "..") || strings.Contains(value, `\`) {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "//") {
		return false
	}
	return true
}
