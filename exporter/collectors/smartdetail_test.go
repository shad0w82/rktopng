package collectors

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

var detailNow = time.Unix(1790977665, 0)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/smart/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// mutate decodes a smartctl document, lets the test edit it, and encodes it again.
func mutate(t *testing.T, doc []byte, edit func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// sub walks into nested objects.
func sub(m map[string]any, path ...string) map[string]any {
	for _, p := range path {
		m = m[p].(map[string]any)
	}
	return m
}

// setAttr edits one row of the ATA attribute table.
func setAttr(t *testing.T, m map[string]any, id int, edit func(row map[string]any)) {
	t.Helper()
	for _, r := range sub(m, "ata_smart_attributes")["table"].([]any) {
		row := r.(map[string]any)
		if int(row["id"].(float64)) == id {
			edit(row)
			return
		}
	}
	t.Fatalf("attribute %d not in the fixture", id)
}

func setDevStat(t *testing.T, m map[string]any, name string, value float64) {
	t.Helper()
	for _, p := range sub(m, "ata_device_statistics")["pages"].([]any) {
		for _, e := range p.(map[string]any)["table"].([]any) {
			if e.(map[string]any)["name"] == name {
				e.(map[string]any)["value"] = value
				return
			}
		}
	}
	t.Fatalf("device statistic %q not in the fixture", name)
}

func near(t *testing.T, what string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-6*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %v, want %v", what, deref(got), want)
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func reasonCodes(d SmartDetail) string {
	var c []string
	for _, r := range d.Health.Reasons {
		c = append(c, r.Level+":"+r.Code)
	}
	return strings.Join(c, ",")
}

func attrByID(t *testing.T, d SmartDetail, id int) Attribute {
	t.Helper()
	for _, a := range d.Attributes {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("attribute %d missing from the report", id)
	return Attribute{}
}

// ── real drives ───────────────────────────────────────────────────────────────────────────

func TestDetailSamsungSATA(t *testing.T) {
	d := buildSmartDetail("sda", fixture(t, "samsung-870-qvo.json"), detailExtras{}, detailNow)
	if !d.Available || d.Protocol != "ATA" || d.Smartctl != "7.4" {
		t.Fatalf("basics: %+v", d)
	}
	id := d.Identity
	if id.Model != "Samsung SSD 870 QVO 2TB" || id.Family != "Samsung based SSDs" || id.Firmware != "SVQ02B6Q" || id.Serial != "TESTSERIAL0001" ||
		id.CapacityBytes != 2000398934016 || id.Interface != "SATA 3.3" || id.LinkCurrent != "6.0 Gb/s" || id.LinkMax != "6.0 Gb/s" ||
		id.FormFactor != "2.5 inches" || id.RotationRPM == nil || *id.RotationRPM != 0 || id.Trim == nil || !*id.Trim ||
		id.KnownModel == nil || !*id.KnownModel || !strings.HasPrefix(id.WWN, "0x5002538") || !strings.HasPrefix(id.Standard, "ACS-4") {
		t.Errorf("identity: %+v", id)
	}
	if d.Health.State != lvOK || d.Health.Passed == nil || !*d.Health.Passed || len(d.Health.Reasons) != 0 {
		t.Errorf("a healthy disk must be ok with no reasons: %+v", d.Health)
	}
	v := d.Vitals
	near(t, "temperature", v.Temperature, 34)
	near(t, "lifetime max (SCT)", v.TempLifetimeMax, 84) // a past peak, not counted
	near(t, "operating limit (SCT)", v.TempLimit, 70)
	near(t, "power-on hours", v.PowerOnHours, 20396)
	near(t, "wear", v.WearUsedPct, 0)
	if v.WearSource != "device statistics" {
		t.Errorf("wear must come from the standard device statistics, got %q", v.WearSource)
	}
	near(t, "written", v.WrittenBytes, 8797610313*512)
	near(t, "read", v.ReadBytes, 3858667964*512)
	if v.WrittenSource != "device statistics" {
		t.Errorf("written source = %q", v.WrittenSource)
	}
	near(t, "unsafe shutdowns", v.UnsafeShutdowns, 99)
	if !strings.Contains(v.UnsafeSource, "POR_Recovery_Count") {
		t.Errorf("unsafe source = %q", v.UnsafeSource)
	}
	near(t, "crc", v.CRCErrors, 0)
	near(t, "reallocated", v.Reallocated, 0)
	near(t, "uncorrectable", v.Uncorrectable, 0)
	if v.Pending != nil {
		t.Error("this drive has no pending-sector counter: it must stay absent, not 0")
	}
	if len(d.Attributes) != 14 {
		t.Fatalf("attributes = %d", len(d.Attributes))
	}
	if a := attrByID(t, d, 177); a.Role != roleWear || !a.Critical || a.Level != lvOK || a.Thresh != 0 {
		t.Errorf("177: %+v", a)
	}
	if a := attrByID(t, d, 235); a.Role != roleUnsafeShutdown || a.Level != lvInfo {
		t.Errorf("235: %+v", a)
	}
	if a := attrByID(t, d, 241); a.Role != roleWritten || a.Bytes == nil || *a.Bytes != 8797610313*512 {
		t.Errorf("241 must carry its size in bytes: %+v", a)
	}
	if a := attrByID(t, d, 9); a.Bytes != nil {
		t.Errorf("only host-traffic attributes carry bytes: %+v", a)
	}
	if a := attrByID(t, d, 5); !a.Prefail || !a.Critical || a.Role != roleReallocated || a.Thresh != 10 {
		t.Errorf("5: %+v", a)
	}
	if d.Logs.ErrorEntries == nil || *d.Logs.ErrorEntries != 0 {
		t.Errorf("error log: %+v", d.Logs)
	}
	if st := d.Logs.SelfTest; st == nil || st.Count != 0 || st.Last != nil || st.ShortMinutes == nil || *st.ShortMinutes != 2 || *st.ExtendedMinutes != 160 {
		t.Errorf("self-test: %+v", d.Logs.SelfTest)
	}
	if h := d.TempHist; h == nil || h.IntervalMinutes != 10 || len(h.Samples) != 128 || h.Samples[127] != 34 {
		t.Errorf("temperature history: %+v", h)
	}
}

func TestDetailSamsungSecond(t *testing.T) {
	d := buildSmartDetail("sdb", fixture(t, "samsung-860-qvo.json"), detailExtras{}, detailNow)
	near(t, "wear (standard indicator)", d.Vitals.WearUsedPct, 1)
	near(t, "lifetime max", d.Vitals.TempLifetimeMax, 73)
	near(t, "written", d.Vitals.WrittenBytes, 25179372268*512)
	near(t, "unsafe", d.Vitals.UnsafeShutdowns, 101)
	if d.Health.State != lvOK {
		t.Errorf("a past temperature peak and unsafe shutdowns must not change the state: %+v", d.Health)
	}
}

func TestDetailLexarNVMe(t *testing.T) {
	ex := detailExtras{TempWarn: fptr(89.85), TempCrit: fptr(94.85), LinkCurrent: "PCIe Gen3 ×1 · 8.0 GT/s", LinkMax: "PCIe Gen4 ×4 · 16.0 GT/s"}
	d := buildSmartDetail("nvme0n1", fixture(t, "lexar-nm790.json"), ex, detailNow)
	// smartctl exits with 4 here only because the drive has no self-test log: the data is valid.
	if !d.Available || d.Protocol != "NVMe" || d.NVMe == nil {
		t.Fatalf("basics: %+v", d)
	}
	if d.Identity.Standard != "NVMe 1.4" || d.Identity.LinkCurrent != ex.LinkCurrent || d.Identity.LinkMax != ex.LinkMax || d.Identity.Model != "Lexar SSD NM790 1TB" {
		t.Errorf("identity: %+v", d.Identity)
	}
	n := d.NVMe
	if n.AvailableSpare != 100 || n.SpareThreshold != 10 || n.PercentageUsed != 0 || n.MediaErrors != 0 || n.UnsafeShutdowns != 104 || len(n.Sensors) != 2 {
		t.Errorf("health log: %+v", n)
	}
	if n.Levels["unsafe_shutdowns"] != lvInfo {
		t.Errorf("unsafe shutdowns are informational: %v", n.Levels)
	}
	v := d.Vitals
	near(t, "written", v.WrittenBytes, 12790374*512000)
	near(t, "read", v.ReadBytes, 73233*512000)
	near(t, "unsafe", v.UnsafeShutdowns, 104)
	near(t, "limit", v.TempLimit, 89.85)
	near(t, "critical", v.TempCritical, 94.85)
	near(t, "spare", v.SpareRemaining, 100)
	if d.Health.State != lvOK || len(d.Health.Reasons) != 0 || d.Logs.SelfTest != nil {
		t.Errorf("health/logs: %+v %+v", d.Health, d.Logs)
	}
}

// ── the state rules ───────────────────────────────────────────────────────────────────────

func TestDetailNVMeRules(t *testing.T) {
	doc := fixture(t, "lexar-nm790.json")
	warn := buildSmartDetail("nvme0n1", mutate(t, doc, func(m map[string]any) {
		h := sub(m, "nvme_smart_health_information_log")
		h["media_errors"], h["available_spare"], h["percentage_used"] = 3.0, 45.0, 84.0
	}), detailExtras{}, detailNow)
	if warn.Health.State != lvWarn || reasonCodes(warn) != "warn:spare,warn:wear,warn:media_errors" {
		t.Errorf("warn: %s %s", warn.Health.State, reasonCodes(warn))
	}
	if warn.Vitals.Levels["wear"] != lvWarn || warn.Vitals.Levels["spare"] != lvWarn || warn.Vitals.Levels["media_errors"] != lvWarn {
		t.Errorf("levels: %v", warn.Vitals.Levels)
	}

	fail := buildSmartDetail("nvme0n1", mutate(t, doc, func(m map[string]any) {
		h := sub(m, "nvme_smart_health_information_log")
		h["critical_warning"], h["available_spare"], h["media_errors"] = 1.0, 8.0, 3.0
		sub(m, "smart_status")["passed"] = false
	}), detailExtras{}, detailNow)
	if fail.Health.State != lvFail || reasonCodes(fail) != "fail:self_assessment,fail:critical_warning,fail:spare,warn:media_errors" {
		t.Errorf("fail: %s %s", fail.Health.State, reasonCodes(fail))
	}

	if d := buildSmartDetail("nvme0n1", mutate(t, doc, func(m map[string]any) { sub(m, "nvme_smart_health_information_log")["percentage_used"] = 100.0 }), detailExtras{}, detailNow); d.Health.State != lvFail {
		t.Errorf("endurance used up must fail: %+v", d.Health)
	}
	// Temperature at the drive's own warning limit is a warning; its past excursions are not counted.
	hot := buildSmartDetail("nvme0n1", mutate(t, doc, func(m map[string]any) { sub(m, "temperature")["current"] = 90.0 }), detailExtras{TempWarn: fptr(89.85)}, detailNow)
	if hot.Health.State != lvWarn || reasonCodes(hot) != "warn:temperature" {
		t.Errorf("hot: %s %s", hot.Health.State, reasonCodes(hot))
	}
	quiet := buildSmartDetail("nvme0n1", mutate(t, doc, func(m map[string]any) {
		h := sub(m, "nvme_smart_health_information_log")
		h["warning_temp_time"], h["unsafe_shutdowns"], h["num_err_log_entries"] = 500.0, 900.0, 7.0
	}), detailExtras{}, detailNow)
	if quiet.Health.State != lvOK {
		t.Errorf("warning-temperature minutes, unsafe shutdowns and error-log entries are shown, not counted: %+v", quiet.Health)
	}
}

func TestDetailATARules(t *testing.T) {
	doc := fixture(t, "samsung-870-qvo.json")
	run := func(edit func(m map[string]any)) SmartDetail {
		return buildSmartDetail("sda", mutate(t, doc, edit), detailExtras{}, detailNow)
	}

	d := run(func(m map[string]any) {
		setAttr(t, m, 5, func(r map[string]any) { r["raw"] = map[string]any{"value": 8.0, "string": "8"} })
		setAttr(t, m, 199, func(r map[string]any) { r["raw"] = map[string]any{"value": 12.0, "string": "12"} })
	})
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:reallocated,warn:crc_errors" {
		t.Errorf("counters: %s %s", d.Health.State, reasonCodes(d))
	}
	if !strings.Contains(d.Health.Reasons[0].Text, "8 reallocated") || attrByID(t, d, 5).Level != lvWarn || attrByID(t, d, 199).Level != lvWarn {
		t.Errorf("text/levels: %+v", d.Health.Reasons)
	}

	// smartctl's own verdict decides failure; the table row carries it.
	d = run(func(m map[string]any) {
		setAttr(t, m, 5, func(r map[string]any) { r["value"], r["worst"], r["when_failed"] = 9.0, 9.0, "now" })
		sub(m, "smart_status")["passed"] = false
	})
	if d.Health.State != lvFail || reasonCodes(d) != "fail:self_assessment,fail:attribute" || attrByID(t, d, 5).Level != lvFail {
		t.Errorf("failing: %s %s", d.Health.State, reasonCodes(d))
	}
	d = run(func(m map[string]any) { setAttr(t, m, 5, func(r map[string]any) { r["when_failed"] = "past" }) })
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:attribute" {
		t.Errorf("failed in the past: %s %s", d.Health.State, reasonCodes(d))
	}

	d = run(func(m map[string]any) { setDevStat(t, m, "Percentage Used Endurance Indicator", 85) })
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:wear" || d.Vitals.Levels["wear"] != lvWarn {
		t.Errorf("wear 85: %s %s", d.Health.State, reasonCodes(d))
	}
	if d = run(func(m map[string]any) { setDevStat(t, m, "Percentage Used Endurance Indicator", 100) }); d.Health.State != lvFail {
		t.Errorf("wear 100: %s", d.Health.State)
	}

	d = run(func(m map[string]any) { sub(m, "ata_smart_error_log", "extended")["count"] = 2.0 })
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:error_log" {
		t.Errorf("error log: %s %s", d.Health.State, reasonCodes(d))
	}
	d = run(func(m map[string]any) {
		st := sub(m, "ata_smart_self_test_log", "extended")
		st["count"] = 1.0
		st["table"] = []any{map[string]any{"type": map[string]any{"string": "Extended offline"}, "status": map[string]any{"string": "Completed: read failure", "passed": false}, "lifetime_hours": 20000.0}}
	})
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:self_test" || d.Logs.SelfTest.Last == nil || d.Logs.SelfTest.Last.Passed {
		t.Errorf("failed self-test: %s %s %+v", d.Health.State, reasonCodes(d), d.Logs.SelfTest)
	}
	d = run(func(m map[string]any) { sub(m, "temperature")["current"] = 71.0 })
	if d.Health.State != lvWarn || reasonCodes(d) != "warn:temperature" {
		t.Errorf("over the operating limit: %s %s", d.Health.State, reasonCodes(d))
	}
	// Reasons are listed worst first.
	d = run(func(m map[string]any) {
		setAttr(t, m, 199, func(r map[string]any) { r["raw"] = map[string]any{"value": 3.0, "string": "3"} })
		sub(m, "smart_status")["passed"] = false
	})
	if d.Health.Reasons[0].Level != lvFail {
		t.Errorf("worst first: %v", d.Health.Reasons)
	}
}

