package utils

import "testing"

func TestParseInternalMinIOHTTPURL(t *testing.T) {
	const image = "http://localhost:9000/nxinweknora/images/a00fbdb2461240f99855168f2432b3c4.png"
	bucket, key, ok := ParseInternalMinIOHTTPURL(image)
	if !ok || bucket != "nxinweknora" || key != "images/a00fbdb2461240f99855168f2432b3c4.png" {
		t.Fatalf("localhost image: bucket=%q key=%q ok=%v", bucket, key, ok)
	}

	for _, raw := range []string{
		"http://127.0.0.1:9000/nxinweknora/images/a.png",
		"http://minio:9000/nxinweknora/images/a.png",
		"https://host.docker.internal:9000/nxinweknora/images/a.png",
		"http://localhost:9000/nxinweknora/images/a.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=deadbeef",
	} {
		if _, _, ok := ParseInternalMinIOHTTPURL(raw); !ok {
			t.Fatalf("expected internal object: %s", raw)
		}
	}

	bucket, key, ok = ParseInternalMinIOHTTPURL(
		"https://minioapi.example.com:9443/nxinweknora/images/a.png",
		"minioapi.example.com:9443",
	)
	if !ok || bucket != "nxinweknora" || key != "images/a.png" {
		t.Fatalf("configured endpoint: bucket=%q key=%q ok=%v", bucket, key, ok)
	}

	for _, raw := range []string{
		"https://cdn.example.com/a.png",
		"http://localhost/nxinweknora/images/a.png",
		"http://evil.example:9000/nxinweknora/images/a.png",
		"http://localhost:9000/nxinweknora",
		"http://localhost:9000/nxinweknora/exports/a.png",
		"http://localhost:9000/nxinweknora/images/../secret.png",
		"http://user:pass@localhost:9000/nxinweknora/images/a.png",
		"minio://nxinweknora/images/a.png",
		"",
	} {
		if _, _, ok := ParseInternalMinIOHTTPURL(raw); ok {
			t.Fatalf("expected rejection: %s", raw)
		}
	}
}
