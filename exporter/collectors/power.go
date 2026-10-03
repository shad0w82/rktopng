package collectors

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PowerCollector reads CM3588 NAS sensors exposed via hwmon: NVMe temperatures,
// input voltage, USB-C PD voltage/current and fan PWM/RPM. All readable without
// root. hwmon indices are not stable across boots, so devices are matched by
// their "name" attribute (and NVMe drives by the controller they belong to).
type PowerCollector struct {
	nvmeTemp *prometheus.Desc
	vin      *prometheus.Desc
	pdVolt   *prometheus.Desc
	pdCurr   *prometheus.Desc
	fanPWM   *prometheus.Desc
	fanRPM   *prometheus.Desc
}

func NewPowerCollector() *PowerCollector {
	return &PowerCollector{
		nvmeTemp: prometheus.NewDesc(Namespace+"_nvme_temp_celsius",
			"NVMe SSD temperature in degrees Celsius.",
			[]string{"device", "sensor"}, nil),
		vin: prometheus.NewDesc(Namespace+"_input_voltage_volts",
			"Board DC input voltage in volts.", nil, nil),
		pdVolt: prometheus.NewDesc(Namespace+"_usb_pd_voltage_volts",
			"USB-C Power Delivery voltage in volts.", nil, nil),
		pdCurr: prometheus.NewDesc(Namespace+"_usb_pd_current_amps",
			"USB-C Power Delivery current in amps.", nil, nil),
		fanPWM: prometheus.NewDesc(Namespace+"_fan_pwm",
			"Fan PWM duty cycle (0-255).", []string{"fan"}, nil),
		fanRPM: prometheus.NewDesc(Namespace+"_fan_rpm",
			"Fan speed in RPM (if a tachometer is available).", []string{"fan"}, nil),
	}
}

func (c *PowerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.nvmeTemp
	ch <- c.vin
	ch <- c.pdVolt
	ch <- c.pdCurr
	ch <- c.fanPWM
	ch <- c.fanRPM
}

func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
	dirs, err := filepath.Glob(Sys("class/hwmon/hwmon*"))
	if err != nil {
		return
	}
	sort.Strings(dirs)
	for _, s := range nvmeSensors() {
		ch <- prometheus.MustNewConstMetric(c.nvmeTemp, prometheus.GaugeValue, s.celsius, s.device, s.sensor)
	}
	for _, d := range dirs {
		name, err := readTrim(filepath.Join(d, "name"))
		if err != nil {
			continue
		}
		switch {
		case name == "simple_vin":
			if mv, err := readFloat(filepath.Join(d, "in0_input")); err == nil {
				ch <- prometheus.MustNewConstMetric(c.vin, prometheus.GaugeValue, mv/1000.0)
			}
		case strings.HasPrefix(name, "tcpm_source_psy"):
			if mv, err := readFloat(filepath.Join(d, "in0_input")); err == nil {
				ch <- prometheus.MustNewConstMetric(c.pdVolt, prometheus.GaugeValue, mv/1000.0)
			}
			if ma, err := readFloat(filepath.Join(d, "curr1_input")); err == nil {
				ch <- prometheus.MustNewConstMetric(c.pdCurr, prometheus.GaugeValue, ma/1000.0)
			}
		case name == "pwmfan":
			c.collectFan(ch, d)
		}
	}
}

// nvmeSensor is one temperature reading of an NVMe drive.
type nvmeSensor struct {
	device  string // block device the drive appears as, e.g. nvme0n1 (matches disk I/O and the disk inventory)
	sensor  string // "composite", "sensor_1", ...
	celsius float64
}

var nvmeCtrlRe = regexp.MustCompile(`^nvme\d+$`)

// nvmeBlockName maps an NVMe controller (nvme0) to its first namespace's block
// device (nvme0n1), which is the name every other metric uses for the disk.
func nvmeBlockName(ctrl string) string {
	if exists(Sys("block/" + ctrl + "n1")) {
		return ctrl + "n1"
	}
	if m, _ := filepath.Glob(Sys("class/nvme/" + ctrl + "/" + ctrl + "n*")); len(m) > 0 {
		return filepath.Base(m[0])
	}
	return ctrl
}

// nvmeCacheTTL: reading an NVMe hwmon temperature makes the kernel send a SMART
// log command to the drive (~6 ms each, 9 reads for 3 drives). Two collectors
// need these values and the UI refreshes every second, so the readings are
// shared and refreshed at most this often; drive temperatures move slowly.
const nvmeCacheTTL = 3 * time.Second

var nvmeCache struct {
	mu  sync.Mutex
	sys string // SysPath the readings belong to (tests use throw-away trees)
	at  time.Time
	val []nvmeSensor
}

// nvmeSensors returns the NVMe temperatures, from a short-lived shared cache.
// The returned slice is shared: callers must not modify it.
func nvmeSensors() []nvmeSensor {
	nvmeCache.mu.Lock()
	defer nvmeCache.mu.Unlock()
	if nvmeCache.sys == SysPath && !nvmeCache.at.IsZero() && time.Since(nvmeCache.at) < nvmeCacheTTL {
		return nvmeCache.val
	}
	nvmeCache.val = readNVMeSensors()
	nvmeCache.sys, nvmeCache.at = SysPath, time.Now()
	return nvmeCache.val
}

// readNVMeSensors reads every NVMe hwmon device. hwmon indices change between
// boots, so a drive is identified by the controller its hwmon "device" symlink
// points to (.../nvme/nvme0), not by enumeration order.
func readNVMeSensors() []nvmeSensor {
	dirs, _ := filepath.Glob(Sys("class/hwmon/hwmon*"))
	sort.Strings(dirs)
	var out []nvmeSensor
	fallback := 0
	for _, d := range dirs {
		if name, err := readTrim(filepath.Join(d, "name")); err != nil || name != "nvme" {
			continue
		}
		ctrl := ""
		if target, err := os.Readlink(filepath.Join(d, "device")); err == nil {
			if base := filepath.Base(target); nvmeCtrlRe.MatchString(base) {
				ctrl = base
			}
		}
		if ctrl == "" { // symlink missing: fall back to enumeration order
			ctrl = "nvme" + strconv.Itoa(fallback)
		}
		fallback++
		dev := nvmeBlockName(ctrl)
		for i := 1; i <= 3; i++ {
			si := strconv.Itoa(i)
			milli, err := readFloat(filepath.Join(d, "temp"+si+"_input"))
			if err != nil {
				continue
			}
			sensor := "temp" + si
			if label, err := readTrim(filepath.Join(d, "temp"+si+"_label")); err == nil && label != "" {
				sensor = strings.ToLower(strings.ReplaceAll(label, " ", "_"))
			}
			out = append(out, nvmeSensor{device: dev, sensor: sensor, celsius: milli / 1000.0})
		}
	}
	return out
}

func (c *PowerCollector) collectFan(ch chan<- prometheus.Metric, dir string) {
	for i := 1; i <= 5; i++ {
		si := strconv.Itoa(i)
		if v, err := readFloat(filepath.Join(dir, "pwm"+si)); err == nil {
			ch <- prometheus.MustNewConstMetric(c.fanPWM, prometheus.GaugeValue, v, si)
		}
		// fan*_input is often empty on CM3588 (no tachometer); skipped on error.
		if v, err := readFloat(filepath.Join(dir, "fan"+si+"_input")); err == nil {
			ch <- prometheus.MustNewConstMetric(c.fanRPM, prometheus.GaugeValue, v, si)
		}
	}
}