// ── drives that cannot be read ────────────────────────────────────────────────────────────

func TestDetailUnavailable(t *testing.T) {
	cases := []struct{ name, doc, reason string }{
		{"standby", `{"smartctl":{"exit_status":2,"messages":[{"string":"Device is in STANDBY mode, exit(2)","severity":"information"}]}}`, "standby"},
		{"permission", `{"smartctl":{"exit_status":2,"messages":[{"string":"Smartctl open device: /dev/sda failed: Permission denied","severity":"error"}]}}`, "permission"},
		{"usb bridge", `{"smartctl":{"exit_status":2,"messages":[{"string":"/dev/sdc: Unknown USB bridge [0x1234:0x5678 (0x100)]","severity":"error"}]}}`, "unsupported"},
		{"other", `{"smartctl":{"exit_status":1,"messages":[{"string":"Smartctl open device: /dev/sdz failed: No such device","severity":"error"}]}}`, "failed"},
		{"garbage", `not json`, "failed"},
	}
	for _, c := range cases {
		d := buildSmartDetail("sdx", []byte(c.doc), detailExtras{}, detailNow)
		if d.Available || d.Reason != c.reason || d.Message == "" || d.Health.State != lvUnknown {
			t.Errorf("%s: %+v", c.name, d)
		}
	}
	// Higher exit-status bits only describe the disk: the data is still valid.
	d := buildSmartDetail("sdx", []byte(`{"smartctl":{"exit_status":64},"device":{"protocol":"ATA"},"model_name":"X","smart_status":{"passed":true}}`), detailExtras{}, detailNow)
	if !d.Available || d.Identity.Model != "X" {
		t.Errorf("exit status 64: %+v", d)
	}
}
