package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// smartResult is the last SMART data read from one disk.
type smartResult struct {
	Model, Firmware string
	Temp            *float64 // °C (smartctl's normalised "temperature.current")
	Healthy         *bool    // overall SMART health self-assessment
	PowerOnHours    *float64
	OK              bool // did the most recent poll succeed
}

// smartctlJSON is the subset of `smartctl -j` output the exporter needs.
type smartctlJSON struct {
	ModelName       string `json:"model_name"`
	FirmwareVersion string `json:"firmware_version"`
	Temperature     *struct {
		Current *float64 `json:"current"`
	} `json:"temperature"`
	SmartStatus *struct {
		Passed *bool `json:"passed"`
	} `json:"smart_status"`
	PowerOnTime *struct {
		Hours *float64 `json:"hours"`
	} `json:"power_on_time"`
	Smartctl struct {
		ExitStatus int `json:"exit_status"`
		Messages   []struct {
			String   string `json:"string"`
			Severity string `json:"severity"`
		} `json:"messages"`
	} `json:"smartctl"`
}

// parseSmartctl turns `smartctl -j` output into a smartResult. smartctl's exit
// status is a bit mask: bits 0-1 mean it could not even run/open the device
// (permissions, unsupported, standby); the higher bits only report disk problems
// and still come with valid data.
func parseSmartctl(b []byte) (smartResult, error) {
	var j smartctlJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return smartResult{}, err
	}
	if j.Smartctl.ExitStatus&3 != 0 {
		msg := "smartctl could not read the device"
		for _, m := range j.Smartctl.Messages {
			if m.Severity == "error" || msg == "smartctl could not read the device" {
				msg = m.String
			}
		}
		return smartResult{}, errors.New(msg)
	}
	r := smartResult{Model: j.ModelName, Firmware: j.FirmwareVersion, OK: true}
	if j.Temperature != nil {
		r.Temp = j.Temperature.Current
	}
	if j.SmartStatus != nil {
		r.Healthy = j.SmartStatus.Passed
	}
	if j.PowerOnTime != nil {
		r.PowerOnHours = j.PowerOnTime.Hours
	}
	return r, nil
}

// smartRunner runs a command and returns its stdout. smartctl exits non-zero for
// many harmless reasons, so a failing exit status is not an error as long as it
// printed something.
type smartRunner func(ctx context.Context, bin string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, bin string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	var ee *exec.ExitError
	if err != nil && errors.As(err, &ee) && len(out) > 0 {
		return out, nil
	}
	return out, err
}

// smartDisks are the disks smartctl can talk to (not eMMC/SD cards).
func smartDisks() []string {
	var out []string
	for _, d := range wholeDisks() {
		switch diskBus(d) {
		case "nvme", "sata", "scsi", "usb":
			out = append(out, d)
		}
	}
	return out
}

// SMARTCollector polls `smartctl` on every disk in the background (SMART queries
// are slow and the values change slowly) and serves the cached results on
// scrape. It needs root (or CAP_SYS_RAWIO) and the smartmontools package; when
// smartctl is missing or the disks are unreadable it simply emits nothing.
type SMARTCollector struct {
	bin      string
	interval time.Duration
	run      smartRunner
	disks    func() []string

	mu     sync.RWMutex
	cache  map[string]smartResult
	warned map[string]bool

	temp, healthy, hours, info, up *prometheus.Desc
}

