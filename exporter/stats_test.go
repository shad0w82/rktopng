package main

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"rktopng/collectors"
)

// testRegistry holds one metric of each kind the views must tell apart.
func testRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	gauge := func(name string, labels ...string) *prometheus.GaugeVec {
		g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: name}, labels)
		reg.MustRegister(g)
		return g
	}
	gauge("rk3588_soc_info", "soc").WithLabelValues("RK3588").Set(1)
	gauge("rk3588_disk_info", "device").WithLabelValues("sda").Set(1)
	gauge("rk3588_disk_size_bytes", "device").WithLabelValues("sda").Set(2e12)
	gauge("rk3588_smart_info", "device").WithLabelValues("sda").Set(1)
	gauge("rk3588_process_cpu_percent", "pid").WithLabelValues("42").Set(12.5)
	gauge("rk3588_process_info", "pid").WithLabelValues("42").Set(1)
	gauge("rk3588_procs", "state").WithLabelValues("total").Set(412) // the task COUNT must stay live
	gauge("rk3588_cpu_usage_percent", "core").WithLabelValues("0").Set(27.27272727272727)
	gauge("rk3588_cpu_governor_info", "governor").WithLabelValues("schedutil").Set(1) // can change at runtime
	gauge("rk3588_bad", "x").WithLabelValues("nan").Set(math.NaN())
	return reg
}

// same serves every view from one registry (the views are then told apart by
// the metric-name filters alone).
func same(g prometheus.Gatherer) gatherers { return gatherers{all: g, live: g, info: g} }

func names(s snapshot) map[string]bool {
	out := map[string]bool{}
	for k := range s.Metrics {
		out[k] = true
	}
	return out
}

func TestViews(t *testing.T) {
	reg := testRegistry(t)

	all := names(gatherSnapshot(reg, viewAll))
	for _, n := range []string{"rk3588_soc_info", "rk3588_process_info", "rk3588_cpu_usage_percent", "rk3588_procs"} {
		if !all[n] {
			t.Errorf("viewAll lacks %s", n)
		}
	}

	live := names(gatherSnapshot(reg, viewLive))
	for _, n := range []string{"rk3588_soc_info", "rk3588_disk_info", "rk3588_disk_size_bytes", "rk3588_smart_info", "rk3588_process_cpu_percent", "rk3588_process_info"} {
		if live[n] {
			t.Errorf("viewLive must not carry %s (static identity / per-process)", n)
		}
	}
	for _, n := range []string{"rk3588_cpu_usage_percent", "rk3588_procs", "rk3588_cpu_governor_info"} {
		if !live[n] {
			t.Errorf("viewLive lacks %s", n)
		}
	}

	info := names(gatherSnapshot(reg, viewInfo))
	if len(info) != 4 || !info["rk3588_soc_info"] || !info["rk3588_disk_info"] || !info["rk3588_disk_size_bytes"] || !info["rk3588_smart_info"] {
		t.Errorf("viewInfo = %v, want exactly the 4 static families", info)
	}
}

func TestSnapshotRoundsAndSkipsNaN(t *testing.T) {
	reg := testRegistry(t)
	s := gatherSnapshot(reg, viewAll)
	if got := s.Metrics["rk3588_cpu_usage_percent"][0].Value; got != 27.27 {
		t.Errorf("value not rounded to 2 decimals: %v", got)
	}
	if got := s.Metrics["rk3588_disk_size_bytes"][0].Value; got != 2e12 {
		t.Errorf("large values must survive rounding: %v", got)
	}
	if _, ok := s.Metrics["rk3588_bad"]; ok {
		t.Error("NaN samples must be skipped (JSON cannot carry them)")
	}
	if _, err := json.Marshal(s); err != nil {
		t.Errorf("snapshot must be JSON-encodable: %v", err)
	}
}

func TestParseView(t *testing.T) {
	for in, want := range map[string]view{"all": viewAll, "live": viewLive, "info": viewInfo} {
		if got := parseView(in, viewAll+7); got != want {
			t.Errorf("parseView(%q) = %v", in, got)
		}
	}
	if got := parseView("bogus", viewLive); got != viewLive {
		t.Errorf("unknown value must fall back to the default, got %v", got)
	}
	if got := parseView("", viewInfo); got != viewInfo {
		t.Errorf("empty value must fall back to the default, got %v", got)
	}
}

func getSnapshot(t *testing.T, h http.Handler, url string) snapshot {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("%s content-type = %q", url, ct)
	}
	var s snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	return s
}

func TestStatsAndInfoHandlers(t *testing.T) {
	reg := testRegistry(t)

	// /api/stats = everything by default, narrowable with ?view=.
	if all := getSnapshot(t, statsHandler(same(reg)), "/api/stats"); !names(all)["rk3588_process_info"] || !names(all)["rk3588_soc_info"] {
		t.Error("/api/stats must carry everything by default")
	}
	if live := getSnapshot(t, statsHandler(same(reg)), "/api/stats?view=live"); names(live)["rk3588_soc_info"] || names(live)["rk3588_process_info"] {
		t.Error("/api/stats?view=live must match the stream")
	}

	// /api/info = only the static identity.
	info := getSnapshot(t, infoHandler(same(reg)), "/api/info")
	if len(info.Metrics) != 4 || info.Timestamp == 0 {
		t.Errorf("/api/info = %v", names(info))
	}
}

// readFirstEvent opens the SSE stream and returns its first "data:" event.
func readFirstEvent(t *testing.T, url string) snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			var s snapshot
			if err := json.Unmarshal([]byte(line[len("data: "):]), &s); err != nil {
				t.Fatal(err)
			}
			return s
		}
	}
	t.Fatal("no SSE event received")
	return snapshot{}
}

