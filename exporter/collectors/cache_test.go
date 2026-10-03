package collectors

import (
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// counting is a collector that records how many times it was really run.
type counting struct {
	desc  *prometheus.Desc
	calls atomic.Int32
}

func newCounting() *counting {
	return &counting{desc: prometheus.NewDesc("rk3588_test_value", "test", nil, nil)}
}

func (c *counting) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c *counting) Collect(ch chan<- prometheus.Metric) {
	n := c.calls.Add(1)
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(n))
}

func TestCachedRunsTheCollectorOncePerTTL(t *testing.T) {
	inner := newCounting()
	c := Cached(inner, 80*time.Millisecond)

	for i := 0; i < 5; i++ {
		got := gather(t, c)["rk3588_test_value"]
		if len(got) != 1 || got[0].value != 1 {
			t.Fatalf("gather %d: %v, want the first reading replayed", i, got)
		}
	}
	if n := inner.calls.Load(); n != 1 {
		t.Errorf("collector ran %d times within the ttl, want 1", n)
	}

	time.Sleep(100 * time.Millisecond)
	if got := gather(t, c)["rk3588_test_value"]; len(got) != 1 || got[0].value != 2 {
		t.Errorf("after the ttl the collector must run again: %v", got)
	}
}

// The wrapper may be registered in several registries (live, all): they share one reading.
func TestCachedIsSharedAcrossRegistries(t *testing.T) {
	inner := newCounting()
	c := Cached(inner, time.Minute)
	for i := 0; i < 3; i++ {
		reg := prometheus.NewPedanticRegistry()
		reg.MustRegister(c)
		if _, err := reg.Gather(); err != nil {
			t.Fatal(err)
		}
	}
	if n := inner.calls.Load(); n != 1 {
		t.Errorf("collector ran %d times for 3 registries, want 1", n)
	}
}

func TestCachedIsSafeUnderConcurrentGathers(t *testing.T) {
	inner := newCounting()
	c := Cached(inner, 20*time.Millisecond)
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := reg.Gather(); err != nil {
					t.Error(err)
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}
	wg.Wait()
}

// The reason the wrapper exists: a second gather right after the first used to see
// an unchanged /proc/stat (zero-length window) and silently drop the CPU usage.
func TestCachedKeepsCPUUsageForClientsThatGatherTogether(t *testing.T) {
	proc, sys := fakeRoots(t)
	_ = sys
	stat := func(user, idle int) {
		write(t, filepath.Join(proc, "stat"), "cpu  "+strconv.Itoa(user)+" 0 0 "+strconv.Itoa(idle)+" 0 0 0 0\ncpu0 "+strconv.Itoa(user)+" 0 0 "+strconv.Itoa(idle)+" 0 0 0 0\n")
	}
	stat(100, 900)
	c := Cached(NewCPUCollector(), 200*time.Millisecond)
	gather(t, c) // first reading: remembers the counters, no rate yet

	time.Sleep(250 * time.Millisecond)
	stat(150, 950) // the CPU did 50 busy + 50 idle ticks since
	first := gather(t, c)["rk3588_cpu_usage_percent"]
	if len(first) != 1 || first[0].value != 50 {
		t.Fatalf("usage = %v, want one core at 50", first)
	}

	// Another client, a moment later, with the counters not moved yet: without the
	// wrapper dt would be 0 and the sample would vanish.
	second := gather(t, c)["rk3588_cpu_usage_percent"]
	if len(second) != 1 || second[0].value != 50 {
		t.Errorf("second client got %v, want the same reading", second)
	}
}
