package collectors

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// GPUCollector reads the Mali GPU load and frequency from devfreq.
// /sys/class/devfreq/fb000000.gpu/load has the form "<load>@<freq>Hz"
// (e.g. "28@300000000Hz"). Readable without root.
type GPUCollector struct {
	base string
	load *prometheus.Desc
	freq *prometheus.Desc
}

func NewGPUCollector() *GPUCollector {
	return &GPUCollector{
		base: Sys("class/devfreq/fb000000.gpu"),
		load: prometheus.NewDesc(Namespace+"_gpu_load_percent",
			"Mali GPU load percent (0-100).", nil, nil),
		freq: prometheus.NewDesc(Namespace+"_gpu_freq_mhz",
			"Mali GPU current frequency in MHz.", nil, nil),
	}
}

func (c *GPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.load
	ch <- c.freq
}

func (c *GPUCollector) Collect(ch chan<- prometheus.Metric) {
	if s, err := readTrim(c.base + "/load"); err == nil {
		// "28@300000000Hz"
		loadStr, freqStr, ok := strings.Cut(s, "@")
		if load, err := strconv.ParseFloat(strings.TrimSpace(loadStr), 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.load, prometheus.GaugeValue, load)
		}
		if ok {
			freqStr = strings.TrimSuffix(strings.TrimSpace(freqStr), "Hz")
			if hz, err := strconv.ParseFloat(freqStr, 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6)
				return
			}
		}
	}
	// Fallback for frequency if not present in the load string.
	if hz, err := readFloat(c.base + "/cur_freq"); err == nil {
		ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6)
	}
}
