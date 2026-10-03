package collectors

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Levels of a single figure and of the whole disk ("ok" | "warn" | "fail"; "info" marks
// a figure that is shown but never counted, such as unsafe shutdowns).
const (
	lvOK   = "ok"
	lvInfo = "info"
	lvWarn = "warn"
	lvFail = "fail"

	lvUnknown = "unknown" // health of a disk that could not be read
)

// Thresholds of the health state (documented in docs/smart_status.md).
const (
	wearWarnPct  = 80 // % of the rated endurance used
	wearFailPct  = 100
	spareWarnPct = 50 // NVMe available spare (%), below it is a warning; at or below the drive's own threshold it fails
)

// unavailableDetail is the report of a disk that could not be read: no health verdict, just why.
func unavailableDetail(dev string, now time.Time, reason, msg string) SmartDetail {
	return SmartDetail{Device: dev, ReadAt: now.UnixMilli(), Reason: reason, Message: msg, Health: Health{State: lvUnknown}, Vitals: Vitals{Levels: map[string]string{}}}
}

// SmartDetail is the on-demand S.M.A.R.T. report of one disk, served at /api/smart/<dev>.
// It is the same shape for every brand and bus: whatever a drive does not expose is
// simply absent (null / omitted), never guessed.
type SmartDetail struct {
	Device    string `json:"device"`
	Available bool   `json:"available"`        // false: nothing could be read, see Reason
	Reason    string `json:"reason,omitempty"` // standby | permission | unsupported | disabled | failed
	Message   string `json:"message,omitempty"`
	ReadAt    int64  `json:"read_at"` // unix ms
	Smartctl  string `json:"smartctl,omitempty"`

	Protocol   string      `json:"protocol,omitempty"` // ATA | NVMe | SCSI
	Identity   Identity    `json:"identity"`
	Health     Health      `json:"health"`
	Vitals     Vitals      `json:"vitals"`
	Attributes []Attribute `json:"attributes,omitempty"` // ATA attribute table
	NVMe       *NVMeLog    `json:"nvme,omitempty"`       // NVMe health log
	Logs       Logs        `json:"logs"`
	TempHist   *TempHist   `json:"temp_history,omitempty"` // the drive's own SCT temperature log (ATA)
}

type Identity struct {
	Model         string `json:"model,omitempty"`
	Family        string `json:"family,omitempty"`
	Firmware      string `json:"firmware,omitempty"`
	Serial        string `json:"serial,omitempty"`
	WWN           string `json:"wwn,omitempty"`
	CapacityBytes int64  `json:"capacity_bytes,omitempty"`
	BlockSize     int    `json:"block_size,omitempty"`
	RotationRPM   *int   `json:"rotation_rpm,omitempty"` // 0 = solid state
	FormFactor    string `json:"form_factor,omitempty"`
	Standard      string `json:"standard,omitempty"` // "NVMe 1.4" / "ACS-4 ..."
	Interface     string `json:"interface,omitempty"`
	LinkCurrent   string `json:"link_current,omitempty"`
	LinkMax       string `json:"link_max,omitempty"`
	Trim          *bool  `json:"trim,omitempty"`
	KnownModel    *bool  `json:"known_model,omitempty"` // ATA: the model is in smartctl's drive database (attribute names are reliable)
}

type Reason struct {
	Level string `json:"level"` // warn | fail
	Code  string `json:"code"`
	Text  string `json:"text"`
}

type Health struct {
	State   string   `json:"state"`            // ok | warn | fail | unknown (not readable)
	Passed  *bool    `json:"passed,omitempty"` // the drive's own overall self-assessment
	Reasons []Reason `json:"reasons,omitempty"`
}

// Vitals are the headline figures. Each one comes from the most standard source the
// drive offers (device statistics, SCT, the NVMe health log) and falls back to a
// recognised vendor attribute; the *Source fields say which.
type Vitals struct {
	Temperature          *float64          `json:"temperature_c,omitempty"`
	TempLifetimeMax      *float64          `json:"temp_lifetime_max_c,omitempty"`
	TempLifetimeMin      *float64          `json:"temp_lifetime_min_c,omitempty"`
	TempLimit            *float64          `json:"temp_limit_c,omitempty"`    // operating limit (ATA SCT) / warning limit (NVMe)
	TempCritical         *float64          `json:"temp_critical_c,omitempty"` // NVMe critical limit
	PowerOnHours         *float64          `json:"power_on_hours,omitempty"`
	PowerCycles          *float64          `json:"power_cycles,omitempty"`
	WearUsedPct          *float64          `json:"wear_used_pct,omitempty"`
	WearSource           string            `json:"wear_source,omitempty"`
	SpareRemaining       *float64          `json:"spare_remaining_pct,omitempty"` // NVMe
	SpareThreshold       *float64          `json:"spare_threshold_pct,omitempty"`
	WrittenBytes         *float64          `json:"written_bytes,omitempty"`
	WrittenSource        string            `json:"written_source,omitempty"`
	ReadBytes            *float64          `json:"read_bytes,omitempty"`
	ReadSource           string            `json:"read_source,omitempty"`
	UnsafeShutdowns      *float64          `json:"unsafe_shutdowns,omitempty"`
	UnsafeSource         string            `json:"unsafe_source,omitempty"`
	Reallocated          *float64          `json:"reallocated,omitempty"`
	Pending              *float64          `json:"pending,omitempty"`
	OfflineUncorrectable *float64          `json:"offline_uncorrectable,omitempty"`
	Uncorrectable        *float64          `json:"uncorrectable,omitempty"`
	CRCErrors            *float64          `json:"crc_errors,omitempty"`
	MediaErrors          *float64          `json:"media_errors,omitempty"` // NVMe
	Levels               map[string]string `json:"levels"`                 // wear, spare, temperature, media_errors, reallocated, pending, uncorrectable, crc_errors
}

type Attribute struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Value      int      `json:"value"` // normalized
	Worst      int      `json:"worst"`
	Thresh     int      `json:"thresh"` // 0 = none
	Raw        float64  `json:"raw"`
	RawText    string   `json:"raw_text"`
	Bytes      *float64 `json:"bytes,omitempty"` // host traffic attributes: the raw value converted to bytes
	Prefail    bool     `json:"prefail"`
	WhenFailed string   `json:"when_failed,omitempty"` // now | past (from smartctl)
	Role       string   `json:"role,omitempty"`
	Critical   bool     `json:"critical"`
	Level      string   `json:"level"`
}

