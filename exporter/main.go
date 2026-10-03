// RkTopNG exporter — Prometheus exporter for Rockchip RK3588 (FriendlyElec CM3588).
//
// Exposes CPU, thermal, GPU, NPU, RGA, VPU, memory, power and system metrics read
// from sysfs / procfs / debugfs. Designed to run in a container with the host
// /proc and /sys bind-mounted (see RKTOP_PROC_PATH / RKTOP_SYS_PATH).
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"rktopng/collectors"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := ":" + getenv("RKTOP_PORT", "9888")

	// Optional sub-path (reverse proxy) and password; see server.go.
	cfg, err := serverConfigFromEnv(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.BasePath != "" {
		log.Printf("serving under %s/ (RKTOP_BASE_PATH)", cfg.BasePath)
	}
	if cfg.authEnabled() {
		log.Printf("password required for everything, /metrics included (user %q, RKTOP_AUTH_USER)", cfg.User)
	}

	// Kept as references so /api/processes can serve the process list sorted by
	// any column, and so SMART (polled in the background: slow, needs root and
	// smartmontools) is shared with the per-disk temperature collector.
	procColl := collectors.NewProcessCollector()
	smartColl := collectors.NewSMARTCollector()
	smartDetail := collectors.NewSMARTDetailReader()

	// Live push interval for the SSE stream (default 1s).
	interval, err := time.ParseDuration(getenv("RKTOP_INTERVAL", "1s"))
	if err != nil || interval <= 0 {
		interval = time.Second
	}

	// Each collector reads its data sources fresh, but at most once per 70% of the
	// push interval, however many clients gather. Three registries keep a gather
	// from running collectors its endpoint discards (see registries.go).
	reg, gs := newRegistries(procColl, smartColl, interval*7/10)

	// Profiling for diagnosing CPU/memory use; off unless RKTOP_PPROF is set, because it exposes internals.
	//   go tool pprof 'http://<board>:9888/debug/pprof/profile?seconds=20'
	pprofOn := getenv("RKTOP_PPROF", "") != ""
	if pprofOn {
		log.Printf("profiling enabled at /debug/pprof/ (RKTOP_PPROF)")
	}
	mux := newMux(deps{reg: reg, gs: gs, procs: procColl, smart: smartDetail, interval: interval, pprof: pprofOn})

	log.Printf("RkTopNG exporter listening on %s (proc=%s sys=%s)",
		addr, collectors.ProcPath, collectors.SysPath)
	if err := http.ListenAndServe(addr, cfg.wrap(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
