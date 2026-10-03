package collectors

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Other brands. There is no such drive on the board, so these documents are built by hand from what
// smartctl's drive database says each vendor calls (and counts in) its attributes. They pin the
// behaviour that makes the report brand-independent.

type at struct {
	id    int
	name  string
	value int
	worst int
	thr   int
	raw   string // as smartctl prints it
	flag  string // "P" pre-fail, "now"/"past" when_failed
}

func ataDoc(t *testing.T, model string, rpm, blockSize int, attrs []at, extra map[string]any) []byte {
	t.Helper()
	var rows []any
	for _, a := range attrs {
		raw := attrRawInt(a.raw, 0)
		wf := ""
		if strings.Contains(a.flag, "now") {
			wf = "now"
		} else if strings.Contains(a.flag, "past") {
			wf = "past"
		}
		rows = append(rows, map[string]any{"id": float64(a.id), "name": a.name, "value": float64(a.value), "worst": float64(a.worst), "thresh": float64(a.thr),
			"when_failed": wf, "flags": map[string]any{"prefailure": strings.Contains(a.flag, "P")}, "raw": map[string]any{"value": raw, "string": a.raw}})
	}
	m := map[string]any{
		"smartctl":             map[string]any{"version": []any{7.0, 4.0}, "exit_status": 0.0},
		"device":               map[string]any{"protocol": "ATA"},
		"model_name":           model,
		"serial_number":        "X",
		"in_smartctl_database": true,
		"logical_block_size":   float64(blockSize),
		"rotation_rate":        float64(rpm),
		"smart_status":         map[string]any{"passed": true},
		"temperature":          map[string]any{"current": 34.0},
		"ata_smart_attributes": map[string]any{"table": rows},
	}
	for k, v := range extra {
		m[k] = v
	}
	return mutate(t, mustJSON(t, m), func(map[string]any) {})
}

func mustJSON(t *testing.T, m map[string]any) []byte {
	t.Helper()
	return mutate(t, []byte(`{}`), func(dst map[string]any) {
		for k, v := range m {
			dst[k] = v
		}
	})
}

func build(t *testing.T, doc []byte) SmartDetail {
	t.Helper()
	return buildSmartDetail("sdx", doc, detailExtras{}, detailNow)
}

func TestBrandKingstonA400(t *testing.T) {
	// Phison-based A400: life left in 231, host writes already in GiB, its own names for unsafe shutdowns and CRC.
	d := build(t, ataDoc(t, "KINGSTON SA400S37240G", 0, 512, []at{
		{1, "Raw_Read_Error_Rate", 100, 100, 50, "0", ""}, {5, "Retired_Block_Count", 100, 100, 10, "0", "P"},
		{9, "Power_On_Hours", 100, 100, 0, "3306", ""}, {12, "Power_Cycle_Count", 100, 100, 0, "180", ""},
		{183, "Unknown_Phison_Attr", 100, 100, 0, "7", ""}, // an unknown attribute with a non-zero raw: must not warn
		{192, "Unsafe_Shutdown_Count", 100, 100, 0, "25", ""},
		{194, "Temperature_Celsius", 32, 52, 0, "32 (Min/Max 20/45)", ""},
		{197, "Not_In_Use", 100, 100, 0, "9", ""}, // the vendor marks it unused: never a pending-sector counter
		{199, "SATA_CRC_Error_Count", 100, 100, 0, "0", ""}, {218, "CRC_Error_Count", 100, 100, 50, "3", ""},
		{231, "SSD_Life_Left", 91, 91, 0, "91", "P"}, {241, "Lifetime_Writes_GiB", 100, 100, 0, "4132", ""}, {242, "Lifetime_Reads_GiB", 100, 100, 0, "1200", ""},
	}, nil))
	v := d.Vitals
	near(t, "wear", v.WearUsedPct, 9)
	if !strings.Contains(v.WearSource, "SSD_Life_Left") {
		t.Errorf("wear source = %q", v.WearSource)
	}
	near(t, "written (GiB, not sectors)", v.WrittenBytes, 4132*math.Pow(2, 30))
	near(t, "read", v.ReadBytes, 1200*math.Pow(2, 30))
	near(t, "unsafe", v.UnsafeShutdowns, 25)
	near(t, "crc: the larger of the two counters", v.CRCErrors, 3)
	near(t, "reallocated (retired blocks)", v.Reallocated, 0)
	if v.Pending != nil {
		t.Error("Not_In_Use at ID 197 must not be read as a pending-sector counter")
	}
	if a := attrByID(t, d, 183); a.Role != "" || a.Level != lvOK {
		t.Errorf("unknown attribute: %+v", a)
	}
	if a := attrByID(t, d, 194); a.Role != roleTemperature || a.Raw != 32 || a.RawText != "32 (Min/Max 20/45)" {
		t.Errorf("temperature attribute: %+v", a)
	}
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:crc_errors" {
		t.Errorf("state: %s %s", d.Health.State, reasonCodes(d))
	}
}

