package collectors

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// VPUCollector reads the Rockchip MPP (Media Process Platform) load table from
// /proc/mpp_service/load — encoder/decoder/JPEG/AV1/IEP units. Readable without
// root. Line format: "<addr>.<unit>   load:  X% utilization:  Y%".
type VPUCollector struct {
	load     *prometheus.Desc
	util     *prometheus.Desc
	freq     *prometheus.Desc
	sessions *prometheus.Desc
}

// Friendly names for the known video units on RK3588 (CM3588).
var vpuNames = map[string]string{
	"fdbd0000.rkvenc-core": "enc_core0",
	"fdbe0000.rkvenc-core": "enc_core1",
	"fdc38100.rkvdec-core": "dec_core0",
	"fdc48100.rkvdec-core": "dec_core1",
	"fdc70000.av1d":        "av1_decoder",
	"fdb51000.avsd-plus":   "avs_decoder",
	"fdb50400.vdpu":        "vdpu",
	"fdb90000.jpegd":       "jpeg_decoder",
	"fdbb0000.iep":         "iep",
	"fdba0000.jpege-core":  "jpeg_enc0",
	"fdba4000.jpege-core":  "jpeg_enc1",
	"fdba8000.jpege-core":  "jpeg_enc2",
	"fdbac000.jpege-core":  "jpeg_enc3",
}

func NewVPUCollector() *VPUCollector {
	return &VPUCollector{
		load: prometheus.NewDesc(Namespace+"_vpu_load_percent",
			"Per-unit VPU load percent (0-100), from /proc/mpp_service/load.",
			[]string{"unit"}, nil),
		util: prometheus.NewDesc(Namespace+"_vpu_utilization_percent",
			"Per-unit VPU utilization percent (0-100), from /proc/mpp_service/load.",
			[]string{"unit"}, nil),
		freq: prometheus.NewDesc(Namespace+"_vpu_freq_mhz",
			"Per-unit VPU frequency in MHz (units that expose a devfreq node).",
			[]string{"unit"}, nil),
		sessions: prometheus.NewDesc(Namespace+"_vpu_sessions",
			"Number of active MPP sessions (encode/decode jobs).", nil, nil),
	}
}

func (c *VPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.load
	ch <- c.util
	ch <- c.freq
	ch <- c.sessions
}

func (c *VPUCollector) Collect(ch chan<- prometheus.Metric) {
	s, err := readTrim(Proc("mpp_service/load"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(s, "\n") {
		f := fields(line)
		// f: [device, "load:", "X%", "utilization:", "Y%"]
		if len(f) < 3 {
			continue
		}
		unit, ok := vpuNames[f[0]]
		if !ok {
			// Fall back to the device string minus the address prefix.
			if _, name, cut := strings.Cut(f[0], "."); cut {
				unit = name
			} else {
				unit = f[0]
			}
		}
		if v, err := strconv.ParseFloat(strings.TrimSuffix(f[2], "%"), 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.load, prometheus.GaugeValue, v, unit)
		}
		if len(f) >= 5 {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(f[4], "%"), 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.util, prometheus.GaugeValue, v, unit)
			}
		}
		// Frequency, for units that expose a devfreq node (e.g. rkvenc cores).
		if hz, err := readFloat(Sys("class/devfreq/" + f[0] + "/cur_freq")); err == nil {
			ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue, hz/1e6, unit)
		}
	}

	// Active encode/decode sessions (one "device:" line each in sessions-summary).
	if s, err := readTrim(Proc("mpp_service/sessions-summary")); err == nil {
		n := float64(strings.Count(s, "device:"))
		ch <- prometheus.MustNewConstMetric(c.sessions, prometheus.GaugeValue, n)
	}
}
