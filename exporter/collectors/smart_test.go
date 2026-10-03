package collectors

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Trimmed real-world shapes of `smartctl -j -n standby -i -H -A <dev>`.
const smartSATA = `{"json_format_version":[1,0],"smartctl":{"version":[7,4],"exit_status":0},
 "model_name":"Samsung SSD 870 QVO 2TB","serial_number":"S5RPNF0T123456","firmware_version":"SVQ02B6Q",
 "smart_status":{"passed":true},"power_on_time":{"hours":1234},"temperature":{"current":33}}`

const smartNVMe = `{"smartctl":{"exit_status":0},"model_name":"Lexar SSD NM790 1TB","serial_number":"NM790X",
 "firmware_version":"12345","smart_status":{"passed":true},"power_on_time":{"hours":77},"temperature":{"current":38}}`

// exit_status 4 = "SMART status check returned DISK FAILING": data is still valid.
const smartFailing = `{"smartctl":{"exit_status":8},"model_name":"Old HDD","smart_status":{"passed":false},"temperature":{"current":41}}`

const smartStandby = `{"smartctl":{"exit_status":2,"messages":[{"string":"Device is in STANDBY mode, exit(2)","severity":"information"}]}}`

const smartNoPermission = `{"smartctl":{"exit_status":2,"messages":[{"string":"Smartctl open device: /dev/sda failed: Permission denied","severity":"error"}]}}`

func TestParseSmartctl(t *testing.T) {
	r, err := parseSmartctl([]byte(smartSATA))
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK || r.Model != "Samsung SSD 870 QVO 2TB" || r.Firmware != "SVQ02B6Q" {
		t.Errorf("identity = %+v", r)
	}
	if r.Temp == nil || *r.Temp != 33 || r.Healthy == nil || !*r.Healthy || r.PowerOnHours == nil || *r.PowerOnHours != 1234 {
		t.Errorf("values = %+v", r)
	}

	// Disk problems are reported in the high exit-status bits but the data is still valid.
	r, err = parseSmartctl([]byte(smartFailing))
	if err != nil || r.Healthy == nil || *r.Healthy {
		t.Errorf("failing disk: err=%v result=%+v", err, r)
	}

	// Bits 0-1 mean smartctl could not read the device at all.
	if _, err = parseSmartctl([]byte(smartStandby)); err == nil || !strings.Contains(err.Error(), "STANDBY") {
		t.Errorf("standby error = %v", err)
	}
	if _, err = parseSmartctl([]byte(smartNoPermission)); err == nil || !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("permission error = %v", err)
	}
	if _, err = parseSmartctl([]byte("not json")); err == nil {
		t.Error("garbage must be an error")
	}
}

// fakeSmartctl answers per device node, recording the arguments it was given.
func fakeSmartctl(answers map[string]string, calls *[][]string) smartRunner {
	return func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		*calls = append(*calls, append([]string{bin}, args...))
		dev := filepath.Base(args[len(args)-1])
		out, ok := answers[dev]
		if !ok {
			return nil, errors.New("exec: no such device")
		}
		return []byte(out), nil
	}
}