func TestBrandSeagateHDD(t *testing.T) {
	// A hard disk: huge, meaningless raw numbers on the error-rate attributes must never raise a warning.
	d := build(t, ataDoc(t, "ST4000VN006-3CW104", 5980, 4096, []at{
		{1, "Raw_Read_Error_Rate", 112, 99, 6, "47183288", ""}, {3, "Spin_Up_Time", 94, 93, 0, "0", ""},
		{5, "Reallocated_Sector_Ct", 100, 100, 10, "0", "P"}, {7, "Seek_Error_Rate", 82, 60, 45, "4552383168", ""},
		{9, "Power_On_Hours", 78, 78, 0, "19102 (211 18 0)", ""}, {10, "Spin_Retry_Count", 100, 100, 97, "0", ""},
		{184, "End-to-End_Error", 100, 100, 99, "0", ""}, {187, "Reported_Uncorrect", 100, 100, 0, "0", ""}, {188, "Command_Timeout", 100, 100, 0, "0", ""},
		{190, "Airflow_Temperature_Cel", 66, 55, 40, "34 (Min/Max 22/45)", ""},
		{192, "Power-Off_Retract_Count", 100, 100, 0, "212", ""}, {193, "Load_Cycle_Count", 98, 98, 0, "4188", ""},
		{194, "Temperature_Celsius", 34, 45, 0, "34 (0 22 0 0 0)", ""},
		{197, "Current_Pending_Sector", 100, 100, 0, "0", ""}, {198, "Offline_Uncorrectable", 100, 100, 0, "0", ""}, {199, "UDMA_CRC_Error_Count", 200, 200, 0, "0", ""},
	}, nil))
	if d.Health.State != lvOK || len(d.Health.Reasons) != 0 {
		t.Errorf("a healthy hard disk must be ok: %+v", d.Health)
	}
	near(t, "unsafe (emergency head retracts)", d.Vitals.UnsafeShutdowns, 212)
	if d.Vitals.WearUsedPct != nil || d.Vitals.WrittenBytes != nil || d.Vitals.ReadBytes != nil {
		t.Errorf("a disk with no wear or traffic counters must report none, not zero: %+v", d.Vitals)
	}
	if a := attrByID(t, d, 1); a.Role != "" || a.Level != lvOK || a.Raw != 47183288 {
		t.Errorf("raw read error rate: %+v", a)
	}
	if a := attrByID(t, d, 9); a.Raw != 19102 { // the number people read, not the packed bytes
		t.Errorf("power-on hours raw = %v", a.Raw)
	}
	if len(d.Attributes) != 16 || d.Identity.RotationRPM == nil || *d.Identity.RotationRPM != 5980 {
		t.Errorf("table/identity: %d %+v", len(d.Attributes), d.Identity)
	}

	// The same disk with sectors waiting to be reallocated.
	d = build(t, ataDoc(t, "ST4000VN006-3CW104", 5980, 4096, []at{
		{5, "Reallocated_Sector_Ct", 100, 100, 10, "0", "P"}, {197, "Current_Pending_Sector", 100, 100, 0, "8", ""},
	}, nil))
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:pending" || !strings.Contains(d.Health.Reasons[0].Text, "8 sectors waiting") {
		t.Errorf("pending: %s %v", d.Health.State, d.Health.Reasons)
	}
}

