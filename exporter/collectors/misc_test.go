package collectors

import (
	"path/filepath"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	ubuntu := "PRETTY_NAME=\"Ubuntu 24.04.4 LTS\"\nNAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\nID=ubuntu\n"
	if got := parseOSRelease(ubuntu); got != "Ubuntu 24.04.4 LTS" {
		t.Errorf("ubuntu = %q", got)
	}
	// No PRETTY_NAME: fall back to NAME + VERSION. Single quotes and comments are handled.
	if got := parseOSRelease("# comment\nNAME='Alpine Linux'\nVERSION=3.20\n"); got != "Alpine Linux 3.20" {
		t.Errorf("fallback = %q", got)
	}
	if got := parseOSRelease(""); got != "" {
		t.Errorf("empty = %q", got)
	}
}

func TestZFSCollector(t *testing.T) {
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "spl/kstat/zfs/NVME_Pool/state"), "ONLINE\n")
	write(t, filepath.Join(proc, "spl/kstat/zfs/SSD_Pool/state"), "DEGRADED\n")
	write(t, filepath.Join(proc, "spl/kstat/zfs/arcstats"), "not a pool\n") // a file, not a pool dir

	m := gather(t, NewZFSCollector())
	if got := find(t, m["rk3588_zfs_pool_online"], "pool", "NVME_Pool").value; got != 1 {
		t.Errorf("NVME_Pool online = %v, want 1", got)
	}
	if got := find(t, m["rk3588_zfs_pool_online"], "pool", "SSD_Pool").value; got != 0 {
		t.Errorf("DEGRADED pool online = %v, want 0", got)
	}
	if s := find(t, m["rk3588_zfs_pool_state"], "pool", "SSD_Pool"); s.labels["state"] != "DEGRADED" {
		t.Errorf("state label = %v", s.labels)
	}
	if len(m["rk3588_zfs_pool_online"]) != 2 {
		t.Errorf("expected 2 pools, got %v", m["rk3588_zfs_pool_online"])
	}
}

func TestZFSCollectorWithoutZFS(t *testing.T) {
	fakeRoots(t)
	if m := gather(t, NewZFSCollector()); len(m) != 0 {
		t.Errorf("a system without ZFS must emit nothing, got %v", m)
	}
}

func TestBracketed(t *testing.T) {
	for in, want := range map[string]string{
		"source [sink]": "sink",
		"[C] PD PD_PPS": "C",
		"host [device]": "device",
		"C PD [PD_PPS]": "PD_PPS",
		"default":       "default",
		"  3.0 \n":      "3.0",
		"":              "",
	} {
		if got := bracketed(in); got != want {
			t.Errorf("bracketed(%q) = %q, want %q", in, got, want)
		}
	}
}

// boardTypeC recreates what the CM3588 exposes with nothing plugged into the USB-C port.
func boardTypeC(t *testing.T, sys string) {
	psy := filepath.Join(sys, "class/power_supply/tcpm-source-psy-6-0022")
	write(t, filepath.Join(psy, "online"), "0\n")
	write(t, filepath.Join(psy, "usb_type"), "[C] PD PD_PPS\n")
	port := filepath.Join(sys, "class/typec/port0")
	write(t, filepath.Join(port, "power_role"), "source [sink]\n")
	write(t, filepath.Join(port, "data_role"), "host [device]\n")
	write(t, filepath.Join(port, "power_operation_mode"), "default\n")
	write(t, filepath.Join(port, "orientation"), "unknown\n")
	write(t, filepath.Join(port, "usb_power_delivery_revision"), "3.0\n")
}

func TestTypeCCollectorNothingPlugged(t *testing.T) {
	_, sys := fakeRoots(t)
	boardTypeC(t, sys)
	m := gather(t, NewTypeCCollector())

	if got := m["rk3588_usb_pd_online"]; len(got) != 1 || got[0].value != 0 {
		t.Errorf("pd_online = %v, want a single 0", got)
	}
	if s := find(t, m["rk3588_usb_pd_type"], "type", "C"); s.value != 1 {
		t.Errorf("pd_type = %v", s)
	}
	p := find(t, m["rk3588_typec_port_info"], "port", "port0")
	want := map[string]string{"power_role": "sink", "data_role": "device", "power_operation_mode": "default",
		"orientation": "unknown", "pd_revision": "3.0", "partner": "no"}
	for k, v := range want {
		if p.labels[k] != v {
			t.Errorf("port label %s = %q, want %q (all: %v)", k, p.labels[k], v, p.labels)
		}
	}
}

func TestTypeCCollectorPartnerAndContract(t *testing.T) {
	_, sys := fakeRoots(t)
	boardTypeC(t, sys)
	// A PD charger is negotiated: partner directory appears, supply goes online with PD active.
	write(t, filepath.Join(sys, "class/typec/port0-partner/usb_power_delivery_revision"), "3.0\n")
	write(t, filepath.Join(sys, "class/power_supply/tcpm-source-psy-6-0022/online"), "1\n")
	write(t, filepath.Join(sys, "class/power_supply/tcpm-source-psy-6-0022/usb_type"), "C [PD] PD_PPS\n")
	write(t, filepath.Join(sys, "class/typec/port0/power_operation_mode"), "usb_power_delivery\n")

	m := gather(t, NewTypeCCollector())
	if got := m["rk3588_usb_pd_online"][0].value; got != 1 {
		t.Errorf("pd_online = %v, want 1", got)
	}
	if !has(m["rk3588_usb_pd_type"], "type", "PD") {
		t.Errorf("active type should be PD, got %v", m["rk3588_usb_pd_type"])
	}
	p := find(t, m["rk3588_typec_port_info"], "port", "port0")
	if p.labels["partner"] != "yes" || p.labels["power_operation_mode"] != "usb_power_delivery" {
		t.Errorf("port labels = %v", p.labels)
	}
	// port0-partner must not be mistaken for a port of its own.
	if n := len(m["rk3588_typec_port_info"]); n != 1 {
		t.Errorf("expected 1 port, got %d", n)
	}
}