func TestSMARTCollectorRefreshAndMetrics(t *testing.T) {
	var calls [][]string
	c := newSMARTCollector("smartctl", time.Minute,
		fakeSmartctl(map[string]string{"sda": smartSATA, "nvme0n1": smartNVMe, "sdb": smartNoPermission}, &calls),
		func() []string { return []string{"nvme0n1", "sda", "sdb"} })
	c.refresh(context.Background())

	if len(calls) != 3 {
		t.Fatalf("expected one smartctl call per disk, got %v", calls)
	}
	// Never wake a sleeping HDD, and ask for machine-readable output.
	args := strings.Join(calls[0], " ")
	for _, want := range []string{"-j", "-n standby", "-H", "-A", filepath.Join(DevPath, "nvme0n1")} {
		if !strings.Contains(args, want) {
			t.Errorf("smartctl args %q lack %q", args, want)
		}
	}

	m := gather(t, c)
	if v := find(t, m["rk3588_smart_temperature_celsius"], "device", "sda").value; v != 33 {
		t.Errorf("sda temp = %v", v)
	}
	if v := find(t, m["rk3588_smart_healthy"], "device", "nvme0n1").value; v != 1 {
		t.Errorf("nvme healthy = %v", v)
	}
	if v := find(t, m["rk3588_smart_power_on_hours"], "device", "sda").value; v != 1234 {
		t.Errorf("sda hours = %v", v)
	}
	if s := find(t, m["rk3588_smart_info"], "device", "sda"); s.labels["model"] != "Samsung SSD 870 QVO 2TB" || s.labels["firmware"] != "SVQ02B6Q" {
		t.Errorf("sda info = %v", s.labels)
	}
	// The serial number is only served on demand (/api/smart/<device>), never as a metric.
	for name, ss := range m {
		for _, s := range ss {
			for k, v := range s.labels {
				if k == "serial" || strings.Contains(v, "S5RPNF0T123456") || strings.Contains(v, "NM790X") {
					t.Errorf("%s exports a serial number: %s=%q", name, k, v)
				}
			}
		}
	}
	// sdb was unreadable: flagged down, and no invented values.
	if v := find(t, m["rk3588_smart_up"], "device", "sdb").value; v != 0 {
		t.Errorf("sdb up = %v, want 0", v)
	}
	if has(m["rk3588_smart_temperature_celsius"], "device", "sdb") {
		t.Error("an unreadable disk must not report a temperature")
	}
	if v := find(t, m["rk3588_smart_up"], "device", "sda").value; v != 1 {
		t.Errorf("sda up = %v, want 1", v)
	}
}

// A failed poll (e.g. HDD in standby) keeps the last good values but marks the disk down.
func TestSMARTCollectorKeepsLastGoodValues(t *testing.T) {
	var calls [][]string
	answers := map[string]string{"sda": smartSATA}
	c := newSMARTCollector("smartctl", time.Minute, fakeSmartctl(answers, &calls), func() []string { return []string{"sda"} })
	c.refresh(context.Background())
	answers["sda"] = smartStandby
	c.refresh(context.Background())

	m := gather(t, c)
	if v := find(t, m["rk3588_smart_temperature_celsius"], "device", "sda").value; v != 33 {
		t.Errorf("last good temp lost: %v", v)
	}
	if v := find(t, m["rk3588_smart_up"], "device", "sda").value; v != 0 {
		t.Errorf("up = %v, want 0 after the failed poll", v)
	}
}

func TestDiskTempMergesHwmonAndSmart(t *testing.T) {
	_, sys := fakeRoots(t)
	write(t, filepath.Join(sys, "block/nvme0n1/size"), "1\n")
	d := filepath.Join(sys, "class/hwmon/hwmon10")
	write(t, filepath.Join(d, "name"), "nvme\n")
	write(t, filepath.Join(d, "temp1_input"), "37850\n")
	write(t, filepath.Join(d, "temp1_label"), "Composite\n")
	link(t, "../../../devices/pcie/nvme/nvme0", filepath.Join(d, "device"))

	var calls [][]string
	smart := newSMARTCollector("smartctl", time.Minute,
		fakeSmartctl(map[string]string{"sda": smartSATA, "nvme0n1": smartNVMe}, &calls),
		func() []string { return []string{"nvme0n1", "sda"} })
	smart.refresh(context.Background())

	m := gather(t, NewDiskTempCollector(smart))
	temps := m["rk3588_disk_temp_celsius"]
	if len(temps) != 2 {
		t.Fatalf("expected exactly one temperature per disk, got %v", temps)
	}
	nv := find(t, temps, "device", "nvme0n1")
	if nv.labels["source"] != "hwmon" || nv.value != 37.85 {
		t.Errorf("nvme must come from hwmon (live), not SMART's 38: %+v", nv)
	}
	if sda := find(t, temps, "device", "sda"); sda.labels["source"] != "smart" || sda.value != 33 {
		t.Errorf("sda = %+v", sda)
	}
}

func TestDiskTempWithoutSMART(t *testing.T) {
	fakeRoots(t)
	m := gather(t, NewDiskTempCollector(nil))
	if len(m) != 0 {
		t.Errorf("no sensors and no SMART must emit nothing, got %v", m)
	}
}

func TestSmartDisksSkipsEMMC(t *testing.T) {
	_, sys := fakeRoots(t)
	buildBoardSysfs(t, sys)
	got := strings.Join(smartDisks(), ",")
	if got != "nvme0n1,nvme1n1,nvme2n1,sda,sdb" {
		t.Errorf("smartDisks = %s (eMMC has no SMART)", got)
	}
}
