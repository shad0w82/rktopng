package collectors

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// DDRCollector reads the DDR memory controller (dmc devfreq) load and frequency.
// /sys/class/devfreq/dmc/load has the "<load>@<freq>Hz" form, like the GPU.
// Readable without root.
type DDRCollector struct {
	base string
	load *prometheus.Desc
	freq *prometheus.Desc
}

func NewDDRCollector() *DDRCollector {
	return &DDRCollector{
		base: Sys("class/devfreq/dmc"),
		load: prometheus.NewDesc(Namespace+"_ddr_load_percent",
			"DDR memory controller load percent (0-100).", nil, nil),
		freq: prometheus.NewDesc(Namespace+"_ddr_freq_mhz",
			"DDR memory controller frequency in MHz.", nil, nil),
	}
}

func (c *DDRCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.load
	ch <- c.freq
}

func (c *DDRCollector) Collect(ch chan<- prometheus.Metric) {
	if s, err := readTrim(c.base + "/load"); err == nil {
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
	if hz, err := readFloat(c.base + "/cur_freq"); err == nil {
		ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6)
	}
}
