package collectors

import (
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// NPUCollector reads per-core NPU load from debugfs (requires root) and the
// shared NPU frequency from devfreq (no root). Load metrics are simply absent
// when debugfs is not readable — the exporter degrades gracefully.
type NPUCollector struct {
	loadRe *regexp.Regexp
	load   *prometheus.Desc
	freq   *prometheus.Desc
}

func NewNPUCollector() *NPUCollector {
	return &NPUCollector{
		loadRe: regexp.MustCompile(`Core(\d+):\s*(\d+)%`),
		load: prometheus.NewDesc(Namespace+"_npu_load_percent",
			"Per-core NPU load percent (0-100). Requires root (debugfs).",
			[]string{"core"}, nil),
		freq: prometheus.NewDesc(Namespace+"_npu_freq_mhz",
			"NPU current frequency in MHz (shared across cores).", nil, nil),
	}
}

func (c *NPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.load
	ch <- c.freq
}

func (c *NPUCollector) Collect(ch chan<- prometheus.Metric) {
	// Load: "NPU load:  Core0:  0%, Core1:  0%, Core2:  0%,"
	if s, err := readTrim(Sys("kernel/debug/rknpu/load")); err == nil {
		for _, m := range c.loadRe.FindAllStringSubmatch(s, -1) {
			pct, _ := strconv.ParseFloat(m[2], 64)
			ch <- prometheus.MustNewConstMetric(c.load, prometheus.GaugeValue, pct, m[1])
		}
	}
	// Frequency (Hz -> MHz), no root needed.
	if hz, err := readFloat(Sys("class/devfreq/fdab0000.npu/cur_freq")); err == nil {
		ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6)
	}
}
