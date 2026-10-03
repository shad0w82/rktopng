package collectors

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// InfoCollector exposes static board information as a single info metric
// (value 1, all details in labels) — the data behind rktop's SYS panel.
type InfoCollector struct {
	desc *prometheus.Desc
}

func NewInfoCollector() *InfoCollector {
	return &InfoCollector{
		desc: prometheus.NewDesc(
			Namespace+"_soc_info",
			"Static board/SoC information (value is always 1).",
			[]string{"vendor", "soc", "model", "kernel", "os", "npu_driver", "rga_driver", "vpu_driver", "gpu_driver"},
			nil,
		),
	}
}

func (c *InfoCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *InfoCollector) Collect(ch chan<- prometheus.Metric) {
	vendor, soc := socVendorAndModel()
	model := boardModel()
	kernel, _ := readTrim(Proc("sys/kernel/osrelease"))
	osName := osPrettyName()
	npuDrv, rgaDrv, vpuDrv, gpuDrv := npuDriver(), rgaDriver(), vpuDriver(), gpuDriver()

	ch <- prometheus.MustNewConstMetric(
		c.desc, prometheus.GaugeValue, 1,
		vendor, soc, model, kernel, osName, npuDrv, rgaDrv, vpuDrv, gpuDrv,
	)
}

// osPrettyName returns the host OS's PRETTY_NAME (e.g. "Ubuntu 24.04.4 LTS") from
// os-release, or "" when it cannot be read.
func osPrettyName() string {
	for _, p := range []string{OSReleasePath, "/usr/lib/os-release"} {
		if b, err := os.ReadFile(p); err == nil {
			if v := parseOSRelease(string(b)); v != "" {
				return v
			}
		}
	}
	return ""
}

// parseOSRelease extracts PRETTY_NAME (falling back to NAME + VERSION) from the
// contents of an os-release file. Values may be single- or double-quoted.
func parseOSRelease(data string) string {
	kv := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		kv[k] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	if v := kv["PRETTY_NAME"]; v != "" {
		return v
	}
	return strings.TrimSpace(kv["NAME"] + " " + kv["VERSION"])
}

// boardModel reads the device-tree model (e.g. "FriendlyElec CM3588").
func boardModel() string {
	for _, p := range []string{Proc("device-tree/model"), Sys("firmware/devicetree/base/model")} {
		if b, err := os.ReadFile(p); err == nil {
			s := strings.Trim(strings.TrimRight(string(b), "\x00"), " \n\t")
			if s != "" {
				return s
			}
		}
	}
	return "Unknown Board"
}

// socVendorAndModel parses /proc/device-tree/compatible, whose NUL-separated
// entries are "vendor,soc" pairs. Returns the vendor (capitalized) and SoC
// (uppercased) of the entry whose SoC starts with "rk" (e.g. Rockchip, RK3588).
func socVendorAndModel() (vendor, soc string) {
	vendor, soc = "Rockchip", "Unknown RK"
	for _, p := range []string{Proc("device-tree/compatible"), Sys("firmware/devicetree/base/compatible")} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, entry := range strings.Split(string(b), "\x00") {
			v, s, ok := strings.Cut(entry, ",")
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if strings.HasPrefix(strings.ToLower(s), "rk") {
				return capitalize(strings.TrimSpace(v)), strings.ToUpper(s)
			}
		}
	}
	return vendor, soc
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// driverVersion reads a "Label: vX.Y.Z" debugfs file and returns the value part.
// Requires root (debugfs); returns "" gracefully if unreadable.
func driverVersion(path string) string {
	s, err := readTrim(path)
	if err != nil {
		return ""
	}
	if _, v, ok := strings.Cut(s, ":"); ok {
		return strings.TrimSpace(v)
	}
	return s
}

// npuDriver is the RKNPU driver version ("v0.9.8"). debugfs has it (root only); the
// module's own sysfs entry ("0.9.8") is the fallback that works for any user.
func npuDriver() string {
	if v := driverVersion(Sys("kernel/debug/rknpu/version")); v != "" {
		return v
	}
	if v, err := readTrim(Sys("module/rknpu/version")); err == nil && v != "" {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return ""
}

// rgaDriver is the RGA driver version ("v1.3.10"); only debugfs (root) has it.
func rgaDriver() string {
	return driverVersion(Sys("kernel/debug/rkrga/driver_version"))
}

// vpuDriver is the MPP (video codec) kernel driver. It has no release number: the
// kernel reports the source commit, e.g. "c79104d97229 author: … 2025-08-25 video:
// rockchip: mpp: …". The line is passed on as it is; the UI shortens it.
func vpuDriver() string {
	for _, p := range []string{Proc("mpp_service/version"), Sys("module/rk_vcodec/version")} {
		if v, err := readTrim(p); err == nil && v != "" {
			line, _, _ := strings.Cut(v, "\n")
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// gpuDriver is the Mali kernel driver ("g29p0-00eac0 (UK version 1.36)"), from the
// module's sysfs entry. The module is named valhall_kbase, bifrost_kbase or mali_kbase
// depending on the GPU generation and the vendor kernel, so any *kbase* module is accepted.
func gpuDriver() string {
	matches, _ := filepath.Glob(Sys("module/*kbase*/version"))
	sort.Strings(matches)
	for _, p := range matches {
		if v, err := readTrim(p); err == nil && v != "" {
			return v
		}
	}
	return ""
}
