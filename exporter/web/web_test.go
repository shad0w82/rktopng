package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var site = fstest.MapFS{
	"index.html":                {Data: []byte("<!doctype html><title>RkTopNG</title>")},
	"favicon.png":               {Data: []byte("\x89PNG fake")},
	"assets/index-abc123.js":    {Data: []byte("console.log('hi');" + strings.Repeat(" ", 200))},
	"assets/index-abc123.css":   {Data: []byte("body{}")},
	"assets/plex-400-xyz.woff2": {Data: []byte("wOF2 fake font")},
}

func get(t *testing.T, method, url string, hdr map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newHandler(site).ServeHTTP(rec, req)
	return rec.Result()
}

func body(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestServesIndexAtRootAndNeverCachesIt(t *testing.T) {
	r := get(t, "GET", "/", nil)
	if r.StatusCode != 200 || !strings.Contains(body(t, r), "RkTopNG") {
		t.Fatalf("status %d", r.StatusCode)
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
	if cc := r.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index must be revalidated, got %q", cc)
	}
}

func TestHashedAssetsAreCachedForever(t *testing.T) {
	r := get(t, "GET", "/assets/index-abc123.js", nil)
	if cc := r.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("Cache-Control = %q", cc)
	}
	if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("js content-type = %q", ct)
	}
	if ct := get(t, "GET", "/assets/plex-400-xyz.woff2", nil).Header.Get("Content-Type"); ct != "font/woff2" {
		t.Errorf("woff2 content-type = %q", ct)
	}
	if cc := get(t, "GET", "/favicon.png", nil).Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("an unhashed file must not be immutable: %q", cc)
	}
}

// The dashboard asks for everything with relative addresses, so index.html must
// only come from "/" (or its own name): answering an unknown path such as
// /a/b with it would load a page whose assets resolve under /a/.
func TestUnknownPathsAreNotAnsweredWithIndex(t *testing.T) {
	for _, u := range []string{"/", "/index.html"} {
		if r := get(t, "GET", u, nil); r.StatusCode != 200 || !strings.Contains(body(t, r), "RkTopNG") {
			t.Errorf("%s → %d, want the page", u, r.StatusCode)
		}
	}
	for _, u := range []string{"/storage", "/a/b", "/assets/missing.js", "/nope.png", "/api/nope", "/api/info/x"} {
		if r := get(t, "GET", u, nil); r.StatusCode != 404 {
			t.Errorf("%s → %d, want 404", u, r.StatusCode)
		}
	}
}

func TestGzipOnlyForTextWhenAccepted(t *testing.T) {
	r := get(t, "GET", "/assets/index-abc123.js", map[string]string{"Accept-Encoding": "br, gzip;q=0.8"})
	if r.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("js should be gzipped when the client accepts it")
	}
	zr, err := gzip.NewReader(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if !strings.HasPrefix(string(plain), "console.log('hi');") {
		t.Errorf("round trip failed: %q", plain)
	}
	if !strings.Contains(r.Header.Get("Vary"), "Accept-Encoding") {
		t.Error("Vary: Accept-Encoding is needed with content negotiation")
	}

	if r := get(t, "GET", "/assets/index-abc123.js", nil); r.Header.Get("Content-Encoding") != "" {
		t.Error("no gzip unless the client asks for it")
	}
	for _, u := range []string{"/assets/plex-400-xyz.woff2", "/favicon.png"} {
		if r := get(t, "GET", u, map[string]string{"Accept-Encoding": "gzip"}); r.Header.Get("Content-Encoding") != "" {
			t.Errorf("%s is already compressed, must not be gzipped again", u)
		}
	}
}

func TestMethodsAndTraversal(t *testing.T) {
	if r := get(t, "POST", "/", nil); r.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST → %d", r.StatusCode)
	}
	if r := get(t, "HEAD", "/", nil); r.StatusCode != 200 || body(t, r) != "" {
		t.Errorf("HEAD must answer without a body (status %d)", r.StatusCode)
	}
	// Cleaned before use: stays inside the site and falls back like any unknown route.
	if r := get(t, "GET", "/../../etc/passwd", nil); strings.Contains(body(t, r), "root:") {
		t.Error("path traversal")
	}
}

// The embedded build always contains an index.html (the placeholder before the
// first `npm run build`, the real app after it).
func TestEmbeddedBuildHasIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("embedded index missing: %d", rec.Code)
	}
}
