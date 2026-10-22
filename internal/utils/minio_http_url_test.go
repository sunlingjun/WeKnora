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

	exportsHTTP := "http://localhost:9000/nxinweknora/10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg"
	bucket, key, ok = ParseInternalMinIOHTTPURL(exportsHTTP)
	if !ok || bucket != "nxinweknora" || key != "10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg" {
		t.Fatalf("exports http: bucket=%q key=%q ok=%v", bucket, key, ok)
	}
}

func TestGalleryImageStoragePath(t *testing.T) {
	const bucket = "nxinweknora"
	const owner uint64 = 10005
	exports := "minio://nxinweknora/10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg"
	got, ok := GalleryImageStoragePath(exports, bucket, owner)
	if !ok || got != exports {
		t.Fatalf("provider exports: got=%q ok=%v", got, ok)
	}

	httpExports := "http://localhost:9000/nxinweknora/10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg"
	got, ok = GalleryImageStoragePath(httpExports, bucket, owner)
	if !ok || got != exports {
		t.Fatalf("http exports: got=%q ok=%v", got, ok)
	}

	for _, raw := range []string{
		"minio://nxinweknora/images/a.png",
		"local://10005/exports/a.png",
		"cos://bucket/10005/exports/a.png",
		"s3://bucket/10005/exports/a.png",
		"tos://bucket/exports/10005/a.png",
		"oss://bucket/10005/exports/a.png",
		"ks3://bucket/10005/exports/a.png",
		"obs://bucket/10005/exports/a.png",
		"storage://backend-1/minio://nxinweknora/10005/exports/a.jpg",
		"storage://backend-1/local://10005/exports/a.png",
	} {
		got, ok := GalleryImageStoragePath(raw, bucket, owner)
		if !ok || got != raw {
			t.Fatalf("same-type object %s: got=%q ok=%v", raw, got, ok)
		}
	}

	for _, raw := range []string{
		"minio://other/10005/exports/a.jpg",
		"minio://nxinweknora/1/exports/a.jpg",
		"minio://nxinweknora/10005/raw/secret.jpg",
		"local://1/exports/a.png",
		"resource://AbCdEfGhIjKlMnOpQrStUv",
		"https://cdn.example.com/a.png",
		"http://localhost:9000/nxinweknora/10005/raw/secret.jpg",
	} {
		if _, ok := GalleryImageStoragePath(raw, bucket, owner); ok {
			t.Fatalf("expected rejection: %s", raw)
		}
	}
}
