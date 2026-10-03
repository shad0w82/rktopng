package collectors

import (
	"regexp"
	"strconv"
	"strings"
)

// What an ATA SMART attribute means is up to the drive's vendor: the same ID is
// "Wear_Leveling_Count" on a Samsung and something else on another brand, and
// the raw value of "Total_LBAs_Written" is in sectors while "Lifetime_Writes_GiB"
// is already in GiB. smartctl resolves the vendor's name from its drive database,
// so the roles below are recognised from that NAME (with the ID only where every
// name smartctl knows for the ID means the same thing). Anything not recognised
// keeps no role: it is still shown in the table, but it never feeds a summary
// figure or the health state.
const (
	roleReallocated    = "reallocated"           // bad sectors/blocks replaced by spares
	rolePending        = "pending"               // sectors waiting to be remapped
	roleOfflineUnc     = "offline_uncorrectable" // sectors a background scan could not read
	roleUncorrectable  = "uncorrectable"         // errors the ECC could not recover
	roleCRC            = "crc"                   // interface (cable) CRC errors
	roleTemperature    = "temperature"
	rolePowerOnHours   = "power_on_hours"
	rolePowerCycles    = "power_cycles"
	roleWear           = "wear"            // endurance indicator (normalized value = life left)
	roleWritten        = "written"         // host data written
	roleRead           = "read"            // host data read
	roleUnsafeShutdown = "unsafe_shutdown" // power lost without a clean shutdown
)

// Counters a drive increments when something is wrong with the media or the link.
// A non-zero raw value on one of them is a warning, whatever the brand.
var counterRoles = map[string]bool{
	roleReallocated: true, rolePending: true, roleOfflineUnc: true, roleUncorrectable: true, roleCRC: true,
}

var (
	tempNameRe = regexp.MustCompile(`^(airflow_temperature_cel|case_temperature|controller_temperature|device_temperature|drive_temperature|temperature_(case|celsius|internal))$`)

	// "...remaining/left" attributes hold the life left in their normalized value (100 = new, falling);
	// the "...used" ones are its mirror image. Spare-block counters and "Wear_Range_Delta" are not life gauges.
	wearNameRe = regexp.MustCompile(`^(wear_leveling_count|media_wearout_indicator|ssd_life_?left(_perc)?|lifetime_(remaining|left)|` +
		`(percent|perc_rated|pct)_(lifetime|life)_(remain\w*|used)|remaining_(lifetime_perc|life)|drive_?life_(remaining|used))$`)

	unsafeNameRe = regexp.MustCompile(`^(unsafe_shutdown_count|unclean_shutdown_ct|unexpect(ed)?_(power_?loss|pwr_?loss|power_cycle)(_c(n)?t)?|unexpected_pwr_loss_c(n)?t|por_recovery_count|power-?off_retract_count)$`)

	crcNameRe = regexp.MustCompile(`crc`)

	reallocNameRe = regexp.MustCompile(`realloc|retire|bad_?bl|remap`)
	uncorrNameRe  = regexp.MustCompile(`unc|reported`)
	pendingNameRe = regexp.MustCompile(`pending|unc`)
	offlineNameRe = regexp.MustCompile(`unc`)

	// Host traffic counters: a direction word, a "who counts" word, and a unit.
	ioWriteRe   = regexp.MustCompile(`writ|wts`)
	ioReadRe    = regexp.MustCompile(`read|rds`)
	ioWhoRe     = regexp.MustCompile(`host|hst|lifetime|total|lbas`)
	ioNotHostRe = regexp.MustCompile(`nand|flash|flsh|slc|tlc|erase|amp|protect|error|head|throttl|command|_ct|_cnt|count|perc|fly|retry|bad|disturb|bits|_high|_low|log_`)
	ioLBARe     = regexp.MustCompile(`lbas?|sectors?`)
	ioUnitRe    = regexp.MustCompile(`(\d*)([kmgt])(i?)b$`)

	leadingIntRe = regexp.MustCompile(`^\s*(\d+)`)
)

// attrRole returns the role of an attribute, or "" when it has none we can trust.
func attrRole(id int, name string) string {
	n := strings.ToLower(name)
	switch {
	case tempNameRe.MatchString(n):
		return roleTemperature
	case crcNameRe.MatchString(n):
		return roleCRC
	case wearNameRe.MatchString(n):
		return roleWear
	case unsafeNameRe.MatchString(n):
		return roleUnsafeShutdown
	case strings.HasPrefix(n, "power_on_hours"):
		return rolePowerOnHours
	case strings.HasPrefix(n, "power_cycle_count"):
		return rolePowerCycles
	}
	if role, _, ok := hostIO(n); ok {
		return role
	}
	// Counters whose ID has the same meaning on every brand: the ID AND a name of that kind are both
	// required, so an attribute the vendor marks "Not_In_Use" or "Unknown_..." never becomes a counter.
	switch id {
	case 5:
		if reallocNameRe.MatchString(n) {
			return roleReallocated
		}
	case 187:
		if uncorrNameRe.MatchString(n) {
			return roleUncorrectable
		}
	case 197:
		if pendingNameRe.MatchString(n) {
			return rolePending
		}
	case 198:
		if offlineNameRe.MatchString(n) {
			return roleOfflineUnc
		}
	}
	return ""
}

// hostIO decodes a host-traffic attribute name (Total_LBAs_Written, Host_Writes_32MiB,
// Lifetime_Writes_GiB, ...) into its direction and the size of one raw unit in bytes.
// bytesPerUnit is 0 for counters in logical sectors: the caller multiplies by the
// drive's logical block size. NAND/flash writes (they include write amplification),
// counters of commands and names without a unit are not host traffic we can size.
func hostIO(lowerName string) (role string, bytesPerUnit float64, ok bool) {
	n := lowerName
	if ioNotHostRe.MatchString(n) || !ioWhoRe.MatchString(n) {
		return "", 0, false
	}
	switch {
	case ioWriteRe.MatchString(n) && !ioReadRe.MatchString(n):
		role = roleWritten
	case ioReadRe.MatchString(n) && !ioWriteRe.MatchString(n):
		role = roleRead
	default:
		return "", 0, false
	}
	if m := ioUnitRe.FindStringSubmatch(n); m != nil {
		mult := float64(1)
		if m[1] != "" {
			v, err := strconv.Atoi(m[1])
			if err != nil || v == 0 {
				return "", 0, false
			}
			mult = float64(v)
		}
		base := float64(1000)
		if m[3] == "i" {
			base = 1024
		}
		for _, p := range "kmgt" {
			mult *= base
			if string(p) == m[2] {
				break
			}
		}
		return role, mult, true
	}
	if ioLBARe.MatchString(n) {
		return role, 0, true
	}
	return "", 0, false
}

// attrRawInt is the number a person reads in smartctl's raw column. It parses the
// leading integer of the formatted string ("34 (Min/Max 20/45)" -> 34) because the
// numeric field also carries packed extra bytes on some drives; hex-formatted raws
// fall back to the numeric field.
func attrRawInt(rawString string, rawValue float64) float64 {
	if m := leadingIntRe.FindStringSubmatch(rawString); m != nil && !strings.HasPrefix(strings.TrimSpace(rawString), "0x") {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	return rawValue
}
