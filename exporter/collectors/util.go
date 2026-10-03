package collectors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Namespace prefix for every metric exposed by this exporter.
const Namespace = "rk3588"

// Base paths for procfs and sysfs. When running in a container the host
// filesystems are bind-mounted and these env vars point at the mount points
// (e.g. /host/proc, /host/sys). Default to the real paths for native runs.
var (
	ProcPath = getenv("RKTOP_PROC_PATH", "/proc")
	SysPath  = getenv("RKTOP_SYS_PATH", "/sys")

	// OSReleasePath is the os-release file describing the host OS. In a container
	// the host's file is bind-mounted somewhere and this points at it.
	OSReleasePath = getenv("RKTOP_OSRELEASE_PATH", "/etc/os-release")
	// DevPath is where block-device nodes (/dev/sda, ...) live, used by SMART.
	DevPath = getenv("RKTOP_DEV_PATH", "/dev")
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Proc joins p onto the procfs base path.
func Proc(p string) string { return filepath.Join(ProcPath, p) }

// Sys joins p onto the sysfs base path.
func Sys(p string) string { return filepath.Join(SysPath, p) }

// readTrim reads a file and returns its trimmed string content.
func readTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// readUint reads a file containing a single unsigned integer.
func readUint(path string) (uint64, error) {
	s, err := readTrim(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

// readFloat reads a file containing a single float.
func readFloat(path string) (float64, error) {
	s, err := readTrim(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(s, 64)
}

// exists reports whether a path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fields splits on whitespace, collapsing runs.
func fields(s string) []string { return strings.Fields(s) }
