package collectors

import (
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// MemoryCollector exposes RAM and swap usage parsed from /proc/meminfo.
type MemoryCollector struct {
	mem  *prometheus.Desc
	swap *prometheus.Desc
}

func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{
		mem: prometheus.NewDesc(Namespace+"_memory_bytes",
			"RAM in bytes by type (total/free/available/used/cached/buffers).",
			[]string{"type"}, nil),
		swap: prometheus.NewDesc(Namespace+"_swap_bytes",
			"Swap in bytes by type (total/free/used).",
			[]string{"type"}, nil),
	}
}

func (c *MemoryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.mem
	ch <- c.swap
}

func (c *MemoryCollector) Collect(ch chan<- prometheus.Metric) {
	data, err := os.ReadFile(Proc("meminfo"))
	if err != nil {
		return
	}
	kv := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		f := fields(line)
		if len(f) < 2 {
			continue
		}
		key := strings.TrimSuffix(f[0], ":")
		v, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			continue
		}
		kv[key] = v * 1024 // kB -> bytes
	}

	emitMem := func(t string, v float64) {
		ch <- prometheus.MustNewConstMetric(c.mem, prometheus.GaugeValue, v, t)
	}
	emitMem("total", kv["MemTotal"])
	emitMem("free", kv["MemFree"])
	emitMem("available", kv["MemAvailable"])
	emitMem("cached", kv["Cached"])
	emitMem("buffers", kv["Buffers"])
	emitMem("used", kv["MemTotal"]-kv["MemAvailable"])

	ch <- prometheus.MustNewConstMetric(c.swap, prometheus.GaugeValue, kv["SwapTotal"], "total")
	ch <- prometheus.MustNewConstMetric(c.swap, prometheus.GaugeValue, kv["SwapFree"], "free")
	ch <- prometheus.MustNewConstMetric(c.swap, prometheus.GaugeValue, kv["SwapTotal"]-kv["SwapFree"], "used")

	// CMA (Contiguous Memory Allocator) pool — used heavily by RK3588 media IP.
	if kv["CmaTotal"] > 0 {
		emitMem("cma_total", kv["CmaTotal"])
		emitMem("cma_free", kv["CmaFree"])
		emitMem("cma_used", kv["CmaTotal"]-kv["CmaFree"])
	}
}
