package collectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// RGACollector reads RGA (2D graphics accelerator) per-scheduler load and
// frequency. Both live in debugfs and require root; metrics are absent when
// not readable (graceful degradation).
type RGACollector struct {
	schedRe *regexp.Regexp
	loadRe  *regexp.Regexp
	load    *prometheus.Desc
	freq    *prometheus.Desc
}

func NewRGACollector() *RGACollector {
	return &RGACollector{
		schedRe: regexp.MustCompile(`scheduler\[(\d+)\]:\s*(\S+)`),
		loadRe:  regexp.MustCompile(`load\s*=\s*(\d+)%`),
		load: prometheus.NewDesc(Namespace+"_rga_load_percent",
			"Per-scheduler RGA load percent (0-100). Requires root (debugfs).",
			[]string{"scheduler"}, nil),
		freq: prometheus.NewDesc(Namespace+"_rga_freq_mhz",
			"Per-core RGA frequency in MHz. Requires root (debugfs).",
			[]string{"scheduler"}, nil),
	}
}

func (c *RGACollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.load
	ch <- c.freq
}

func (c *RGACollector) Collect(ch chan<- prometheus.Metric) {
	// Load: pair each "scheduler[N]: name" with the following "load = X%".
	if s, err := readTrim(Sys("kernel/debug/rkrga/load")); err == nil {
		var cur string
		for _, line := range strings.Split(s, "\n") {
			if m := c.schedRe.FindStringSubmatch(line); m != nil {
				cur = m[2] + "_" + m[1] // e.g. rga3_0, rga3_1, rga2_2
			} else if m := c.loadRe.FindStringSubmatch(line); m != nil && cur != "" {
				pct, _ := strconv.ParseFloat(m[1], 64)
				ch <- prometheus.MustNewConstMetric(c.load, prometheus.GaugeValue, pct, cur)
			}
		}
	}

	// Frequency. debugfs has a tiny clk_rate file per clock; clk_summary is a
	// ~270 KB dump of the whole clock tree that costs ~14 ms of kernel time to
	// produce, so it is only a fallback for kernels without the per-clock files.
	found := false
	for _, rc := range rgaClocks {
		if hz, err := readFloat(Sys("kernel/debug/clk/" + rc.clock + "/clk_rate")); err == nil {
			ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6, rc.scheduler)
			found = true
		}
	}
	if found {
		return
	}
	clkToSched := map[string]string{}
	for _, rc := range rgaClocks {
		clkToSched[rc.clock] = rc.scheduler
	}
	if s, err := readTrim(Sys("kernel/debug/clk/clk_summary")); err == nil {
		for _, line := range strings.Split(s, "\n") {
			f := fields(line)
			if len(f) < 5 {
				continue
			}
			if sched, ok := clkToSched[f[0]]; ok {
				if hz, err := strconv.ParseFloat(f[4], 64); err == nil {
					ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6, sched)
				}
			}
		}
	}
}

// rgaClocks maps each RGA core clock to the scheduler name used by the load file.
var rgaClocks = []struct{ clock, scheduler string }{
	{"clk_rga3_0_core", "rga3_0"},
	{"clk_rga3_1_core", "rga3_1"},
	{"clk_rga2_core", "rga2_2"},
}