func TestBrandOtherWearAndTrafficNames(t *testing.T) {
	// Intel: wear in Media_Wearout_Indicator, host writes in units of 32 MiB. Crucial: Percent_Lifetime_Remain. 4Kn: LBAs are 4096 bytes.
	d := build(t, ataDoc(t, "INTEL SSDSC2BB480G6", 0, 512, []at{
		{233, "Media_Wearout_Indicator", 97, 97, 0, "0", ""}, {241, "Host_Writes_32MiB", 100, 100, 0, "5000", ""}, {242, "Host_Reads_32MiB", 100, 100, 0, "800", ""},
	}, nil))
	near(t, "intel wear", d.Vitals.WearUsedPct, 3)
	near(t, "intel written", d.Vitals.WrittenBytes, 5000*32*math.Pow(2, 20))
	near(t, "intel read", d.Vitals.ReadBytes, 800*32*math.Pow(2, 20))

	d = build(t, ataDoc(t, "CT500MX500SSD1", 0, 512, []at{{202, "Percent_Lifetime_Remain", 88, 88, 1, "12", ""}}, nil))
	near(t, "crucial wear", d.Vitals.WearUsedPct, 12)

	d = build(t, ataDoc(t, "Enterprise SSD", 0, 4096, []at{{241, "Total_LBAs_Written", 100, 100, 0, "1000", ""}}, nil))
	near(t, "4Kn sectors", d.Vitals.WrittenBytes, 1000*4096)

	// A wear gauge that is not a percentage (a constant 200) must not become a bogus figure.
	d = build(t, ataDoc(t, "Odd SSD", 0, 512, []at{{177, "Wear_Leveling_Count", 200, 200, 0, "5", ""}}, nil))
	if d.Vitals.WearUsedPct != nil {
		t.Errorf("a normalized value outside 0..100 is not a gauge: %v", *d.Vitals.WearUsedPct)
	}
	// Heavily worn drive.
	d = build(t, ataDoc(t, "Worn SSD", 0, 512, []at{{233, "Media_Wearout_Indicator", 15, 15, 0, "0", ""}}, nil))
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:wear" {
		t.Errorf("85%% used: %s %s", d.Health.State, reasonCodes(d))
	}
}

func TestBrandUnknownModel(t *testing.T) {
	doc := ataDoc(t, "NoName SSD 128GB", 0, 512, []at{
		{1, "Unknown_Attribute", 100, 100, 0, "0", ""}, {160, "Unknown_Attribute", 100, 100, 0, "55", ""}, {231, "Unknown_Attribute", 100, 100, 0, "9", ""},
	}, map[string]any{"in_smartctl_database": false})
	d := build(t, doc)
	if d.Identity.KnownModel == nil || *d.Identity.KnownModel {
		t.Error("the report must say the model is not in smartctl's database")
	}
	if len(d.Attributes) != 3 || d.Health.State != lvOK {
		t.Errorf("table must still be complete and the state ok: %+v", d.Health)
	}
	for _, a := range d.Attributes {
		if a.Role != "" {
			t.Errorf("unknown attributes get no role: %+v", a)
		}
	}
	if d.Vitals.WearUsedPct != nil || d.Vitals.UnsafeShutdowns != nil {
		t.Errorf("nothing may be guessed: %+v", d.Vitals)
	}
}

func TestBrandStandardCountersFillGaps(t *testing.T) {
	// A hard disk that reports the standard device statistics but no vendor counters for them.
	stats := map[string]any{"ata_device_statistics": map[string]any{"pages": []any{map[string]any{"table": []any{
		map[string]any{"name": "Number of Reallocated Logical Sectors", "value": 4.0, "flags": map[string]any{"valid": true}},
		map[string]any{"name": "Number of Interface CRC Errors", "value": 2.0, "flags": map[string]any{"valid": true}},
		map[string]any{"name": "Number of Reported Uncorrectable Errors", "value": 7.0, "flags": map[string]any{"valid": false}}, // not valid: ignored
	}}}}}
	d := build(t, ataDoc(t, "Some HDD", 7200, 512, []at{{9, "Power_On_Hours", 90, 90, 0, "1000", ""}}, stats))
	near(t, "reallocated", d.Vitals.Reallocated, 4)
	near(t, "crc", d.Vitals.CRCErrors, 2)
	if d.Vitals.Uncorrectable != nil {
		t.Error("an entry the drive marks invalid must be ignored")
	}
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:reallocated,warn:crc_errors" {
		t.Errorf("state: %s %s", d.Health.State, reasonCodes(d))
	}
}

