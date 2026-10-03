package collectors

import (
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
)

// rootfsPath, when set, is prefixed to mount points before statfs() so disk
// usage can be read from a bind-mounted host root inside a container.
var rootfsPath = os.Getenv("RKTOP_ROOTFS_PATH")

// Filesystem types that never describe real storage.
var virtualFS = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "proc": true, "sysfs": true,
	"devpts": true, "cgroup": true, "cgroup2": true, "securityfs": true,
	"debugfs": true, "tracefs": true, "pstore": true, "bpf": true,
	"configfs": true, "hugetlbfs": true, "mqueue": true, "overlay": true,
	"fusectl": true, "binfmt_misc": true, "autofs": true, "ramfs": true,
	"squashfs": true, "nsfs": true, "rpc_pipefs": true, "efivarfs": true,
	"fuse.gvfsd-fuse": true, "fuse.portal": true, "fuse.lxcfs": true,
}

// Mount-point prefixes whose contents are bookkeeping, not user storage
// (container layers, snaps, runtime dirs, ...).
var skipMountPrefixes = []string{
	"/proc", "/sys", "/dev", "/run", "/snap", "/var/snap",
	"/var/lib/docker", "/var/lib/containers", "/var/lib/kubelet",
}

type mountEntry struct {
	source, mount, fstype string
}

// reportableMount decides whether a mount is real user-visible storage. Overlay
// mounts are skipped except "/" itself, because some images (FriendlyElec's, for
// instance) run the root filesystem as an overlay on top of the eMMC.
func reportableMount(mount, fstype string) bool {
	if fstype == "overlay" {
		return mount == "/"
	}
	if virtualFS[fstype] {
		return false
	}
	for _, p := range skipMountPrefixes {
		if mount == p || strings.HasPrefix(mount, p+"/") {
			return false
		}
	}
	return true
}

// unescapeMount decodes the octal escapes /proc/mounts uses for odd characters
// (a space is "\040", a tab "\011", a backslash "\134").
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseMounts parses the contents of /proc/mounts.
func parseMounts(data string) []mountEntry {
	var out []mountEntry
	for _, line := range strings.Split(data, "\n") {
		f := fields(line)
		if len(f) < 3 {
			continue
		}
		out = append(out, mountEntry{source: unescapeMount(f[0]), mount: unescapeMount(f[1]), fstype: f[2]})
	}
	return out
}

// readMounts returns the host's mount table. Inside a container /proc/self/mounts
// is the container's own table, so pid 1's is preferred (the host init when the
// host /proc is mounted; the same table on a native install). Reading pid 1 needs
// privileges, so it falls back to /proc/mounts.
func readMounts() []mountEntry {
	for _, p := range []string{Proc("1/mounts"), Proc("mounts")} {
		if b, err := os.ReadFile(p); err == nil {
			return parseMounts(string(b))
		}
	}
	return nil
}

// FilesystemCollector exposes size / used / available bytes of every real
// mounted filesystem, plus its source (device, or ZFS dataset) so the UI can
// group volumes by pool.
type FilesystemCollector struct {
	size  *prometheus.Desc
	used  *prometheus.Desc
	avail *prometheus.Desc
}

func NewFilesystemCollector() *FilesystemCollector {
	labels := []string{"mount", "fstype", "source"}
	return &FilesystemCollector{
		size: prometheus.NewDesc(Namespace+"_filesystem_size_bytes",
			"Filesystem total size in bytes.", labels, nil),
		used: prometheus.NewDesc(Namespace+"_filesystem_used_bytes",
			"Filesystem used space in bytes.", labels, nil),
		avail: prometheus.NewDesc(Namespace+"_filesystem_avail_bytes",
			"Filesystem space available to unprivileged users, in bytes. Usage as df shows it is used/(used+avail).", labels, nil),
	}
}

func (c *FilesystemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.size
	ch <- c.used
	ch <- c.avail
}

func (c *FilesystemCollector) Collect(ch chan<- prometheus.Metric) {
	seen := map[string]bool{}
	for _, m := range readMounts() {
		if !reportableMount(m.mount, m.fstype) || seen[m.mount] {
			continue
		}
		seen[m.mount] = true

		var st syscall.Statfs_t
		if err := syscall.Statfs(rootfsPath+m.mount, &st); err != nil {
			continue
		}
		bs := uint64(st.Bsize)
		size := st.Blocks * bs
		if size == 0 {
			continue
		}
		used := (st.Blocks - st.Bfree) * bs
		avail := st.Bavail * bs
		ch <- prometheus.MustNewConstMetric(c.size, prometheus.GaugeValue, float64(size), m.mount, m.fstype, m.source)
		ch <- prometheus.MustNewConstMetric(c.used, prometheus.GaugeValue, float64(used), m.mount, m.fstype, m.source)
		ch <- prometheus.MustNewConstMetric(c.avail, prometheus.GaugeValue, float64(avail), m.mount, m.fstype, m.source)
	}
}
