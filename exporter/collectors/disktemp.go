package collectors

import "github.com/prometheus/client_golang/prometheus"

// DiskTempCollector exposes ONE temperature per disk, whatever the source, keyed
// by the same device name as every other disk metric (nvme0n1, sda, ...). NVMe
// drives come from hwmon (live, no root); the others from SMART (polled slowly).
// The source is in the "source" label. eMMC exposes no sensor, so it is absent.
type DiskTempCollector struct {
	temp  *prometheus.Desc
	smart *SMARTCollector // may be nil
}

func NewDiskTempCollector(smart *SMARTCollector) *DiskTempCollector {
	return &DiskTempCollector{
		temp: prometheus.NewDesc(Namespace+"_disk_temp_celsius",
			"Disk temperature in degrees Celsius; source is hwmon (NVMe, live) or smart (slow poll).",
			[]string{"device", "source"}, nil),
		smart: smart,
	}
}

func (c *DiskTempCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.temp }

func (c *DiskTempCollector) Collect(ch chan<- prometheus.Metric) {
	done := map[string]bool{}
	// NVMe: the Composite sensor (falling back to the first one the drive exposes).
	first := map[string]nvmeSensor{}
	for _, s := range nvmeSensors() {
		if _, ok := first[s.device]; !ok || s.sensor == "composite" {
			first[s.device] = s
		}
	}
	for dev, s := range first {
		ch <- prometheus.MustNewConstMetric(c.temp, prometheus.GaugeValue, s.celsius, dev, "hwmon")
		done[dev] = true
	}
	// Everything else: SMART.
	if c.smart != nil {
		for dev, r := range c.smart.snapshot() {
			if done[dev] || r.Temp == nil {
				continue
			}
			ch <- prometheus.MustNewConstMetric(c.temp, prometheus.GaugeValue, *r.Temp, dev, "smart")
		}
	}
}