// ── name recognition ─────────────────────────────────────────────────────────────────────

func TestAttrRoles(t *testing.T) {
	cases := []struct {
		id   int
		name string
		want string
	}{
		{5, "Reallocated_Sector_Ct", roleReallocated}, {5, "Retired_Block_Count", roleReallocated}, {5, "Runtime_Bad_Block", roleReallocated},
		{5, "Reallocate_NAND_Blk_Cnt", roleReallocated}, {5, "Not_In_Use", ""}, {183, "Runtime_Bad_Block", ""}, {183, "Unknown_Phison_Attr", ""},
		{187, "Reported_Uncorrect", roleUncorrectable}, {187, "Uncorrectable_Error_Cnt", roleUncorrectable}, {187, "Unknown_Attribute", ""},
		{197, "Current_Pending_Sector", rolePending}, {197, "Not_In_Use", ""}, {198, "Offline_Uncorrectable", roleOfflineUnc},
		{199, "UDMA_CRC_Error_Count", roleCRC}, {218, "CRC_Error_Count", roleCRC}, {199, "Host_Writes_GiB", roleWritten}, {199, "Write_Sectors_Tot_Ct", ""},
		{190, "Airflow_Temperature_Cel", roleTemperature}, {194, "Temperature_Celsius", roleTemperature}, {194, "Temperature_Internal", roleTemperature},
		{190, "Drive_Temp_Warning", ""}, {244, "Temp_Throttle_Status", ""}, {207, "Thermal_Throttling_Cnt", ""},
		{9, "Power_On_Hours", rolePowerOnHours}, {12, "Power_Cycle_Count", rolePowerCycles}, {9, "Power_On_Hours_and_Msec", rolePowerOnHours},
		{177, "Wear_Leveling_Count", roleWear}, {233, "Media_Wearout_Indicator", roleWear}, {231, "SSD_Life_Left", roleWear}, {202, "Percent_Lifetime_Remain", roleWear},
		{248, "Remaining_Life", roleWear}, {245, "Drive_Life_Used", roleWear}, {177, "Wear_Range_Delta", ""}, {230, "Life_Curve_Status", ""}, {232, "Available_Reservd_Space", ""},
		{235, "POR_Recovery_Count", roleUnsafeShutdown}, {192, "Unsafe_Shutdown_Count", roleUnsafeShutdown}, {174, "Unexpect_Power_Loss_Ct", roleUnsafeShutdown},
		{174, "Unexpected_Pwr_Loss_Cnt", roleUnsafeShutdown}, {192, "Power-Off_Retract_Count", roleUnsafeShutdown}, {192, "Unclean_Shutdown_Ct", roleUnsafeShutdown},
		{195, "Power_Fail_Health", ""}, {175, "Power_Loss_Cap_Test", ""},
		{241, "Total_LBAs_Written", roleWritten}, {242, "Total_LBAs_Read", roleRead}, {241, "Lifetime_Writes_GiB", roleWritten}, {242, "Lifetime_Reads_GiB", roleRead},
		{241, "Host_Writes_32MiB", roleWritten}, {241, "Host_Writes_MiB", roleWritten}, {241, "Total_Writes_GB", roleWritten}, {241, "Lifetime_Wts_Frm_Hst_GB", roleWritten},
		{233, "Flash_Writes_GiB", ""}, {233, "Lifetime_Wts_To_Flsh_GB", ""}, {249, "NAND_Writes_1GiB", ""}, {233, "Total_NAND_Writes_GiB", ""}, {241, "Nand_Sectors_Written", ""},
		{241, "Host_Writes", ""}, {241, "Total_LBAs_Written_Low", ""}, {243, "Total_LBAs_Written_High", ""}, {32, "Write_Ampflication", ""},
		{206, "Write_Error_Rate", ""}, {225, "Data_Log_Write_Count", ""}, {1, "Raw_Read_Error_Rate", ""}, {7, "Seek_Error_Rate", ""},
	}
	for _, c := range cases {
		if got := attrRole(c.id, c.name); got != c.want {
			t.Errorf("attrRole(%d, %q) = %q, want %q", c.id, c.name, got, c.want)
		}
	}
}

