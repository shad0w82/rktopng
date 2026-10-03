package collectors

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// CPUCollector exposes per-core usage and frequency plus a few /proc/stat
// aggregates. Per-core usage is computed from the delta between scrapes.
type CPUCollector struct {
	usage    *prometheus.Desc
	freq     *prometheus.Desc
	procs    *prometheus.Desc
	ctxt     *prometheus.Desc
	cpuSecs  *prometheus.Desc
	governor *prometheus.Desc
	cluster  map[int]string // core index -> "little"/"big"

	mu   sync.Mutex
	prev map[int][2]uint64 // core -> {total, idle}
}

func NewCPUCollector() *CPUCollector {
	return &CPUCollector{
		usage: prometheus.NewDesc(Namespace+"_cpu_usage_percent",
			"Per-core CPU usage percent (0-100).",
			[]string{"core", "cluster"}, nil),
		freq: prometheus.NewDesc(Namespace+"_cpu_freq_mhz",
			"Per-core current CPU frequency in MHz.",
			[]string{"core", "cluster"}, nil),
		procs: prometheus.NewDesc(Namespace+"_procs",
			"Process counts from /proc/stat.",
			[]string{"state"}, nil),
		ctxt: prometheus.NewDesc(Namespace+"_context_switches_total",
			"Total context switches since boot.", nil, nil),
		cpuSecs: prometheus.NewDesc(Namespace+"_cpu_seconds_total",
			"Aggregate CPU time spent in each mode, in seconds (counter).",
			[]string{"mode"}, nil),
		governor: prometheus.NewDesc(Namespace+"_cpu_governor_info",
			"Active CPU frequency governor (value is always 1).",
			[]string{"governor"}, nil),
		cluster: detectClusters(),
		prev:    map[int][2]uint64{},
	}
}

func (c *CPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.usage
	ch <- c.freq
	ch <- c.procs
	ch <- c.ctxt
	ch <- c.cpuSecs
	ch <- c.governor
}

func (c *CPUCollector) Collect(ch chan<- prometheus.Metric) {
	data, err := os.ReadFile(Proc("stat"))
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, line := range strings.Split(string(data), "\n") {
		f := fields(line)
		if len(f) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(f[0], "cpu") && len(f[0]) > 3:
			idx, err := strconv.Atoi(f[0][3:])
			if err != nil || len(f) < 8 {
				continue
			}
			var total, idle uint64
			for i := 1; i < len(f); i++ {
				v, _ := strconv.ParseUint(f[i], 10, 64)
				total += v
				if i == 4 || i == 5 { // idle + iowait
					idle += v
				}
			}
			cl := c.cluster[idx]
			if p, ok := c.prev[idx]; ok {
				dt := float64(total - p[0])
				di := float64(idle - p[1])
				if dt > 0 {
					usage := (dt - di) / dt * 100
					ch <- prometheus.MustNewConstMetric(c.usage, prometheus.GaugeValue,
						usage, strconv.Itoa(idx), cl)
				}
			}
			c.prev[idx] = [2]uint64{total, idle}

			if mhz, ok := coreFreqMHz(idx); ok {
				ch <- prometheus.MustNewConstMetric(c.freq, prometheus.GaugeValue,
					mhz, strconv.Itoa(idx), cl)
			}
		case f[0] == "cpu" && len(f) >= 8:
			// Aggregate CPU time breakdown, exposed as counters (seconds).
			modes := []string{"user", "nice", "system", "idle", "iowait", "irq", "softirq"}
			for i, mode := range modes {
				jiffies, _ := strconv.ParseUint(f[i+1], 10, 64)
				ch <- prometheus.MustNewConstMetric(c.cpuSecs, prometheus.CounterValue,
					float64(jiffies)/clkTck, mode)
			}
		case f[0] == "ctxt" && len(f) >= 2:
			v, _ := strconv.ParseFloat(f[1], 64)
			ch <- prometheus.MustNewConstMetric(c.ctxt, prometheus.CounterValue, v)
		case f[0] == "procs_running" && len(f) >= 2:
			v, _ := strconv.ParseFloat(f[1], 64)
			ch <- prometheus.MustNewConstMetric(c.procs, prometheus.GaugeValue, v, "running")
		case f[0] == "procs_blocked" && len(f) >= 2:
			v, _ := strconv.ParseFloat(f[1], 64)
			ch <- prometheus.MustNewConstMetric(c.procs, prometheus.GaugeValue, v, "blocked")
		}
	}

	// Active frequency governor (read from cpu0; RK3588 clusters usually share it).
	if gov, err := readTrim(Sys("devices/system/cpu/cpu0/cpufreq/scaling_governor")); err == nil && gov != "" {
		ch <- prometheus.MustNewConstMetric(c.governor, prometheus.GaugeValue, 1, gov)
	}
}

// clkTck is the kernel's USER_HZ (jiffies per second); 100 on effectively all
// Linux/arm64 kernels, used to convert /proc/stat jiffies to seconds.
const clkTck = 100.0

func coreFreqMHz(idx int) (float64, bool) {
	khz, err := readUint(Sys("devices/system/cpu/cpu" + strconv.Itoa(idx) + "/cpufreq/scaling_cur_freq"))
	if err != nil {
		return 0, false
	}
	return float64(khz) / 1000.0, true
}

// detectClusters maps each CPU core to "little" or "big" using the ARM part
// number in /proc/cpuinfo (0xd05 = A55 little, 0xd0b = A76 big on RK3588).
func detectClusters() map[int]string {
	out := map[int]string{}
	data, err := os.ReadFile(Proc("cpuinfo"))
	if err != nil {
		return out
	}
	idx := -1
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				idx, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		} else if strings.HasPrefix(line, "CPU part") && idx >= 0 {
			_, v, _ := strings.Cut(line, ":")
			switch strings.TrimSpace(v) {
			case "0xd05", "0xd04", "0xd03": // A55 / A35 / A53
				out[idx] = "little"
			default: // A76 (0xd0b) and other big cores
				out[idx] = "big"
			}
		}
	}
	return out
}
