package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"rktopng/collectors"
	"rktopng/web"
)

// serverConfig is how the HTTP surface is exposed: under a sub-path (behind a
// reverse proxy that does not strip it) and/or behind a password. Both are off by
// default, which keeps the exporter open at / exactly as before.
type serverConfig struct {
	BasePath string // "" (served at /) or "/segment[/segment...]", never a trailing slash
	User     string // HTTP Basic credentials; both empty = no authentication
	Password string
}

var basePathSegment = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

// normalizeBasePath turns "rktopng", "/rktopng" and "/rktopng/" into "/rktopng",
// and "" or "/" into "". Only plain segments are accepted: the value ends up in
// redirects, so nothing that needs escaping gets through.
func normalizeBasePath(s string) (string, error) {
	t := strings.Trim(strings.TrimSpace(s), "/")
	if t == "" {
		return "", nil
	}
	for _, seg := range strings.Split(t, "/") {
		if seg == "." || seg == ".." || !basePathSegment.MatchString(seg) {
			return "", fmt.Errorf("RKTOP_BASE_PATH=%q is not a plain path such as /rktopng", s)
		}
	}
	return "/" + t, nil
}

// serverConfigFromEnv reads RKTOP_BASE_PATH, RKTOP_AUTH_USER and
// RKTOP_AUTH_PASSWORD. A half-configured login is an error rather than an open
// server: someone who set a user clearly wants a password.
func serverConfigFromEnv(env func(string) string) (serverConfig, error) {
	base, err := normalizeBasePath(env("RKTOP_BASE_PATH"))
	if err != nil {
		return serverConfig{}, err
	}
	c := serverConfig{BasePath: base, User: env("RKTOP_AUTH_USER"), Password: env("RKTOP_AUTH_PASSWORD")}
	if (c.User == "") != (c.Password == "") {
		return serverConfig{}, errors.New("RKTOP_AUTH_USER and RKTOP_AUTH_PASSWORD must be set together (leave both empty for no password)")
	}
	if strings.Contains(c.User, ":") {
		return serverConfig{}, errors.New("RKTOP_AUTH_USER cannot contain ':' (HTTP Basic separates user and password with it)")
	}
	return c, nil
}

func (c serverConfig) authEnabled() bool { return c.User != "" }

// wrap puts the routes under the base path and behind the password. The password
// is the outermost layer, so every request needs it: the dashboard, /api/*,
// /metrics, /debug/pprof, redirects and even a 404.
func (c serverConfig) wrap(h http.Handler) http.Handler {
	h = underBasePath(c.BasePath, h)
	if c.authEnabled() {
		h = basicAuth(c.User, c.Password, h)
	}
	return h
}

// basicAuth asks for HTTP Basic credentials. Both values are always compared (over
// their SHA-256, so the lengths do not leak either) and the comparison takes the
// same time wherever the first difference is.
func basicAuth(user, pass string, next http.Handler) http.Handler {
	wantUser, wantPass := sha256.Sum256([]byte(user)), sha256.Sum256([]byte(pass))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		gotUser, gotPass := sha256.Sum256([]byte(u)), sha256.Sum256([]byte(p))
		userOK := subtle.ConstantTimeCompare(gotUser[:], wantUser[:])
		passOK := subtle.ConstantTimeCompare(gotPass[:], wantPass[:])
		if !ok || userOK&passOK != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="RkTopNG", charset="UTF-8"`)
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// underBasePath serves next below base, for a reverse proxy that forwards
// /rktopng/... unchanged. "/rktopng" is sent to "/rktopng/" because the dashboard
// asks for its data with relative addresses, which only resolve under a slash.
// With no base path it is a pass-through.
func underBasePath(base string, next http.Handler) http.Handler {
	if base == "" {
		return next
	}
	strip := http.StripPrefix(base, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == base:
			target := base + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		case strings.HasPrefix(r.URL.Path, base+"/"):
			strip.ServeHTTP(w, r)
		case r.URL.Path == "/":
			http.Redirect(w, r, base+"/", http.StatusFound) // someone opened the port directly
		default:
			http.NotFound(w, r)
		}
	})
}

// deps is everything the routes need.
type deps struct {
	reg      prometheus.Gatherer // /metrics
	gs       gatherers
	procs    *collectors.ProcessCollector
	smart    smartGetter
	interval time.Duration
	pprof    bool
}

// newMux registers every endpoint. They are all relative to the root: the base
// path and the password are applied around the whole mux (serverConfig.wrap).
func newMux(d deps) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(d.reg, promhttp.HandlerOpts{})) // Prometheus
	mux.HandleFunc("/api/stats", statsHandler(d.gs))                           // JSON snapshot of everything (?view=live|info)
	mux.HandleFunc("/api/info", infoHandler(d.gs))                             // static board/disk identity, loaded once
	mux.HandleFunc("/api/stream", streamHandler(d.gs, d.interval))             // SSE push of the live metrics only
	mux.HandleFunc("/api/processes", processesHandler(d.procs))                // sortable process list
	mux.HandleFunc("/api/smart/", smartHandler(d.smart))                       // full S.M.A.R.T. report of one disk, read on demand
	if d.pprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	mux.Handle("/", web.Handler()) // the dashboard itself (embedded build of frontend/)
	mux.HandleFunc("/exporter", exporterPage)
	return mux
}

// exporterPage lists the endpoints. Its links are relative, so they work under a
// base path too.
func exporterPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><title>RkTopNG exporter</title>
<h1>RkTopNG exporter</h1><p>RK3588 / CM3588 metrics.</p>
<ul><li><a href="metrics">metrics</a> (Prometheus, everything)</li>
<li><a href="api/stats">api/stats</a> (JSON snapshot of everything; ?view=live or ?view=info)</li>
<li><a href="api/info">api/info</a> (JSON, static board and disk identity: load once)</li>
<li>api/stream (Server-Sent Events: live metrics only, one push per interval)</li>
<li><a href="api/processes?sort=cpu&amp;limit=20">api/processes</a> (JSON, sortable process list)</li>
<li>api/smart/&lt;disk&gt; (JSON, full S.M.A.R.T. report of one disk, e.g. api/smart/nvme0n1)</li></ul>`))
}