func TestHostIOUnits(t *testing.T) {
	gib, mib := math.Pow(2, 30), math.Pow(2, 20)
	for name, want := range map[string]float64{
		"total_lbas_written": 0, "host_writes_32mib": 32 * mib, "host_writes_gib": gib, "host_writes_mib": mib, "lifetime_writes_gib": gib,
		"lifetime_wts_frm_hst_gb": 1e9, "total_writes_gb": 1e9, "total_reads_gib": gib, "host_reads_32mib": 32 * mib,
	} {
		if _, unit, ok := hostIO(name); !ok || unit != want {
			t.Errorf("hostIO(%q) = %v, %v; want unit %v", name, unit, ok, want)
		}
	}
}

func TestAttrRawInt(t *testing.T) {
	for _, c := range []struct {
		s    string
		v    float64
		want float64
	}{{"34 (Min/Max 20/45)", 99999, 34}, {"19102 (211 18 0)", 1, 19102}, {"0", 0, 0}, {"8797610313", 8797610313, 8797610313}, {"0x0000ffff", 65535, 65535}, {"", 7, 7}, {"n/a", 5, 5}} {
		if got := attrRawInt(c.s, c.v); got != c.want {
			t.Errorf("attrRawInt(%q, %v) = %v, want %v", c.s, c.v, got, c.want)
		}
	}
}

// TestRolesAgainstDriveDB checks the name rules against every attribute name smartctl's own drive
// database knows (the file is GPL, so it is read from the system, not vendored). Skipped when absent.
func TestRolesAgainstDriveDB(t *testing.T) {
	path := os.Getenv("RKTOP_DRIVEDB")
	if path == "" {
		path = "/usr/share/smartmontools/drivedb.h"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("drive database not available (%v); set RKTOP_DRIVEDB to run this check", err)
	}
	re := regexp.MustCompile(`-v\s+(\d+),[A-Za-z0-9:()/+_-]+,([A-Za-z0-9_/.\-]+)`)
	type pair struct {
		id   int
		name string
	}
	seen := map[pair]bool{}
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		id, _ := strconv.Atoi(m[1])
		seen[pair{id, m[2]}] = true
	}
	if len(seen) < 300 {
		t.Fatalf("parsed only %d definitions from %s", len(seen), path)
	}
	byRole := map[string]int{}
	for p := range seen {
		role := attrRole(p.id, p.name)
		byRole[role]++
		n := strings.ToLower(p.name)
		switch role {
		case roleWritten, roleRead:
			if regexp.MustCompile(`nand|flash|flsh|slc|tlc|amp`).MatchString(n) {
				t.Errorf("%d %s: NAND/flash traffic taken for host traffic", p.id, p.name)
			}
		case roleReallocated, roleUncorrectable, rolePending, roleOfflineUnc:
			if regexp.MustCompile(`not_in_use|unknown`).MatchString(n) {
				t.Errorf("%d %s: an unused attribute taken for a counter", p.id, p.name)
			}
		}
		// Whatever the vendor calls an unsafe shutdown must be recognised.
		if regexp.MustCompile(`unsafe|unexpect|unclean`).MatchString(n) && role != roleUnsafeShutdown {
			t.Errorf("%d %s: looks like an unsafe-shutdown counter but has role %q", p.id, p.name, role)
		}
		if strings.Contains(n, "crc") && role != roleCRC && role != roleWritten && role != roleRead {
			t.Errorf("%d %s: looks like a CRC counter but has role %q", p.id, p.name, role)
		}
		// The IDs used as counters only ever carry counter-like names.
		switch p.id {
		case 5:
			if role == "" && !regexp.MustCompile(`not_in_use|unknown|retried`).MatchString(n) { // "Retried_Blk_Ct" (JMicron) counts read retries: not a reallocation count
				t.Errorf("%d %s: ID 5 name not recognised", p.id, p.name)
			}
		case 187, 197, 198:
			if role == "" {
				t.Logf("note: %d %s has no counter role", p.id, p.name)
			}
		}
	}
	t.Logf("%d (id,name) pairs in the drive database; roles found: %v", len(seen), byRole)
	for _, role := range []string{roleWear, roleWritten, roleRead, roleUnsafeShutdown, roleCRC, roleTemperature, roleReallocated} {
		if byRole[role] == 0 {
			t.Errorf("no attribute of the drive database has role %s: the rules match nothing", role)
		}
	}
}