type NVMeLog struct {
	CriticalWarning  int               `json:"critical_warning"`
	AvailableSpare   float64           `json:"available_spare"`
	SpareThreshold   float64           `json:"available_spare_threshold"`
	PercentageUsed   float64           `json:"percentage_used"`
	DataUnitsRead    float64           `json:"data_units_read"`
	DataUnitsWritten float64           `json:"data_units_written"`
	HostReads        float64           `json:"host_reads"`
	HostWrites       float64           `json:"host_writes"`
	ControllerBusy   float64           `json:"controller_busy_minutes"`
	PowerCycles      float64           `json:"power_cycles"`
	PowerOnHours     float64           `json:"power_on_hours"`
	UnsafeShutdowns  float64           `json:"unsafe_shutdowns"`
	MediaErrors      float64           `json:"media_errors"`
	ErrorLogEntries  float64           `json:"error_log_entries"`
	WarningTempMin   float64           `json:"warning_temp_minutes"`
	CriticalTempMin  float64           `json:"critical_temp_minutes"`
	Sensors          []float64         `json:"sensors,omitempty"`
	Levels           map[string]string `json:"levels"` // per field: critical_warning, available_spare, percentage_used, media_errors, error_log_entries, temperature, unsafe_shutdowns
}

type Logs struct {
	ErrorEntries *float64  `json:"error_entries,omitempty"` // ATA error log / NVMe error log entries
	SelfTest     *SelfTest `json:"self_test,omitempty"`     // nil: the drive has no self-test log
}

type SelfTest struct {
	Count           int           `json:"count"`
	ShortMinutes    *int          `json:"short_minutes,omitempty"`
	ExtendedMinutes *int          `json:"extended_minutes,omitempty"`
	Last            *SelfTestItem `json:"last,omitempty"`
}

type SelfTestItem struct {
	Type          string `json:"type"`
	Status        string `json:"status"`
	Passed        bool   `json:"passed"`
	LifetimeHours *int   `json:"lifetime_hours,omitempty"`
}

// TempHist is the drive's own temperature log (ATA SCT), oldest sample first.
type TempHist struct {
	IntervalMinutes int   `json:"interval_minutes"`
	Samples         []int `json:"samples"`
}

// ── smartctl -j input (only what is used; unknown fields are ignored) ─────────────────────

