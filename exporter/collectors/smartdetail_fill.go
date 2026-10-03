package collectors

import (
	"fmt"
	"strings"
)

type addReason = func(level, code, text string)

// wearLevel maps the share of rated endurance used to a level.
func wearLevel(usedPct float64) string {
	switch {
	case usedPct >= wearFailPct:
		return lvFail
	case usedPct >= wearWarnPct:
		return lvWarn
	}
	return lvOK
}

func maxPtr(p *float64, v float64) *float64 {
	if p == nil || v > *p {
		return fptr(v)
	}
	return p
}

// fillATA reads an ATA/SATA disk. The standard sources come first (the ATA device
// statistics log and SCT, identical on every brand); the vendor attribute table is
// shown in full, recognised by name for the figures the standard logs lack, and judged
// only by the rules every drive shares: smartctl's own when_failed verdict and the
// error counters that mean the same thing everywhere.
func fillATA(d *SmartDetail, j *scFull, add addReason) {
	v := &d.Vitals
	lbs := float64(j.LogicalBlockSize)
	if lbs <= 0 {
		lbs = 512
	}

	// Standard source: device statistics (name -> value, only entries the drive marks valid).
	ds := map[string]float64{}
	if j.ATADeviceStatistics != nil {
		for _, p := range j.ATADeviceStatistics.Pages {
			for _, e := range p.Table {
				if e.Value != nil && (e.Flags == nil || e.Flags.Valid == nil || *e.Flags.Valid) {
					ds[e.Name] = *e.Value
				}
			}
		}
	}

	// Vendor attributes.
	var realloc, pending, offline, uncorr, crc *float64
	if j.ATAAttributes != nil {
		for _, a := range j.ATAAttributes.Table {
			raw := attrRawInt(a.Raw.String, a.Raw.Value)
			role := attrRole(a.ID, a.Name)
			at := Attribute{ID: a.ID, Name: a.Name, Value: a.Value, Worst: a.Worst, Thresh: a.Thresh, Raw: raw, RawText: a.Raw.String,
				Prefail: a.Flags.Prefailure, WhenFailed: a.WhenFailed, Role: role, Level: lvOK}
			at.Critical = a.Flags.Prefailure || counterRoles[role] || role == roleWear
			switch a.WhenFailed {
			case "now":
				at.Level = lvFail
				add(lvFail, "attribute", fmt.Sprintf("Attribute %d %s is at %d, at or below its threshold (%d).", a.ID, a.Name, a.Value, a.Thresh))
			case "past":
				at.Level = lvWarn
				add(lvWarn, "attribute", fmt.Sprintf("Attribute %d %s fell to its threshold (%d) in the past (worst %d).", a.ID, a.Name, a.Thresh, a.Worst))
			}
			switch role {
			case roleReallocated:
				realloc = maxPtr(realloc, raw)
			case rolePending:
				pending = maxPtr(pending, raw)
			case roleOfflineUnc:
				offline = maxPtr(offline, raw)
			case roleUncorrectable:
				uncorr = maxPtr(uncorr, raw)
			case roleCRC:
				crc = maxPtr(crc, raw)
			case roleWear:
				if a.Value >= 0 && a.Value <= 100 { // a normalized "life left" outside 0..100 is a constant, not a gauge
					used := float64(100 - a.Value)
					if v.WearUsedPct == nil || used > *v.WearUsedPct {
						v.WearUsedPct, v.WearSource = fptr(used), fmt.Sprintf("attribute %d %s", a.ID, a.Name)
					}
					if l := wearLevel(used); at.Level == lvOK {
						at.Level = l
					}
				}
			case roleWritten, roleRead:
				if _, unit, ok := hostIO(strings.ToLower(a.Name)); ok {
					if unit == 0 {
						unit = lbs
					}
					bytes, src := raw*unit, fmt.Sprintf("attribute %d %s", a.ID, a.Name)
					at.Bytes = fptr(bytes)
					if role == roleWritten && (v.WrittenBytes == nil || bytes > *v.WrittenBytes) {
						v.WrittenBytes, v.WrittenSource = fptr(bytes), src
					}
					if role == roleRead && (v.ReadBytes == nil || bytes > *v.ReadBytes) {
						v.ReadBytes, v.ReadSource = fptr(bytes), src
					}
				}
			case roleUnsafeShutdown:
				v.UnsafeShutdowns, v.UnsafeSource = maxPtr(v.UnsafeShutdowns, raw), fmt.Sprintf("attribute %d %s", a.ID, a.Name)
				at.Level = lvInfo
			}
			if counterRoles[role] && raw > 0 && at.Level == lvOK {
				at.Level = lvWarn
			}
			d.Attributes = append(d.Attributes, at)
		}
	}

	// Standard figures win over vendor ones for wear and traffic; for the counters they fill in what the attributes lack.
	if x, ok := ds["Percentage Used Endurance Indicator"]; ok {
		v.WearUsedPct, v.WearSource = fptr(x), "device statistics"
	}
	if x, ok := ds["Logical Sectors Written"]; ok {
		v.WrittenBytes, v.WrittenSource = fptr(x*lbs), "device statistics"
	}
	if x, ok := ds["Logical Sectors Read"]; ok {
		v.ReadBytes, v.ReadSource = fptr(x*lbs), "device statistics"
	}
	for _, c := range []struct {
		dst  **float64
		stat string
	}{
		{&realloc, "Number of Reallocated Logical Sectors"},
		{&pending, "Number of Reallocation Candidate Logical Sectors"},
		{&uncorr, "Number of Reported Uncorrectable Errors"},
		{&crc, "Number of Interface CRC Errors"},
	} {
		if x, ok := ds[c.stat]; ok {
			*c.dst = maxPtr(*c.dst, x)
		}
	}
	v.Reallocated, v.Pending, v.OfflineUncorrectable, v.Uncorrectable, v.CRCErrors = realloc, pending, offline, uncorr, crc
	for _, c := range []struct {
		key  string
		n    *float64
		what string
	}{
		{"reallocated", realloc, "reallocated sectors or blocks"},
		{"pending", pending, "sectors waiting to be reallocated"},
		{"offline_uncorrectable", offline, "sectors the offline scan could not read"},
		{"uncorrectable", uncorr, "uncorrectable errors"},
		{"crc_errors", crc, "interface CRC errors (check the cable or connector)"},
	} {
		if c.n != nil {
			counterLevel(d, add, c.key, *c.n, c.what)
		}
	}
	if v.WearUsedPct != nil {
		l := wearLevel(*v.WearUsedPct)
		v.Levels["wear"] = l
		switch l {
		case lvFail:
			add(lvFail, "wear", fmt.Sprintf("Rated endurance is used up (%.0f%% used).", *v.WearUsedPct))
		case lvWarn:
			add(lvWarn, "wear", fmt.Sprintf("%.0f%% of the rated endurance is used.", *v.WearUsedPct))
		}
	}

	// Logs.
	if e := j.ATAErrorLog; e != nil {
		var n *float64
		switch {
		case e.Extended != nil:
			n = fptr(e.Extended.Count)
		case e.Summary != nil:
			n = fptr(e.Summary.Count)
		}
		if n != nil {
			d.Logs.ErrorEntries = n
			if *n > 0 {
				add(lvWarn, "error_log", fmt.Sprintf("The ATA error log has %s entries.", fmtCount(*n)))
			}
		}
	}
	if j.ATASelfTestLog != nil || (j.ATASmartData != nil && j.ATASmartData.Capabilities != nil && j.ATASmartData.Capabilities.SelfTestsSupported) {
		st := &SelfTest{}
		if j.ATASmartData != nil && j.ATASmartData.SelfTest != nil && j.ATASmartData.SelfTest.PollingMinutes != nil {
			st.ShortMinutes, st.ExtendedMinutes = j.ATASmartData.SelfTest.PollingMinutes.Short, j.ATASmartData.SelfTest.PollingMinutes.Extended
		}
		if l := j.ATASelfTestLog; l != nil {
			lg := l.Extended
			if lg == nil {
				lg = l.Standard
			}
			if lg != nil {
				st.Count = int(lg.Count)
				if len(lg.Table) > 0 { // newest first
					t := lg.Table[0]
					st.Last = &SelfTestItem{Type: t.Type.String, Status: t.Status.String, Passed: t.Status.Passed == nil || *t.Status.Passed, LifetimeHours: t.LifetimeHours}
					if !st.Last.Passed {
						add(lvWarn, "self_test", fmt.Sprintf("The last self-test (%s) did not complete without error: %s.", t.Type.String, t.Status.String))
					}
				}
			}
		}
		d.Logs.SelfTest = st
	}
	if h := j.SCTHistory; h != nil && len(h.Table) > 0 {
		d.TempHist = &TempHist{IntervalMinutes: h.LoggingIntervalMinutes, Samples: h.Table}
	}
}