func newSMARTCollector(bin string, interval time.Duration, run smartRunner, disks func() []string) *SMARTCollector {
	dev := []string{"device"}
	return &SMARTCollector{
		bin: bin, interval: interval, run: run, disks: disks,
		cache: map[string]smartResult{}, warned: map[string]bool{},
		temp: prometheus.NewDesc(Namespace+"_smart_temperature_celsius",
			"Disk temperature reported by SMART, in degrees Celsius.", dev, nil),
		healthy: prometheus.NewDesc(Namespace+"_smart_healthy",
			"SMART overall health self-assessment: 1 = PASSED, 0 = FAILED.", dev, nil),
		hours: prometheus.NewDesc(Namespace+"_smart_power_on_hours",
			"Total power-on time in hours, from SMART.", dev, nil),
		info: prometheus.NewDesc(Namespace+"_smart_info",
			"Disk identity from SMART (value is always 1). The serial number is deliberately not exported here: it is only in /api/smart/<device>.", []string{"device", "model", "firmware"}, nil),
		up: prometheus.NewDesc(Namespace+"_smart_up",
			"1 if the last SMART poll of the disk succeeded (0: unreadable, e.g. no permission or in standby).", dev, nil),
	}
}

// NewSMARTCollector builds the collector from the environment and starts its
// background poller:
//
//	RKTOP_SMART           off|0|false disables SMART polling
//	RKTOP_SMARTCTL        path of the smartctl binary (default "smartctl")
//	RKTOP_SMART_INTERVAL  poll period (default 60s)
func NewSMARTCollector() *SMARTCollector {
	interval, err := time.ParseDuration(getenv("RKTOP_SMART_INTERVAL", "60s"))
	if err != nil || interval < 5*time.Second {
		interval = 60 * time.Second
	}
	c := newSMARTCollector(getenv("RKTOP_SMARTCTL", "smartctl"), interval, execRunner, smartDisks)
	switch strings.ToLower(os.Getenv("RKTOP_SMART")) {
	case "off", "0", "false", "no":
		log.Printf("SMART polling disabled (RKTOP_SMART)")
	default:
		go c.loop()
	}
	return c
}

func (c *SMARTCollector) loop() {
	if _, err := exec.LookPath(c.bin); err != nil {
		log.Printf("SMART: %q not found, SMART metrics disabled (install smartmontools)", c.bin)
		return
	}
	for {
		c.refresh(context.Background())
		time.Sleep(c.interval)
	}
}

// refresh polls every SMART-capable disk once.
func (c *SMARTCollector) refresh(ctx context.Context) {
	for _, name := range c.disks() {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		// -n standby: never wake a spun-down hard disk just to read SMART.
		out, err := c.run(cctx, c.bin, "-j", "-n", "standby", "-i", "-H", "-A", filepath.Join(DevPath, name))
		cancel()

		var res smartResult
		if err == nil {
			res, err = parseSmartctl(out)
		}

		c.mu.Lock()
		if err != nil {
			prev := c.cache[name] // keep the last good values, but flag the poll as failed
			prev.OK = false
			c.cache[name] = prev
			if !c.warned[name] {
				c.warned[name] = true
				log.Printf("SMART: %s: %v", name, err)
			}
		} else {
			c.cache[name] = res
			delete(c.warned, name)
		}
		c.mu.Unlock()
	}
}

// snapshot returns a copy of the cached results.
func (c *SMARTCollector) snapshot() map[string]smartResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]smartResult, len(c.cache))
	for k, v := range c.cache {
		out[k] = v
	}
	return out
}

func (c *SMARTCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.temp
	ch <- c.healthy
	ch <- c.hours
	ch <- c.info
	ch <- c.up
}

func (c *SMARTCollector) Collect(ch chan<- prometheus.Metric) {
	for dev, r := range c.snapshot() {
		up := 0.0
		if r.OK {
			up = 1
		}
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, up, dev)
		if r.Model != "" || r.Firmware != "" {
			ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1, dev, r.Model, r.Firmware)
		}
		if r.Temp != nil {
			ch <- prometheus.MustNewConstMetric(c.temp, prometheus.GaugeValue, *r.Temp, dev)
		}
		if r.Healthy != nil {
			v := 0.0
			if *r.Healthy {
				v = 1
			}
			ch <- prometheus.MustNewConstMetric(c.healthy, prometheus.GaugeValue, v, dev)
		}
		if r.PowerOnHours != nil {
			ch <- prometheus.MustNewConstMetric(c.hours, prometheus.GaugeValue, *r.PowerOnHours, dev)
		}
	}
}
