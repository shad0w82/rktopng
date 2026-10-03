package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"rktopng/collectors"
)

// sample is one metric point: its value plus any labels.
type sample struct {
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// snapshot is the set of current metrics, grouped by metric name. It is produced
// by re-using the same Prometheus registry the collectors feed, so the JSON/SSE
// outputs stay automatically in sync with /metrics.
type snapshot struct {
	Timestamp int64               `json:"timestamp"`
	Metrics   map[string][]sample `json:"metrics"`
}

// view selects which part of the metrics a JSON/SSE response carries. The UI
// loads the static part once (/api/info) and then follows only the live part
// (/api/stream), so nothing that never changes is re-sent every second.
type view int

const (
	viewAll  view = iota // every metric
	viewLive             // what changes: no static identity metrics, no per-process metrics
	viewInfo             // only the static identity metrics
)

// staticFamilies identify the board and its disks and cannot change while the
// exporter runs, so they are served once by /api/info rather than in every push.
// (Things that look static but can change at runtime — CPU governor, ZFS pool
// state, USB-C port — stay in the live stream.)
var staticFamilies = map[string]bool{
	collectors.Namespace + "_soc_info":        true,
	collectors.Namespace + "_disk_info":       true,
	collectors.Namespace + "_disk_size_bytes": true,
	collectors.Namespace + "_smart_info":      true,
}

// processFamilyPrefix matches the per-process metrics. The UI's process table is
// fed by /api/processes (sortable by any column), so the live stream does not
// repeat them; /metrics keeps them for Prometheus.
const processFamilyPrefix = collectors.Namespace + "_process_"

func (v view) includes(name string) bool {
	switch v {
	case viewInfo:
		return staticFamilies[name]
	case viewLive:
		return !staticFamilies[name] && !strings.HasPrefix(name, processFamilyPrefix)
	}
	return true
}

// parseView reads a ?view= value (all|live|info), falling back to def.
func parseView(s string, def view) view {
	switch s {
	case "all":
		return viewAll
	case "live":
		return viewLive
	case "info":
		return viewInfo
	}
	return def
}

func metricValue(m *dto.Metric) float64 {
	switch {
	case m.Gauge != nil:
		return m.Gauge.GetValue()
	case m.Counter != nil:
		return m.Counter.GetValue()
	case m.Untyped != nil:
		return m.Untyped.GetValue()
	}
	return 0
}

// round2 keeps two decimals: plenty for a dashboard, and it stops values like
// 27.27272727272727 from bloating every push.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// gatherSnapshot collects the current metrics from the registry into a snapshot
// restricted to the given view. NaN/±Inf (which JSON cannot carry) are skipped.
func gatherSnapshot(g prometheus.Gatherer, v view) snapshot {
	snap := snapshot{Timestamp: time.Now().UnixMilli(), Metrics: map[string][]sample{}}
	families, err := g.Gather()
	if err != nil {
		return snap
	}
	for _, mf := range families {
		name := mf.GetName()
		if !v.includes(name) {
			continue
		}
		for _, m := range mf.GetMetric() {
			val := metricValue(m)
			if math.IsNaN(val) || math.IsInf(val, 0) {
				continue
			}
			s := sample{Value: round2(val)}
			if labels := m.GetLabel(); len(labels) > 0 {
				s.Labels = make(map[string]string, len(labels))
				for _, l := range labels {
					s.Labels[l.GetName()] = l.GetValue()
				}
			}
			snap.Metrics[name] = append(snap.Metrics[name], s)
		}
	}
	return snap
}

// statsHandler serves a one-shot JSON snapshot at /api/stats. By default it
// carries everything (handy for inspecting all the data); ?view=live|info
// narrows it to the same parts as /api/stream and /api/info.
func statsHandler(gs gatherers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := parseView(r.URL.Query().Get("view"), viewAll)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(gatherSnapshot(gs.of(v), v))
	}
}

// infoHandler serves the static identity metrics (board, disks) at /api/info.
// The UI calls it once when the page opens. SMART identity is filled in by a
// background poll, so it can be missing for the first seconds after start-up.
func infoHandler(gs gatherers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(gatherSnapshot(gs.info, viewInfo))
	}
}

// streamHandler pushes a fresh snapshot every interval over Server-Sent Events
// at /api/stream. The browser consumes it with EventSource. It carries only the
// live metrics (see viewLive); ?view=all adds everything back.
func streamHandler(gs gatherers, interval time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		v := parseView(r.URL.Query().Get("view"), viewLive)
		g := gs.of(v)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no") // nginx: pass each event on at once (it may otherwise hold it back)
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		enc := json.NewEncoder(w)
		send := func() {
			fmt.Fprint(w, "data: ")
			_ = enc.Encode(gatherSnapshot(g, v)) // writes compact JSON + newline
			fmt.Fprint(w, "\n")                  // blank line terminates the SSE event
			flusher.Flush()
		}

		send() // push immediately so the client isn't blank until the first tick
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				send()
			}
		}
	}
}

// processesHandler serves the process list sorted by ?sort= (cpu|mem|pid|name|
// user|threads, default cpu; biggest first for cpu, mem and threads, A→Z / lowest
// first for the others), flipped by ?reverse=1 and truncated to ?limit= (default 25) at
// /api/processes. Backs the sortable, clickable process table in the UI.
func processesHandler(pc *collectors.ProcessCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("sort")
		limit := 25
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
			limit = v
		}
		reverse := r.URL.Query().Get("reverse") == "1"
		procs, total := pc.List(key, limit, reverse)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"timestamp": time.Now().UnixMilli(),
			"sort":      key,
			"reverse":   reverse,
			"total":     total, // all processes, not just the returned rows
			"processes": procs,
		})
	}
}

// smartGetter is what smartHandler needs from the S.M.A.R.T. reader (a fake in tests).
type smartGetter interface {
	Get(ctx context.Context, dev string) (collectors.SmartDetail, error)
}

// smartHandler serves the full S.M.A.R.T. report of one disk at /api/smart/<dev> (nvme0n1,
// sda, ...): identity, health state with its reasons, headline figures, the attribute table
// or NVMe health log, and the drive's own logs. The disk is read when asked (and reused for a
// few seconds); a disk that cannot be read answers 200 with available=false and a reason.
// 404: not one of the system's disks. Backs the S.M.A.R.T. window of the UI.
func smartHandler(rd smartGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		d, err := rd.Get(r.Context(), strings.TrimPrefix(r.URL.Path, "/api/smart/"))
		switch {
		case errors.Is(err, collectors.ErrUnknownDisk):
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown disk"})
		case err != nil:
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		default:
			_ = json.NewEncoder(w).Encode(d)
		}
	}
}
