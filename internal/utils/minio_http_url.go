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
	if !safeMinIOName(bucket) || !strings.HasPrefix(key, "images/") || !safeMinIOName(key) {
		return "", "", false
	}
	return bucket, key, true
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
