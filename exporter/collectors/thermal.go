package collectors

import (
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// ThermalCollector reads every /sys/class/thermal/thermal_zone*/temp.
// Values are in milli-degrees Celsius. Readable without root.
type ThermalCollector struct {
	temp *prometheus.Desc
}

func NewThermalCollector() *ThermalCollector {
	return &ThermalCollector{
		temp: prometheus.NewDesc(Namespace+"_temp_celsius",
			"Thermal zone temperature in degrees Celsius.",
			[]string{"zone"}, nil),
	}
}

func (c *ThermalCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.temp }

func (c *ThermalCollector) Collect(ch chan<- prometheus.Metric) {
	zones, err := filepath.Glob(Sys("class/thermal/thermal_zone*"))
	if err != nil {
		return
	}
	for _, z := range zones {
		typ, err := readTrim(filepath.Join(z, "type"))
		if err != nil {
			continue
		}
		milli, err := readFloat(filepath.Join(z, "temp"))
		if err != nil {
			continue
		}
		// "soc-thermal" / "soc_thermal" -> "soc"
		zone := strings.NewReplacer("-thermal", "", "_thermal", "").Replace(typ)
		ch <- prometheus.MustNewConstMetric(c.temp, prometheus.GaugeValue,
			milli/1000.0, zone)
	}
}