type scInt = *int

type scFull struct {
	Smartctl struct {
		Version    []int `json:"version"`
		ExitStatus int   `json:"exit_status"`
		Messages   []struct {
			String   string `json:"string"`
			Severity string `json:"severity"`
		} `json:"messages"`
	} `json:"smartctl"`
	Device struct {
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelFamily        string `json:"model_family"`
	ModelName          string `json:"model_name"`
	SCSIVendor         string `json:"scsi_vendor"`
	SCSIProduct        string `json:"scsi_product"`
	SerialNumber       string `json:"serial_number"`
	FirmwareVersion    string `json:"firmware_version"`
	InSmartctlDatabase *bool  `json:"in_smartctl_database"`
	WWN                *struct {
		NAA int64 `json:"naa"`
		OUI int64 `json:"oui"`
		ID  int64 `json:"id"`
	} `json:"wwn"`
	UserCapacity *struct {
		Bytes int64 `json:"bytes"`
	} `json:"user_capacity"`
	LogicalBlockSize int   `json:"logical_block_size"`
	RotationRate     scInt `json:"rotation_rate"`
	FormFactor       *struct {
		Name string `json:"name"`
	} `json:"form_factor"`
	Trim *struct {
		Supported bool `json:"supported"`
	} `json:"trim"`
	ATAVersion *struct {
		String string `json:"string"`
	} `json:"ata_version"`
	SATAVersion *struct {
		String string `json:"string"`
	} `json:"sata_version"`
	InterfaceSpeed *struct {
		Max *struct {
			String string `json:"string"`
		} `json:"max"`
		Current *struct {
			String string `json:"string"`
		} `json:"current"`
	} `json:"interface_speed"`
	SmartStatus *struct {
		Passed *bool `json:"passed"`
	} `json:"smart_status"`
	Temperature *struct {
		Current     *float64 `json:"current"`
		LifetimeMin *float64 `json:"lifetime_min"`
		LifetimeMax *float64 `json:"lifetime_max"`
		OpLimitMax  *float64 `json:"op_limit_max"`
	} `json:"temperature"`
	PowerOnTime *struct {
		Hours *float64 `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount *float64 `json:"power_cycle_count"`

	ATAAttributes *struct {
		Table []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Value      int    `json:"value"`
			Worst      int    `json:"worst"`
			Thresh     int    `json:"thresh"`
			WhenFailed string `json:"when_failed"`
			Flags      struct {
				Prefailure bool `json:"prefailure"`
			} `json:"flags"`
			Raw struct {
				Value  float64 `json:"value"`
				String string  `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	ATADeviceStatistics *struct {
		Pages []struct {
			Table []struct {
				Name  string   `json:"name"`
				Value *float64 `json:"value"`
				Flags *struct {
					Valid *bool `json:"valid"`
				} `json:"flags"`
			} `json:"table"`
		} `json:"pages"`
	} `json:"ata_device_statistics"`
	ATAErrorLog *struct {
		Summary *struct {
			Count float64 `json:"count"`
		} `json:"summary"`
		Extended *struct {
			Count float64 `json:"count"`
		} `json:"extended"`
	} `json:"ata_smart_error_log"`
	ATASelfTestLog *struct {
		Standard *scSelfTestLog `json:"standard"`
		Extended *scSelfTestLog `json:"extended"`
	} `json:"ata_smart_self_test_log"`
	ATASmartData *struct {
		SelfTest *struct {
			PollingMinutes *struct {
				Short    scInt `json:"short"`
				Extended scInt `json:"extended"`
			} `json:"polling_minutes"`
		} `json:"self_test"`
		Capabilities *struct {
			SelfTestsSupported bool `json:"self_tests_supported"`
		} `json:"capabilities"`
	} `json:"ata_smart_data"`
	SCTHistory *struct {
		LoggingIntervalMinutes int   `json:"logging_interval_minutes"`
		Table                  []int `json:"table"` // already oldest first (checked against smartd's own log)
	} `json:"ata_sct_temperature_history"`

	NVMeVersion *struct {
		String string `json:"string"`
	} `json:"nvme_version"`
	NVMeHealth *struct {
		CriticalWarning  int       `json:"critical_warning"`
		AvailableSpare   float64   `json:"available_spare"`
		SpareThreshold   float64   `json:"available_spare_threshold"`
		PercentageUsed   float64   `json:"percentage_used"`
		DataUnitsRead    float64   `json:"data_units_read"`
		DataUnitsWritten float64   `json:"data_units_written"`
		HostReads        float64   `json:"host_reads"`
		HostWrites       float64   `json:"host_writes"`
		ControllerBusy   float64   `json:"controller_busy_time"`
		PowerCycles      float64   `json:"power_cycles"`
		PowerOnHours     float64   `json:"power_on_hours"`
		UnsafeShutdowns  float64   `json:"unsafe_shutdowns"`
		MediaErrors      float64   `json:"media_errors"`
		ErrLogEntries    float64   `json:"num_err_log_entries"`
		WarningTempTime  float64   `json:"warning_temp_time"`
		CriticalCompTime float64   `json:"critical_comp_time"`
		Sensors          []float64 `json:"temperature_sensors"`
	} `json:"nvme_smart_health_information_log"`
	NVMeSelfTestLog *struct {
		Table []struct {
			Code struct {
				String string `json:"string"`
			} `json:"self_test_code"`
			Result struct {
				Value  int    `json:"value"`
				String string `json:"string"`
			} `json:"self_test_result"`
			PowerOnHours scInt `json:"power_on_hours"`
		} `json:"table"`
	} `json:"nvme_self_test_log"`

	SCSIGrownDefects *float64 `json:"scsi_grown_defect_list"`
}

type scSelfTestLog struct {
	Count float64 `json:"count"`
	Table []struct {
		Type struct {
			String string `json:"string"`
		} `json:"type"`
		Status struct {
			String string `json:"string"`
			Passed *bool  `json:"passed"`
		} `json:"status"`
		LifetimeHours scInt `json:"lifetime_hours"`
	} `json:"table"`
}

// detailExtras are the few facts smartctl does not report but the kernel does (no root needed).
type detailExtras struct {
	TempWarn, TempCrit *float64 // NVMe: the drive's warning/critical composite-temperature limits (hwmon)
	LinkCurrent        string   // NVMe: "PCIe Gen3 ×1 · 8.0 GT/s"
	LinkMax            string
}

func fptr(v float64) *float64 { return &v }

// buildSmartDetail turns one `smartctl -j` document into a SmartDetail. It never fails:
// an unreadable disk becomes Available=false with a reason.
func buildSmartDetail(dev string, raw []byte, ex detailExtras, now time.Time) SmartDetail {
	d := SmartDetail{Device: dev, ReadAt: now.UnixMilli(), Health: Health{State: lvOK}, Vitals: Vitals{Levels: map[string]string{}}}
	var j scFull
	if err := json.Unmarshal(raw, &j); err != nil {
		return unavailableDetail(dev, now, "failed", "smartctl produced unreadable output")
	}
	if v := j.Smartctl.Version; len(v) >= 2 {
		d.Smartctl = fmt.Sprintf("%d.%d", v[0], v[1])
	}
	// Exit-status bits 0-1: smartctl could not run or open the device. The higher bits only
	// describe the disk (some NVMe drives set bit 2 just because they lack a self-test log).
	if j.Smartctl.ExitStatus&3 != 0 {
		reason, msg := unavailableReason(&j)
		u := unavailableDetail(dev, now, reason, msg)
		u.Smartctl = d.Smartctl
		return u
	}
	d.Available = true
	d.Protocol = j.Device.Protocol
	fillIdentity(&d, &j, ex)

	var reasons []Reason
	add := func(level, code, text string) { reasons = append(reasons, Reason{level, code, text}) }
	if j.SmartStatus != nil && j.SmartStatus.Passed != nil {
		d.Health.Passed = j.SmartStatus.Passed
		if !*j.SmartStatus.Passed {
			add(lvFail, "self_assessment", "The drive’s own health self-assessment did not pass.")
		}
	}
	if j.NVMeHealth != nil {
		fillNVMe(&d, &j, ex, add)
	} else {
		fillATA(&d, &j, add)
	}
	fillCommon(&d, &j, add)
	if t := d.Vitals.Temperature; t != nil && d.Vitals.TempLimit != nil && *t >= *d.Vitals.TempLimit {
		d.Vitals.Levels["temperature"] = lvWarn
		add(lvWarn, "temperature", fmt.Sprintf("Temperature %.0f °C is at or above the drive’s limit (%.0f °C).", *t, *d.Vitals.TempLimit))
	}
	if d.NVMe != nil && d.Vitals.Levels["temperature"] != "" {
		d.NVMe.Levels["temperature"] = d.Vitals.Levels["temperature"]
	}
	// Worst first, otherwise in the order found.
	sort.SliceStable(reasons, func(a, b int) bool { return reasons[a].Level == lvFail && reasons[b].Level != lvFail })
	d.Health.Reasons = reasons
	for _, r := range reasons {
		if r.Level == lvFail {
			d.Health.State = lvFail
			break
		}
		d.Health.State = lvWarn
	}
	return d
}

func unavailableReason(j *scFull) (reason, msg string) {
	reason, msg = "failed", "smartctl could not read the disk"
	for _, m := range j.Smartctl.Messages {
		l := strings.ToLower(m.String)
		switch {
		case strings.Contains(l, "standby"):
			return "standby", "The disk is in standby; it was not woken."
		case strings.Contains(l, "permission denied"):
			return "permission", m.String
		case strings.Contains(l, "unknown usb bridge"), strings.Contains(l, "unsupported"), strings.Contains(l, "specify device type"):
			return "unsupported", m.String
		case m.Severity == "error" || msg == "smartctl could not read the disk":
			msg = m.String
		}
	}
	return reason, msg
}

func fillIdentity(d *SmartDetail, j *scFull, ex detailExtras) {
	id := &d.Identity
	id.Model, id.Family, id.Firmware, id.Serial = j.ModelName, j.ModelFamily, j.FirmwareVersion, j.SerialNumber
	if id.Model == "" {
		id.Model = strings.TrimSpace(j.SCSIVendor + " " + j.SCSIProduct)
	}
	if w := j.WWN; w != nil {
		id.WWN = fmt.Sprintf("0x%x%06x%09x", w.NAA, w.OUI, w.ID)
	}
	if j.UserCapacity != nil {
		id.CapacityBytes = j.UserCapacity.Bytes
	}
	id.BlockSize = j.LogicalBlockSize
	id.RotationRPM = j.RotationRate
	if j.FormFactor != nil {
		id.FormFactor = j.FormFactor.Name
	}
	if j.Trim != nil {
		id.Trim = &j.Trim.Supported
	}
	id.KnownModel = j.InSmartctlDatabase
	switch {
	case j.NVMeVersion != nil:
		id.Standard = "NVMe " + j.NVMeVersion.String
	case j.ATAVersion != nil:
		id.Standard = j.ATAVersion.String
	}
	if j.SATAVersion != nil {
		id.Interface = j.SATAVersion.String
	}
	if s := j.InterfaceSpeed; s != nil {
		if s.Current != nil {
			id.LinkCurrent = s.Current.String
		}
		if s.Max != nil {
			id.LinkMax = s.Max.String
		}
	}
	if ex.LinkCurrent != "" {
		id.LinkCurrent, id.LinkMax = ex.LinkCurrent, ex.LinkMax
	}
}

// fillCommon fills what every protocol reports the same way.
func fillCommon(d *SmartDetail, j *scFull, add func(level, code, text string)) {
	v := &d.Vitals
	if t := j.Temperature; t != nil {
		if t.Current != nil {
			v.Temperature = t.Current
		}
		v.TempLifetimeMax, v.TempLifetimeMin = t.LifetimeMax, t.LifetimeMin
		if t.OpLimitMax != nil && v.TempLimit == nil {
			v.TempLimit = t.OpLimitMax
		}
	}
	if j.PowerOnTime != nil && j.PowerOnTime.Hours != nil {
		v.PowerOnHours = j.PowerOnTime.Hours
	}
	if j.PowerCycleCount != nil {
		v.PowerCycles = j.PowerCycleCount
	}
	if j.SCSIGrownDefects != nil {
		v.Reallocated = j.SCSIGrownDefects
		counterLevel(d, add, "reallocated", *j.SCSIGrownDefects, "grown defects (reallocated sectors)")
	}
}

// counterLevel sets the level of a media/link error counter: any non-zero value is a warning.
func counterLevel(d *SmartDetail, add func(level, code, text string), key string, n float64, what string) {
	if n > 0 {
		d.Vitals.Levels[key] = lvWarn
		add(lvWarn, key, fmt.Sprintf("%s %s.", fmtCount(n), what))
	} else {
		d.Vitals.Levels[key] = lvOK
	}
}

func fmtCount(n float64) string {
	s := fmt.Sprintf("%.0f", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