// ── the on-demand reader ─────────────────────────────────────────────────────────────────

func fakeReader(t *testing.T, answers map[string]string, calls *int32) *SMARTDetailReader {
	t.Helper()
	run := func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		atomic.AddInt32(calls, 1)
		out, ok := answers[filepath.Base(args[len(args)-1])]
		if !ok {
			return nil, errors.New("exec: no such device")
		}
		return []byte(out), nil
	}
	return newSMARTDetailReader("smartctl", true, run, func() []string { return []string{"nvme0n1", "sda"} },
		func(dev string) []string { return []string{"-j", "-x", "/dev/" + dev} }, func(string) detailExtras { return detailExtras{} })
}

func TestReaderCachesAndValidates(t *testing.T) {
	var calls int32
	doc := string(mustRead(t, "testdata/smart/samsung-870-qvo.json"))
	r := fakeReader(t, map[string]string{"sda": doc}, &calls)
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }

	if _, err := r.Get(context.Background(), "sdz"); !errors.Is(err, ErrUnknownDisk) {
		t.Errorf("unknown disk: %v", err)
	}
	if _, err := r.Get(context.Background(), "../../etc/passwd"); !errors.Is(err, ErrUnknownDisk) {
		t.Errorf("a path is not a disk: %v", err)
	}
	if calls != 0 {
		t.Errorf("an unknown disk must never reach smartctl (%d calls)", calls)
	}

	d, err := r.Get(context.Background(), "sda")
	if err != nil || !d.Available || d.Identity.Model != "Samsung SSD 870 QVO 2TB" {
		t.Fatalf("first read: %v %+v", err, d)
	}
	if _, err = r.Get(context.Background(), "sda"); err != nil || calls != 1 {
		t.Errorf("a second read inside the cache window must reuse the report: calls=%d err=%v", calls, err)
	}
	clock = clock.Add(smartDetailTTL + time.Second)
	if _, err = r.Get(context.Background(), "sda"); err != nil || calls != 2 {
		t.Errorf("after the window it must read again: calls=%d", calls)
	}
}

func TestReaderUnreadableDiskIsRetriedSooner(t *testing.T) {
	var calls int32
	r := fakeReader(t, map[string]string{"sda": `{"smartctl":{"exit_status":2,"messages":[{"string":"Device is in STANDBY mode","severity":"information"}]}}`}, &calls)
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }
	d, _ := r.Get(context.Background(), "sda")
	if d.Available || d.Reason != "standby" {
		t.Fatalf("standby: %+v", d)
	}
	clock = clock.Add(smartDetailFailTTL + time.Second)
	_, _ = r.Get(context.Background(), "sda")
	if calls != 2 {
		t.Errorf("an unreadable disk must be retried after %v, calls=%d", smartDetailFailTTL, calls)
	}
}

func TestReaderSharesOneRunBetweenRequests(t *testing.T) {
	var calls int32
	release := make(chan struct{})
	run := func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return mustRead(t, "testdata/smart/samsung-870-qvo.json"), nil
	}
	r := newSMARTDetailReader("smartctl", true, run, func() []string { return []string{"sda"} },
		func(dev string) []string { return []string{dev} }, func(string) detailExtras { return detailExtras{} })
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d, err := r.Get(context.Background(), "sda"); err != nil || !d.Available {
				t.Errorf("get: %v %+v", err, d)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls != 1 {
		t.Errorf("five simultaneous windows must share one smartctl run, got %d", calls)
	}
}