func TestStreamIsLiveOnlyByDefault(t *testing.T) {
	reg := testRegistry(t)
	srv := httptest.NewServer(streamHandler(same(reg), 50*time.Millisecond))
	defer srv.Close()

	live := names(readFirstEvent(t, srv.URL))
	if live["rk3588_process_info"] || live["rk3588_soc_info"] || live["rk3588_smart_info"] {
		t.Errorf("the stream must not carry processes or static info: %v", live)
	}
	if !live["rk3588_cpu_usage_percent"] || !live["rk3588_procs"] {
		t.Errorf("the stream must carry live metrics: %v", live)
	}

	// ?view=all brings everything back (debugging).
	if all := names(readFirstEvent(t, srv.URL+"?view=all")); !all["rk3588_process_info"] || !all["rk3588_soc_info"] {
		t.Errorf("?view=all must include processes and static info: %v", all)
	}
}

// fakeProc builds a minimal /proc with n processes so the process list can be
// served without touching the real system.
func fakeProc(t *testing.T, n int) {
	t.Helper()
	dir := t.TempDir()
	old := collectors.ProcPath
	collectors.ProcPath = dir
	t.Cleanup(func() { collectors.ProcPath = old })
	if err := os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal:       16000000 kB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		pdir := filepath.Join(dir, strconv.Itoa(i))
		if err := os.MkdirAll(pdir, 0o755); err != nil {
			t.Fatal(err)
		}
		// pid (comm) state ppid ... — 22+ fields after the closing paren; field 20 = threads, 24 = rss pages.
		stat := strconv.Itoa(i) + " (proc" + strconv.Itoa(i) + ") S 1 1 1 0 -1 0 0 0 0 0 5 5 0 0 20 0 " + strconv.Itoa(i) + " 0 100 1000 " + strconv.Itoa(i*10) + " 0\n"
		if err := os.WriteFile(filepath.Join(pdir, "stat"), []byte(stat), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(pdir, "cmdline"), []byte("/usr/bin/proc"+strconv.Itoa(i)+"\x00--flag\x00"), 0o644)
	}
}

func TestProcessesEndpointReportsTotal(t *testing.T) {
	fakeProc(t, 12)
	h := processesHandler(collectors.NewProcessCollector())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/processes?sort=pid&limit=5", nil))

	var resp struct {
		Total     int `json:"total"`
		Processes []struct {
			Pid  int    `json:"pid"`
			Name string `json:"name"`
			Cmd  string `json:"cmd"`
		} `json:"processes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%v: %s", err, rec.Body.String())
	}
	if resp.Total != 12 {
		t.Errorf("total = %d, want 12 (every process, not only the returned rows)", resp.Total)
	}
	if len(resp.Processes) != 5 || resp.Processes[0].Pid != 1 || resp.Processes[4].Pid != 5 {
		t.Errorf("rows = %+v, want pids 1..5 (sorted by pid, limit 5)", resp.Processes)
	}
	if resp.Processes[0].Cmd != "/usr/bin/proc1 --flag" {
		t.Errorf("cmdline = %q", resp.Processes[0].Cmd)
	}
}

func TestProcessesEndpointReverse(t *testing.T) {
	fakeProc(t, 12)
	h := processesHandler(collectors.NewProcessCollector())
	get := func(url string) (rev bool, pids []int) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		var resp struct {
			Reverse   bool `json:"reverse"`
			Processes []struct {
				Pid int `json:"pid"`
			} `json:"processes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		for _, p := range resp.Processes {
			pids = append(pids, p.Pid)
		}
		return resp.Reverse, pids
	}
	if rev, got := get("/api/processes?sort=pid&limit=3"); rev || got[0] != 1 || got[2] != 3 {
		t.Errorf("default: reverse=%v pids=%v, want [1 2 3]", rev, got)
	}
	// the other end of the whole list, not the first page upside down
	if rev, got := get("/api/processes?sort=pid&limit=3&reverse=1"); !rev || got[0] != 12 || got[2] != 10 {
		t.Errorf("reverse: reverse=%v pids=%v, want [12 11 10]", rev, got)
	}
}

// fakeSMART answers /api/smart/<dev> without running smartctl.
type fakeSMART struct{ err error }

func (f fakeSMART) Get(ctx context.Context, dev string) (collectors.SmartDetail, error) {
	if f.err != nil {
		return collectors.SmartDetail{}, f.err
	}
	return collectors.SmartDetail{Device: dev, Available: true, Protocol: "ATA", Health: collectors.Health{State: "ok"}}, nil
}

func TestSmartEndpoint(t *testing.T) {
	get := func(rd smartGetter, url string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		smartHandler(rd).ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		return rec
	}
	rec := get(fakeSMART{}, "/api/smart/sda")
	var d collectors.SmartDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil || rec.Code != 200 || d.Device != "sda" || !d.Available || d.Health.State != "ok" {
		t.Errorf("ok: %d %v %+v", rec.Code, err, d)
	}
	if rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers: %v", rec.Header())
	}
	if rec := get(fakeSMART{err: collectors.ErrUnknownDisk}, "/api/smart/nope"); rec.Code != 404 || !strings.Contains(rec.Body.String(), "unknown disk") {
		t.Errorf("unknown disk: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(fakeSMART{err: context.Canceled}, "/api/smart/sda"); rec.Code != 503 {
		t.Errorf("a failed wait must be a 503, got %d", rec.Code)
	}
}
