package collectors

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Block devices that are never real, user-visible disks.
var skipDiskPrefixes = []string{"loop", "ram", "zram", "sr", "dm-", "md", "nbd", "zd", "fd"}

var (
	mmcHiddenRe  = regexp.MustCompile(`^mmcblk\d+(boot\d+|rpmb)$`) // eMMC boot partitions / RPMB
	nvmeHiddenRe = regexp.MustCompile(`^nvme\d+c\d+n\d+$`)         // multipath "hidden" namespaces
)

// isWholeDisk reports whether name (an entry of /sys/block) is a real, whole disk.
// Partitions are not listed under /sys/block, so listing there already excludes them.
func isWholeDisk(name string) bool {
	for _, p := range skipDiskPrefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	if mmcHiddenRe.MatchString(name) || nvmeHiddenRe.MatchString(name) {
		return false
	}
	return exists(Sys("block/" + name))
}

// wholeDisks lists the system's real disks (sorted), e.g. nvme0n1, sda, mmcblk2.
func wholeDisks() []string {
	entries, err := os.ReadDir(Sys("block"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if isWholeDisk(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// diskBus classifies how a disk is attached: nvme, sata, usb, emmc, sd, scsi,
// virtio or other. It looks at the /sys/block symlink target (which embeds the
// device path) and falls back to the device name.
func diskBus(name string) string {
	target, _ := os.Readlink(Sys("block/" + name))
	switch {
	case strings.HasPrefix(name, "nvme") || strings.Contains(target, "/nvme/"):
		return "nvme"
	case strings.HasPrefix(name, "mmcblk"):
		if typ, _ := readTrim(Sys("block/" + name + "/device/type")); typ == "SD" {
			return "sd"
		}
		return "emmc"
	case strings.Contains(target, "/usb"):
		return "usb"
	case strings.Contains(target, "/ata"):
		return "sata"
	case strings.HasPrefix(name, "vd"):
		return "virtio"
	case strings.HasPrefix(name, "sd"), strings.HasPrefix(name, "hd"):
		return "scsi"
	}
	return "other"
}

// diskModel returns the model string of a disk when the kernel exposes one.
func diskModel(name string) string {
	for _, f := range []string{"device/model", "device/name"} {
		if s, err := readTrim(Sys("block/" + name + "/" + f)); err == nil && s != "" {
			return s
		}
	}
	return ""
}

// DiskInfoCollector exposes a static inventory of the system's disks (model, bus,
// size) — what the dashboard's "disks" tiles show.
type DiskInfoCollector struct {
	info *prometheus.Desc
	size *prometheus.Desc
}

func NewDiskInfoCollector() *DiskInfoCollector {
	return &DiskInfoCollector{
		info: prometheus.NewDesc(Namespace+"_disk_info",
			"Disk inventory (value is always 1): model, vendor, bus (nvme/sata/usb/emmc/...) and whether it is rotational.",
			[]string{"device", "model", "vendor", "bus", "rotational"}, nil),
		size: prometheus.NewDesc(Namespace+"_disk_size_bytes",
			"Disk capacity in bytes.", []string{"device"}, nil),
	}
}

func (c *DiskInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.size
}

func (c *DiskInfoCollector) Collect(ch chan<- prometheus.Metric) {
	for _, name := range wholeDisks() {
		vendor, _ := readTrim(Sys("block/" + name + "/device/vendor"))
		rot := "0"
		if v, err := readTrim(Sys("block/" + name + "/queue/rotational")); err == nil && v == "1" {
			rot = "1"
		}
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1,
			name, diskModel(name), vendor, diskBus(name), rot)
		if sectors, err := readUint(Sys("block/" + name + "/size")); err == nil {
			ch <- prometheus.MustNewConstMetric(c.size, prometheus.GaugeValue,
				float64(sectors)*512, name) // /sys/block/*/size is always in 512-byte sectors
		}
	}
}
