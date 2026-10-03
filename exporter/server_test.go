package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rktopng/collectors"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestNormalizeBasePath(t *testing.T) {
	ok := map[string]string{
		"": "", "/": "", "  ": "", "rktopng": "/rktopng", "/rktopng": "/rktopng", "/rktopng/": "/rktopng",
		"/a/b": "/a/b", "a/b/": "/a/b", "//x": "/x", // the result never starts with "//", which a redirect would read as another host
		"/rk-top_ng.1~x": "/rk-top_ng.1~x",
	}
	for in, want := range ok {
		if got, err := normalizeBasePath(in); err != nil || got != want {
			t.Errorf("normalizeBasePath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"/a//b", "/a/../b", "/./a", "/a b", "/a?x=1", "/a#x", "/%2e%2e", `/a\b`, "/é"} {
		if got, err := normalizeBasePath(in); err == nil {
			t.Errorf("normalizeBasePath(%q) = %q, want an error", in, got)
		}
	}
}

func TestServerConfigFromEnv(t *testing.T) {
	c, err := serverConfigFromEnv(envOf(nil))
	if err != nil || c.BasePath != "" || c.authEnabled() {
		t.Errorf("default: %+v, %v (must be open and at /)", c, err)
	}
	c, err = serverConfigFromEnv(envOf(map[string]string{"RKTOP_BASE_PATH": "rktopng/", "RKTOP_AUTH_USER": "admin", "RKTOP_AUTH_PASSWORD": " pa ss "}))
	if err != nil || c.BasePath != "/rktopng" || !c.authEnabled() || c.User != "admin" || c.Password != " pa ss " {
		t.Errorf("full: %+v, %v (the password must be kept as typed)", c, err)
	}
	// A half-configured login must stop the start, not leave the server open.
	for name, env := range map[string]map[string]string{
		"user only":     {"RKTOP_AUTH_USER": "admin"},
		"password only": {"RKTOP_AUTH_PASSWORD": "secret"},
		"colon in user": {"RKTOP_AUTH_USER": "a:b", "RKTOP_AUTH_PASSWORD": "secret"},
		"bad base path": {"RKTOP_BASE_PATH": "/a b"},
	} {
		if _, err := serverConfigFromEnv(envOf(env)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func okHandler(seen *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.URL.RequestURI()
		}
		_, _ = io.WriteString(w, "secret data")
	})
}

func TestBasicAuth(t *testing.T) {
	h := basicAuth("admin", "s3cret", okHandler(nil))
	try := func(user, pass string, send bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/metrics", nil)
		if send {
			req.SetBasicAuth(user, pass)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := try("admin", "s3cret", true); rec.Code != 200 || rec.Body.String() != "secret data" {
		t.Errorf("right credentials: %d %q", rec.Code, rec.Body.String())
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"no credentials":       try("", "", false),
		"wrong password":       try("admin", "nope", true),
		"wrong user":           try("root", "s3cret", true),
		"both wrong":           try("root", "nope", true),
		"empty both":           try("", "", true),
		"longer than the real": try("admin", strings.Repeat("s3cret", 1000), true),
		"prefix of the real":   try("admin", "s3cre", true),
		"user as password":     try("s3cret", "admin", true),
	} {
		if rec.Code != 401 {
			t.Errorf("%s: %d, want 401", name, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "secret data") {
			t.Errorf("%s: the data leaked", name)
		}
		if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic realm=") {
			t.Errorf("%s: WWW-Authenticate = %q", name, got)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: a 401 must not be cached", name)
		}
	}
}

func TestUnderBasePath(t *testing.T) {
	var seen string
	h := underBasePath("/rktopng", okHandler(&seen))
	get := func(url string) *httptest.ResponseRecorder {
		seen = ""
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		return rec
	}
	for url, want := range map[string]string{
		"/rktopng/":         "/",
		"/rktopng/api/info": "/api/info",
		"/rktopng/api/processes?sort=cpu&limit=5": "/api/processes?sort=cpu&limit=5",
		"/rktopng/assets/index-ab.js":             "/assets/index-ab.js",
		"/rktopng/metrics":                        "/metrics",
	} {
		if rec := get(url); rec.Code != 200 || seen != want {
			t.Errorf("%s → %d, handler saw %q; want %q", url, rec.Code, seen, want)
		}
	}
	// The bare prefix gets its slash (relative addresses need it), keeping the query.
	if rec := get("/rktopng"); rec.Code != 301 || rec.Header().Get("Location") != "/rktopng/" || seen != "" {
		t.Errorf("/rktopng → %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get("/rktopng?view=all"); rec.Header().Get("Location") != "/rktopng/?view=all" {
		t.Errorf("query lost: %q", rec.Header().Get("Location"))
	}
	// The port opened directly goes to the dashboard; anything else outside the prefix is a 404,
	// including a path that merely starts with the same letters.
	if rec := get("/"); rec.Code != 302 || rec.Header().Get("Location") != "/rktopng/" {
		t.Errorf("/ → %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, url := range []string{"/metrics", "/api/info", "/rktopngx/", "/rktopng2", "/other/rktopng/"} {
		if rec := get(url); rec.Code != 404 || seen != "" {
			t.Errorf("%s → %d (handler saw %q), want 404", url, rec.Code, seen)
		}
	}
	// No base path: untouched.
	if h := underBasePath("", okHandler(&seen)); true {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/info", nil))
		if rec.Code != 200 || seen != "/api/info" {
			t.Errorf("no base path: %d %q", rec.Code, seen)
		}
	}
}

// realRoutes is the exporter's actual route table, with fake data behind it.
func realRoutes(t *testing.T) *http.ServeMux {
	t.Helper()
	reg := testRegistry(t)
	return newMux(deps{
		reg: reg, gs: same(reg), procs: collectors.NewProcessCollector(),
		smart: fakeSMART{}, interval: 50 * time.Millisecond, pprof: true,
	})
}

// With a password set, NOTHING answers without it: not the dashboard, not the API,
// not the Prometheus endpoint, not the profiler, not even an unknown path.
func TestEveryRouteNeedsThePassword(t *testing.T) {
	cfg := serverConfig{User: "admin", Password: "s3cret"}
	srv := httptest.NewServer(cfg.wrap(realRoutes(t)))
	defer srv.Close()

	paths := []string{
		"/", "/index.html", "/favicon.png", "/exporter", "/metrics",
		"/api/stats", "/api/stats?view=all", "/api/info", "/api/stream", "/api/processes", "/api/smart/sda",
		"/debug/pprof/", "/debug/pprof/cmdline", "/nope", "/api/nope",
	}
	for _, p := range paths {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("GET %s without a password → %d, want 401", p, resp.StatusCode)
		}
		if strings.Contains(string(b), "rk3588_") || strings.Contains(string(b), `"metrics"`) {
			t.Errorf("GET %s leaked data in its 401 body", p)
		}
	}

	// And with it, the same routes work.
	for _, p := range []string{"/", "/exporter", "/metrics", "/api/info", "/api/processes", "/api/smart/sda", "/debug/pprof/cmdline"} {
		req, _ := http.NewRequest("GET", srv.URL+p, nil)
		req.SetBasicAuth("admin", "s3cret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("GET %s with the password → %d, want 200", p, resp.StatusCode)
		}
	}
}

// The stream is the one long-lived request: it must also work with the password.
func TestStreamWorksWithThePassword(t *testing.T) {
	cfg := serverConfig{BasePath: "/rktopng", User: "admin", Password: "s3cret"}
	srv := httptest.NewServer(cfg.wrap(realRoutes(t)))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/rktopng/api/stream", nil)
	req.SetBasicAuth("admin", "s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	buf := make([]byte, 6)
	if _, err := io.ReadFull(resp.Body, buf); err != nil || string(buf) != "data: " {
		t.Errorf("first bytes = %q, %v", buf, err)
	}
}

// Behind a base path every endpoint moves with it, and the root is not served.
func TestRoutesUnderBasePath(t *testing.T) {
	cfg := serverConfig{BasePath: "/rktopng"}
	srv := httptest.NewServer(cfg.wrap(realRoutes(t)))
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	want := map[string]int{
		"/rktopng/": 200, "/rktopng/metrics": 200, "/rktopng/api/info": 200, "/rktopng/api/processes": 200,
		"/rktopng/api/smart/sda": 200, "/rktopng/exporter": 200, "/rktopng/index.html": 200, // index.html exists in a placeholder build too
		"/rktopng": 301, "/": 302,
		"/metrics": 404, "/api/info": 404, "/rktopng/nope": 404,
	}
	for p, code := range want {
		resp, err := client.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != code {
			t.Errorf("GET %s → %d, want %d", p, resp.StatusCode, code)
		}
	}
}

func TestStreamAsksProxiesNotToBuffer(t *testing.T) {
	srv := httptest.NewServer(realRoutes(t))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no (nginx would hold events back)", got)
	}
}

// The endpoint list page works under a base path only if its links are relative.
func TestExporterPageLinksAreRelative(t *testing.T) {
	rec := httptest.NewRecorder()
	exporterPage(rec, httptest.NewRequest("GET", "/exporter", nil))
	if strings.Contains(rec.Body.String(), `href="/`) {
		t.Errorf("an absolute link would escape the base path:\n%s", rec.Body.String())
	}
}
