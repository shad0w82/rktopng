package collectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportableMount(t *testing.T) {
	// The mount table of the CM3588 test board: only "/", "/DATA" and "/SSD_Pool"
	// are real storage the dashboard should show.
	cases := []struct {
		mount, fstype string
		want          bool
	}{
		{"/", "overlay", true}, // root on an overlay over the eMMC
		{"/DATA", "zfs", true},
		{"/SSD_Pool", "zfs", true},
		{"/boot/efi", "vfat", true},
		{"/mnt/usb disk", "ext4", true},
		{"/var/log/journal", "overlay", false}, // secondary overlay
		{"/var/lib/docker", "zfs", false},      // docker's dataset
		{"/var/lib/docker/zfs/graph/631eb14c", "zfs", false},
		{"/var/lib/docker/overlay2/abc/merged", "overlay", false},
		{"/run/user/1000/gvfs", "fuse.gvfsd-fuse", false},
		{"/run/user/1000/doc", "fuse.portal", false},
		{"/sys/fs/fuse/connections", "fusectl", false},
		{"/snap/core24/2125", "squashfs", false},
		{"/proc", "proc", false},
		{"/dev/shm", "tmpfs", false},
		{"/dev/pts", "devpts", false},
		{"/mnt/pool", "fuse.mergerfs", true}, // a NAS union filesystem must stay visible
		{"/srv/run-data", "ext4", true},      // "run" only matters as a path prefix
	}
	for _, c := range cases {
		if got := reportableMount(c.mount, c.fstype); got != c.want {
			t.Errorf("reportableMount(%q, %q) = %v, want %v", c.mount, c.fstype, got, c.want)
		}
	}
}

func TestParseMountsUnescapes(t *testing.T) {
	got := parseMounts("NVME_Pool /DATA zfs rw,xattr 0 0\n/dev/sdc1 /mnt/my\\040disk ext4 rw 0 0\n\nbroken\n")
	if len(got) != 2 {
		t.Fatalf("parseMounts = %v", got)
	}
	if got[0] != (mountEntry{source: "NVME_Pool", mount: "/DATA", fstype: "zfs"}) {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].mount != "/mnt/my disk" {
		t.Errorf("octal escape not decoded: %q", got[1].mount)
	}
}

func TestUnescapeMountEdgeCases(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:     "plain",
		`a\040b`:    "a b",
		`a\134b`:    `a\b`,
		`trailing\`: `trailing\`,
		`bad\zzzx`:  `bad\zzzx`,
		`end\040`:   "end ",
	} {
		if got := unescapeMount(in); got != want {
			t.Errorf("unescapeMount(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFilesystemCollector reads a fake host mount table through pid 1 (as in a
// container with the host /proc mounted) and statfs()es directories under a fake
// host root.
func TestFilesystemCollector(t *testing.T) {
	proc, _ := fakeRoots(t)
	root := t.TempDir()
	for _, d := range []string{"DATA", "var/lib/docker"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := rootfsPath
	rootfsPath = root
	t.Cleanup(func() { rootfsPath = old })

	write(t, filepath.Join(proc, "1", "mounts"), "overlay / overlay rw 0 0\n"+
		"NVME_Pool /DATA zfs rw 0 0\n"+
		"NVME_Pool/docker /var/lib/docker zfs rw 0 0\n"+
		"proc /proc proc rw 0 0\n")
	// /proc/mounts (the container's own table) must NOT be used when pid 1's is readable.
	write(t, filepath.Join(proc, "mounts"), "tmpfs /bogus tmpfs rw 0 0\n")

	m := gather(t, NewFilesystemCollector())
	size := m["rk3588_filesystem_size_bytes"]
	if len(size) != 2 || !has(size, "mount", "/") || !has(size, "mount", "/DATA") {
		t.Fatalf("filesystem mounts = %v, want only / and /DATA", size)
	}
	data := find(t, size, "mount", "/DATA")
	if data.labels["fstype"] != "zfs" || data.labels["source"] != "NVME_Pool" {
		t.Errorf("/DATA labels = %v", data.labels)
	}
	if data.value <= 0 {
		t.Errorf("size = %v", data.value)
	}
	used := find(t, m["rk3588_filesystem_used_bytes"], "mount", "/DATA").value
	avail := find(t, m["rk3588_filesystem_avail_bytes"], "mount", "/DATA").value
	if used < 0 || avail <= 0 || used+avail > data.value*1.0001 {
		t.Errorf("used=%v avail=%v size=%v are inconsistent", used, avail, data.value)
	}
}

func TestReadMountsFallsBackToProcMounts(t *testing.T) {
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "mounts"), "/dev/mmcblk2p9 / ext4 rw 0 0\n")
	got := readMounts()
	if len(got) != 1 || got[0].source != "/dev/mmcblk2p9" {
		t.Errorf("readMounts = %v", got)
	}
}