func TestReaderGivesUpWaitingButKeepsReading(t *testing.T) {
	release := make(chan struct{})
	run := func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		<-release
		return mustRead(t, "testdata/smart/samsung-870-qvo.json"), nil
	}
	r := newSMARTDetailReader("smartctl", true, run, func() []string { return []string{"sda"} },
		func(dev string) []string { return []string{dev} }, func(string) detailExtras { return detailExtras{} })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := r.Get(ctx, "sda"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a client that stops waiting gets its context error: %v", err)
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if d, err := r.Get(context.Background(), "sda"); err == nil && d.Available {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the read must still complete and be cached after a client leaves")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReaderFailureModes(t *testing.T) {
	var calls int32
	r := fakeReader(t, map[string]string{}, &calls)
	if d, _ := r.Get(context.Background(), "sda"); d.Available || d.Reason != "failed" || !strings.Contains(d.Message, "no such device") || d.Health.State != lvUnknown {
		t.Errorf("runner error: %+v", d)
	}
	r.lookup = func(string) error { return errors.New("not found") }
	r.cache = map[string]smartCached{}
	if d, _ := r.Get(context.Background(), "sda"); d.Reason != "smartctl_missing" {
		t.Errorf("missing smartctl: %+v", d)
	}
	r.enabled = false
	if d, _ := r.Get(context.Background(), "sda"); d.Reason != "disabled" {
		t.Errorf("disabled: %+v", d)
	}
}

func TestDetailArgs(t *testing.T) {
	if got := strings.Join(detailArgs("nvme0n1"), " "); got != "-j -a "+filepath.Join(DevPath, "nvme0n1") {
		t.Errorf("nvme args: %s", got)
	}
	if got := strings.Join(detailArgs("sda"), " "); got != "-j -x -n standby "+filepath.Join(DevPath, "sda") {
		t.Errorf("sata args: %s", got)
	}
}

func TestReadDetailExtras(t *testing.T) {
	_, sys := fakeRoots(t)
	base := filepath.Join(sys, "class/nvme/nvme0")
	write(t, base+"/hwmon10/temp1_max", "89850\n")
	write(t, base+"/hwmon10/temp1_crit", "94850\n")
	write(t, base+"/device/current_link_speed", "8.0 GT/s PCIe\n")
	write(t, base+"/device/current_link_width", "1\n")
	write(t, base+"/device/max_link_speed", "16.0 GT/s PCIe\n")
	write(t, base+"/device/max_link_width", "4\n")
	ex := readDetailExtras("nvme0n1")
	near(t, "warn limit", ex.TempWarn, 89.85)
	near(t, "critical limit", ex.TempCrit, 94.85)
	if ex.LinkCurrent != "PCIe Gen3 ×1 · 8.0 GT/s" || ex.LinkMax != "PCIe Gen4 ×4 · 16.0 GT/s" {
		t.Errorf("link: %q / %q", ex.LinkCurrent, ex.LinkMax)
	}
	if ex := readDetailExtras("sda"); ex.TempWarn != nil || ex.LinkCurrent != "" {
		t.Errorf("a SATA disk has no NVMe extras: %+v", ex)
	}
	if ex := readDetailExtras("nvme9n1"); ex.TempWarn != nil || ex.LinkCurrent != "" {
		t.Errorf("a missing controller must give nothing: %+v", ex)
	}
	for in, want := range map[[2]string]string{{"2.5 GT/s PCIe", "1"}: "PCIe Gen1 ×1 · 2.5 GT/s", {"32.0 GT/s PCIe", "8"}: "PCIe Gen5 ×8 · 32.0 GT/s", {"", "1"}: "", {"Unknown", "1"}: "", {"12.0 GT/s PCIe", "2"}: "PCIe 12.0 GT/s ×2"} {
		if got := pcieLink(in[0], in[1]); got != want {
			t.Errorf("pcieLink(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