// TestNVMeSensorsIdentifiedByController is the regression test for the fragile
// "nvme"+counter naming: hwmon indices are unrelated to the drive number, so the
// drive must come from the hwmon "device" symlink.
func TestNVMeSensorsIdentifiedByController(t *testing.T) {
	_, sys := fakeRoots(t)
	for _, n := range []string{"nvme0n1", "nvme1n1", "nvme2n1"} {
		write(t, filepath.Join(sys, "block", n, "size"), "1\n")
	}
	// hwmon3 belongs to nvme2, hwmon7 to nvme0, hwmon9 to nvme1 — enumeration order would say nvme0,nvme1,nvme2.
	for hw, ctrl := range map[string]string{"hwmon3": "nvme2", "hwmon7": "nvme0", "hwmon9": "nvme1"} {
		d := filepath.Join(sys, "class/hwmon", hw)
		write(t, filepath.Join(d, "name"), "nvme\n")
		write(t, filepath.Join(d, "temp1_input"), map[string]string{"nvme2": "30850", "nvme0": "37850", "nvme1": "34850"}[ctrl]+"\n")
		write(t, filepath.Join(d, "temp1_label"), "Composite\n")
		write(t, filepath.Join(d, "temp2_input"), "29000\n")
		write(t, filepath.Join(d, "temp2_label"), "Sensor 1\n")
		link(t, "../../../devices/platform/pcie/nvme/"+ctrl, filepath.Join(d, "device"))
	}
	write(t, filepath.Join(sys, "class/hwmon/hwmon1/name"), "soc_thermal\n") // unrelated hwmon

	got := map[string]float64{}
	for _, s := range nvmeSensors() {
		if s.sensor == "composite" {
			got[s.device] = s.celsius
		}
	}
	want := map[string]float64{"nvme0n1": 37.85, "nvme1n1": 34.85, "nvme2n1": 30.85}
	for dev, c := range want {
		if d := got[dev] - c; d > 1e-9 || d < -1e-9 {
			t.Errorf("%s composite = %v, want %v (all: %v)", dev, got[dev], c, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 drives, got %v", got)
	}
}

func TestNVMeSensorsFallbackWithoutDeviceLink(t *testing.T) {
	_, sys := fakeRoots(t)
	d := filepath.Join(sys, "class/hwmon/hwmon0")
	write(t, filepath.Join(d, "name"), "nvme\n")
	write(t, filepath.Join(d, "temp1_input"), "40000\n")
	s := nvmeSensors()
	if len(s) != 1 || s[0].device != "nvme0" || s[0].celsius != 40 {
		t.Errorf("fallback sensors = %+v", s)
	}
}

func TestSocInfoDriverVersions(t *testing.T) {
	proc, sys := fakeRoots(t)
	write(t, filepath.Join(sys, "kernel/debug/rkrga/driver_version"), "RGA multicore Driver version: v1.3.10\n")
	write(t, filepath.Join(sys, "module/rknpu/version"), "0.9.8\n") // no debugfs: the module's own entry is used
	write(t, filepath.Join(proc, "mpp_service/version"), "c79104d97229 author: Yandong Lin 2025-08-25 video: rockchip: mpp: Fix load info\nsecond line\n")
	write(t, filepath.Join(sys, "module/valhall_kbase/version"), "g29p0-00eac0 (UK version 1.36)\n")

	info := gather(t, NewInfoCollector())["rk3588_soc_info"]
	if len(info) != 1 {
		t.Fatalf("soc_info = %v", info)
	}
	want := map[string]string{
		"npu_driver": "v0.9.8",
		"rga_driver": "v1.3.10",
		"vpu_driver": "c79104d97229 author: Yandong Lin 2025-08-25 video: rockchip: mpp: Fix load info",
		"gpu_driver": "g29p0-00eac0 (UK version 1.36)",
	}
	for k, v := range want {
		if got := info[0].labels[k]; got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestSocInfoDriverVersionsMissingAreEmpty(t *testing.T) {
	fakeRoots(t)
	info := gather(t, NewInfoCollector())["rk3588_soc_info"]
	for _, k := range []string{"npu_driver", "rga_driver", "vpu_driver", "gpu_driver"} {
		if got := info[0].labels[k]; got != "" {
			t.Errorf("%s = %q, want empty when the driver is not there", k, got)
		}
	}
}

func TestSocInfoNPUDebugfsWinsOverModule(t *testing.T) {
	_, sys := fakeRoots(t)
	write(t, filepath.Join(sys, "kernel/debug/rknpu/version"), "RKNPU driver: v0.9.8\n")
	write(t, filepath.Join(sys, "module/rknpu/version"), "0.0.1\n")
	if got := gather(t, NewInfoCollector())["rk3588_soc_info"][0].labels["npu_driver"]; got != "v0.9.8" {
		t.Errorf("npu_driver = %q, want the debugfs value", got)
	}
}
