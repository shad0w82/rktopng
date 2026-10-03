package collectors

import (
	"path/filepath"
	"testing"
)

// buildBoardSysfs recreates the disk layout of the CM3588 test board (3 NVMe,
// 2 SATA SSD, 1 eMMC with its boot partitions) under a fake /sys.
func buildBoardSysfs(t *testing.T, sys string) {
	t.Helper()
	dev := func(p string) string { return filepath.Join(sys, "devices", p) }
	blk := func(n string) string { return filepath.Join(sys, "block", n) }

	for i, ctrl := range []string{"nvme0", "nvme1", "nvme2"} {
		d := dev("platform/pcie" + ctrl + "/pci/nvme/" + ctrl + "/" + ctrl + "n1")
		write(t, filepath.Join(d, "size"), "2000409264\n") // 1 TB
		write(t, filepath.Join(d, "device", "model"), "Lexar SSD NM790 1TB    \n")
		write(t, filepath.Join(d, "queue", "rotational"), "0\n")
		link(t, "../devices/platform/pcie"+ctrl+"/pci/nvme/"+ctrl+"/"+ctrl+"n1", blk(ctrl+"n1"))
		_ = i
	}
	for _, n := range []string{"sda", "sdb"} {
		d := dev("platform/sata/ata1/host0/target0:0:0/0:0:0:0/block/" + n)
		write(t, filepath.Join(d, "size"), "3907029168\n") // 2 TB
		write(t, filepath.Join(d, "device", "model"), "Samsung SSD 870 \n")
		write(t, filepath.Join(d, "device", "vendor"), "ATA     \n")
		write(t, filepath.Join(d, "queue", "rotational"), "0\n")
		link(t, "../devices/platform/sata/ata1/host0/target0:0:0/0:0:0:0/block/"+n, blk(n))
	}
	for _, n := range []string{"mmcblk2", "mmcblk2boot0", "mmcblk2boot1"} {
		d := dev("platform/mmc_host/mmc0/mmc0:0001/block/" + n)
		write(t, filepath.Join(d, "size"), "120832000\n")
		link(t, "../devices/platform/mmc_host/mmc0/mmc0:0001/block/"+n, blk(n))
	}
	write(t, dev("platform/mmc_host/mmc0/mmc0:0001/block/mmcblk2/device/type"), "MMC\n")
	write(t, dev("platform/mmc_host/mmc0/mmc0:0001/block/mmcblk2/device/name"), "A3A564\n")
	for _, n := range []string{"loop0", "zram0", "ram0"} {
		write(t, filepath.Join(blk(n), "size"), "8\n")
	}
}

func TestWholeDisksFiltersNonDisks(t *testing.T) {
	_, sys := fakeRoots(t)
	buildBoardSysfs(t, sys)
	got := wholeDisks()
	want := []string{"mmcblk2", "nvme0n1", "nvme1n1", "nvme2n1", "sda", "sdb"}
	if len(got) != len(want) {
		t.Fatalf("wholeDisks = %v, want %v (eMMC boot partitions, loop, zram, ram must be skipped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wholeDisks = %v, want %v", got, want)
		}
	}
}

func TestDiskBus(t *testing.T) {
	_, sys := fakeRoots(t)
	buildBoardSysfs(t, sys)
	for name, want := range map[string]string{
		"nvme0n1": "nvme", "nvme2n1": "nvme", "sda": "sata", "sdb": "sata", "mmcblk2": "emmc",
	} {
		if got := diskBus(name); got != want {
			t.Errorf("diskBus(%s) = %q, want %q", name, got, want)
		}
	}
	// An SD card is told apart from eMMC by the device type attribute.
	write(t, filepath.Join(sys, "block", "mmcblk0", "device", "type"), "SD\n")
	if got := diskBus("mmcblk0"); got != "sd" {
		t.Errorf("diskBus(mmcblk0 SD) = %q, want sd", got)
	}
}

func TestDiskInfoCollector(t *testing.T) {
	_, sys := fakeRoots(t)
	buildBoardSysfs(t, sys)
	m := gather(t, NewDiskInfoCollector())

	if n := len(m["rk3588_disk_info"]); n != 6 {
		t.Fatalf("disk_info has %d samples, want 6", n)
	}
	sda := find(t, m["rk3588_disk_info"], "device", "sda")
	if sda.labels["model"] != "Samsung SSD 870" || sda.labels["vendor"] != "ATA" || sda.labels["bus"] != "sata" || sda.labels["rotational"] != "0" {
		t.Errorf("sda labels = %v", sda.labels)
	}
	nv := find(t, m["rk3588_disk_info"], "device", "nvme1n1")
	if nv.labels["model"] != "Lexar SSD NM790 1TB" || nv.labels["bus"] != "nvme" {
		t.Errorf("nvme1n1 labels = %v", nv.labels)
	}
	if emmc := find(t, m["rk3588_disk_info"], "device", "mmcblk2"); emmc.labels["bus"] != "emmc" || emmc.labels["model"] != "A3A564" {
		t.Errorf("mmcblk2 labels = %v", emmc.labels)
	}
	if sz := find(t, m["rk3588_disk_size_bytes"], "device", "nvme0n1").value; sz != 2000409264*512 {
		t.Errorf("nvme0n1 size = %v, want %v", sz, 2000409264*512)
	}
}
