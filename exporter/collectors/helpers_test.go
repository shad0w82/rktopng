package collectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// sample is one gathered metric point.
type sample struct {
	labels map[string]string
	value  float64
}

// gather runs a collector through a pedantic registry (so a Describe/Collect
// mismatch fails the test) and returns its samples grouped by metric name.
func gather(t *testing.T, c prometheus.Collector) map[string][]sample {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	out := map[string][]sample{}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			s := sample{labels: map[string]string{}}
			for _, l := range m.GetLabel() {
				s.labels[l.GetName()] = l.GetValue()
			}
			s.value = valueOf(m)
			out[mf.GetName()] = append(out[mf.GetName()], s)
		}
	}
	return out
}

func valueOf(m *dto.Metric) float64 {
	if m.Gauge != nil {
		return m.Gauge.GetValue()
	}
	if m.Counter != nil {
		return m.Counter.GetValue()
	}
	return 0
}

// find returns the sample of a metric whose label key has the given value.
func find(t *testing.T, ss []sample, key, val string) sample {
	t.Helper()
	for _, s := range ss {
		if s.labels[key] == val {
			return s
		}
	}
	t.Fatalf("no sample with %s=%q in %v", key, val, ss)
	return sample{}
}

// has reports whether a sample with label key=val exists.
func has(ss []sample, key, val string) bool {
	for _, s := range ss {
		if s.labels[key] == val {
			return true
		}
	}
	return false
}

// fakeRoots points the package's ProcPath/SysPath at fresh temp trees for the
// duration of a test and returns their roots.
func fakeRoots(t *testing.T) (proc, sys string) {
	t.Helper()
	proc, sys = t.TempDir(), t.TempDir()
	oldP, oldS := ProcPath, SysPath
	ProcPath, SysPath = proc, sys
	t.Cleanup(func() { ProcPath, SysPath = oldP, oldS })
	return proc, sys
}

// write creates a file (and its parent dirs) with the given content.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// link creates a symlink (and its parent dirs).
func link(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