// NVMe critical-warning bits (NVMe base specification, SMART / Health log page).
var nvmeWarnBits = []string{"available spare below threshold", "temperature out of range", "reliability degraded", "media in read-only mode", "volatile memory backup failed", "persistent memory region unreliable"}

// fillNVMe reads an NVMe drive. The health log is defined by the NVMe specification, so
// every brand reports the same fields.
func fillNVMe(d *SmartDetail, j *scFull, ex detailExtras, add addReason) {
	h, v := j.NVMeHealth, &d.Vitals
	n := &NVMeLog{CriticalWarning: h.CriticalWarning, AvailableSpare: h.AvailableSpare, SpareThreshold: h.SpareThreshold, PercentageUsed: h.PercentageUsed,
		DataUnitsRead: h.DataUnitsRead, DataUnitsWritten: h.DataUnitsWritten, HostReads: h.HostReads, HostWrites: h.HostWrites,
		ControllerBusy: h.ControllerBusy, PowerCycles: h.PowerCycles, PowerOnHours: h.PowerOnHours, UnsafeShutdowns: h.UnsafeShutdowns,
		MediaErrors: h.MediaErrors, ErrorLogEntries: h.ErrLogEntries, WarningTempMin: h.WarningTempTime, CriticalTempMin: h.CriticalCompTime,
		Sensors: h.Sensors, Levels: map[string]string{}}
	d.NVMe = n
	lv := n.Levels

	if h.CriticalWarning != 0 {
		var what []string
		for bit, name := range nvmeWarnBits {
			if h.CriticalWarning&(1<<bit) != 0 {
				what = append(what, name)
			}
		}
		lv["critical_warning"] = lvFail
		add(lvFail, "critical_warning", fmt.Sprintf("Critical warning flags are set (0x%02x): %s.", h.CriticalWarning, strings.Join(what, ", ")))
	}
	switch {
	case h.AvailableSpare <= h.SpareThreshold:
		lv["available_spare"] = lvFail
		add(lvFail, "spare", fmt.Sprintf("Available spare is %.0f%%, at or below the drive’s threshold (%.0f%%).", h.AvailableSpare, h.SpareThreshold))
	case h.AvailableSpare < spareWarnPct:
		lv["available_spare"] = lvWarn
		add(lvWarn, "spare", fmt.Sprintf("Available spare is down to %.0f%%.", h.AvailableSpare))
	}
	if l := wearLevel(h.PercentageUsed); l != lvOK {
		lv["percentage_used"] = l
		if l == lvFail {
			add(lvFail, "wear", fmt.Sprintf("Rated endurance is used up (%.0f%% used).", h.PercentageUsed))
		} else {
			add(lvWarn, "wear", fmt.Sprintf("%.0f%% of the rated endurance is used.", h.PercentageUsed))
		}
	}
	if h.MediaErrors > 0 {
		lv["media_errors"] = lvWarn
		add(lvWarn, "media_errors", fmt.Sprintf("%s media and data-integrity errors (unrecovered).", fmtCount(h.MediaErrors)))
	}
	if h.ErrLogEntries > 0 {
		lv["error_log_entries"] = lvInfo // entries can be harmless (a rejected command), so they are shown, not counted
	}
	if h.UnsafeShutdowns > 0 {
		lv["unsafe_shutdowns"] = lvInfo
	}

	v.WearUsedPct, v.WearSource = fptr(h.PercentageUsed), "NVMe health log"
	v.SpareRemaining, v.SpareThreshold = fptr(h.AvailableSpare), fptr(h.SpareThreshold)
	v.WrittenBytes, v.WrittenSource = fptr(h.DataUnitsWritten*512000), "NVMe health log" // a data unit is 1000 x 512 bytes
	v.ReadBytes, v.ReadSource = fptr(h.DataUnitsRead*512000), "NVMe health log"
	v.UnsafeShutdowns, v.UnsafeSource = fptr(h.UnsafeShutdowns), "NVMe health log"
	v.MediaErrors = fptr(h.MediaErrors)
	v.TempLimit, v.TempCritical = ex.TempWarn, ex.TempCrit
	v.Levels["wear"] = wearLevel(h.PercentageUsed)
	if lv["available_spare"] != "" {
		v.Levels["spare"] = lv["available_spare"]
	} else {
		v.Levels["spare"] = lvOK
	}
	v.Levels["media_errors"] = lvOK
	if h.MediaErrors > 0 {
		v.Levels["media_errors"] = lvWarn
	}
	e := h.ErrLogEntries
	d.Logs.ErrorEntries = &e

	if t := j.NVMeSelfTestLog; t != nil {
		st := &SelfTest{Count: len(t.Table)}
		if len(t.Table) > 0 {
			r := t.Table[0]
			st.Last = &SelfTestItem{Type: r.Code.String, Status: r.Result.String, Passed: r.Result.Value == 0, LifetimeHours: r.PowerOnHours}
			if !st.Last.Passed {
				add(lvWarn, "self_test", fmt.Sprintf("The last self-test (%s) did not complete without error: %s.", r.Code.String, r.Result.String))
			}
		}
		d.Logs.SelfTest = st
	}
}
