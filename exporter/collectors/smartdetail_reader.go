package collectors

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrUnknownDisk is returned for a device that is not one of the system's SMART-capable disks.
var ErrUnknownDisk = errors.New("unknown disk")

const (
	smartDetailTTL     = 30 * time.Second // a report is reused for this long, however many windows ask
	smartDetailFailTTL = 8 * time.Second  // an unreadable disk (standby, no permission) is retried sooner
	smartDetailTimeout = 25 * time.Second
)

// SMARTDetailReader produces the full S.M.A.R.T. report of one disk on demand, for
// /api/smart/<dev>. Unlike SMARTCollector (a slow background poll of a few figures) it
// runs smartctl only when somebody asks, shares one run between simultaneous requests,
// keeps the result for a short while and never runs two smartctl at once.
type SMARTDetailReader struct {
	bin     string
	enabled bool
	run     smartRunner
	disks   func() []string
	args    func(dev string) []string
	extras  func(dev string) detailExtras
	now     func() time.Time
	lookup  func(bin string) error // nil: do not check that smartctl is installed (tests)

	mu       sync.Mutex
	cache    map[string]smartCached
	inflight map[string]*smartCall
	gate     chan struct{}
}

type smartCached struct {
	d  SmartDetail
	at time.Time
}

type smartCall struct {
	done chan struct{}
	d    SmartDetail
}

// NewSMARTDetailReader builds the reader from the environment (RKTOP_SMART=off disables
// it, RKTOP_SMARTCTL names the binary).
func NewSMARTDetailReader() *SMARTDetailReader {
	r := newSMARTDetailReader(getenv("RKTOP_SMARTCTL", "smartctl"), smartEnabled(), execRunner, smartDisks, detailArgs, readDetailExtras)
	r.lookup = func(bin string) error { _, err := exec.LookPath(bin); return err }
	return r
}

func newSMARTDetailReader(bin string, enabled bool, run smartRunner, disks func() []string, args func(string) []string, extras func(string) detailExtras) *SMARTDetailReader {
	return &SMARTDetailReader{bin: bin, enabled: enabled, run: run, disks: disks, args: args, extras: extras, now: time.Now,
		cache: map[string]smartCached{}, inflight: map[string]*smartCall{}, gate: make(chan struct{}, 1)}
}

func smartEnabled() bool {
	switch strings.ToLower(os.Getenv("RKTOP_SMART")) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// detailArgs: NVMe drives get -a (their self-test log is read when they have one), everything
// else -x (adds the standard device-statistics and SCT logs). -n standby never wakes a sleeping disk.
func detailArgs(dev string) []string {
	path := filepath.Join(DevPath, dev)
	if diskBus(dev) == "nvme" {
		return []string{"-j", "-a", path}
	}
	return []string{"-j", "-x", "-n", "standby", path}
}

// Get returns the report of a disk, ErrUnknownDisk if it is not one we can query.
func (r *SMARTDetailReader) Get(ctx context.Context, dev string) (SmartDetail, error) {
	known := false
	for _, d := range r.disks() {
		if d == dev {
			known = true
			break
		}
	}
	if !known {
		return SmartDetail{}, ErrUnknownDisk
	}
	if !r.enabled {
		return unavailableDetail(dev, r.now(), "disabled", "S.M.A.R.T. is turned off (RKTOP_SMART)."), nil
	}

	r.mu.Lock()
	if c, ok := r.cache[dev]; ok {
		ttl := smartDetailTTL
		if !c.d.Available {
			ttl = smartDetailFailTTL
		}
		if r.now().Sub(c.at) < ttl {
			r.mu.Unlock()
			return c.d, nil
		}
	}
	call, running := r.inflight[dev]
	if !running {
		call = &smartCall{done: make(chan struct{})}
		r.inflight[dev] = call
		go r.fetch(dev, call)
	}
	r.mu.Unlock()

	select {
	case <-call.done:
		return call.d, nil
	case <-ctx.Done():
		return SmartDetail{}, ctx.Err()
	}
}

func (r *SMARTDetailReader) fetch(dev string, call *smartCall) {
	r.gate <- struct{}{} // one smartctl at a time
	d := r.read(dev)
	<-r.gate

	r.mu.Lock()
	r.cache[dev] = smartCached{d: d, at: r.now()}
	delete(r.inflight, dev)
	r.mu.Unlock()
	call.d = d
	close(call.done)
}

func (r *SMARTDetailReader) read(dev string) SmartDetail {
	now := r.now()
	if r.lookup != nil && r.lookup(r.bin) != nil {
		return unavailableDetail(dev, now, "smartctl_missing", "smartctl is not installed (install smartmontools).")
	}
	ctx, cancel := context.WithTimeout(context.Background(), smartDetailTimeout)
	defer cancel()
	out, err := r.run(ctx, r.bin, r.args(dev)...)
	if err != nil && len(out) == 0 {
		return unavailableDetail(dev, now, "failed", err.Error())
	}
	return buildSmartDetail(dev, out, r.extras(dev), now)
}

// ── what the kernel knows about an NVMe drive (no root needed) ───────────────────────────

var nvmeBlockRe = regexp.MustCompile(`^(nvme\d+)n\d+$`)

// readDetailExtras reads the drive's composite-temperature limits (hwmon) and its PCIe link
// from sysfs; smartctl's JSON has neither.
func readDetailExtras(dev string) detailExtras {
	var ex detailExtras
	m := nvmeBlockRe.FindStringSubmatch(dev)
	if m == nil {
		return ex
	}
	base := Sys("class/nvme/" + m[1])
	if hw, _ := filepath.Glob(base + "/hwmon*"); len(hw) > 0 {
		ex.TempWarn, ex.TempCrit = milliCelsius(hw[0]+"/temp1_max"), milliCelsius(hw[0]+"/temp1_crit")
	}
	cs, _ := readTrim(base + "/device/current_link_speed")
	cw, _ := readTrim(base + "/device/current_link_width")
	ms, _ := readTrim(base + "/device/max_link_speed")
	mw, _ := readTrim(base + "/device/max_link_width")
	ex.LinkCurrent, ex.LinkMax = pcieLink(cs, cw), pcieLink(ms, mw)
	return ex
}

// milliCelsius reads a hwmon temperature file (thousandths of a degree), nil if absent or implausible.
func milliCelsius(path string) *float64 {
	v, err := readFloat(path)
	if err != nil || v <= 0 || v/1000 > 200 {
		return nil
	}
	return fptr(v / 1000)
}

var pcieGen = map[string]int{"2.5": 1, "5.0": 2, "8.0": 3, "16.0": 4, "32.0": 5, "64.0": 6}

// pcieLink formats sysfs link values ("8.0 GT/s PCIe", "1") as "PCIe Gen3 ×1 · 8.0 GT/s".
func pcieLink(speed, width string) string {
	f := strings.Fields(speed)
	if len(f) == 0 {
		return ""
	}
	gen, ok := pcieGen[f[0]]
	if !ok {
		if _, err := strconv.ParseFloat(f[0], 64); err != nil {
			return ""
		}
		return "PCIe " + f[0] + " GT/s ×" + width
	}
	return "PCIe Gen" + strconv.Itoa(gen) + " ×" + width + " · " + f[0] + " GT/s"
}
