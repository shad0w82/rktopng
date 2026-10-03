package collectors

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var typecPortRe = regexp.MustCompile(`^port\d+$`) // excludes port0-partner, port0-cable, ...

// bracketed returns the active choice of a sysfs attribute written as
// "source [sink]" or "[C] PD PD_PPS" (the value between brackets). Attributes
// without brackets are returned trimmed.
func bracketed(s string) string {
	s = strings.TrimSpace(s)
	if a := strings.Index(s, "["); a >= 0 {
		if b := strings.Index(s[a:], "]"); b > 0 {
			return strings.TrimSpace(s[a+1 : a+b])
		}
	}
	return s
}

// TypeCCollector exposes the state of the USB-C port: whether a PD contract is
// powering the board, which USB type is active and what role the port plays.
// Voltage/current of the contract are exposed by PowerCollector. Readable
// without root.
type TypeCCollector struct {
	pdOnline *prometheus.Desc
	pdType   *prometheus.Desc
	port     *prometheus.Desc
}

func NewTypeCCollector() *TypeCCollector {
	return &TypeCCollector{
		pdOnline: prometheus.NewDesc(Namespace+"_usb_pd_online",
			"1 if an external USB-C source is currently powering the board, 0 if there is no contract.", nil, nil),
		pdType: prometheus.NewDesc(Namespace+"_usb_pd_type",
			"Active USB power type of the USB-C source (C, PD, PD_PPS, ...); value is always 1.", []string{"type"}, nil),
		port: prometheus.NewDesc(Namespace+"_typec_port_info",
			"USB-C port state (value is always 1): roles, operation mode, orientation, PD revision and whether a partner is attached.",
			[]string{"port", "power_role", "data_role", "power_operation_mode", "orientation", "pd_revision", "partner"}, nil),
	}
}

func (c *TypeCCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pdOnline
	ch <- c.pdType
	ch <- c.port
}

func (c *TypeCCollector) Collect(ch chan<- prometheus.Metric) {
	// Power-supply side: tcpm-source-psy-* is the source that powers us when the port is a sink.
	psy, _ := filepath.Glob(Sys("class/power_supply/tcpm-source-psy*"))
	for _, d := range psy {
		if v, err := readFloat(filepath.Join(d, "online")); err == nil {
			ch <- prometheus.MustNewConstMetric(c.pdOnline, prometheus.GaugeValue, v)
		}
		if s, err := readTrim(filepath.Join(d, "usb_type")); err == nil && s != "" {
			ch <- prometheus.MustNewConstMetric(c.pdType, prometheus.GaugeValue, 1, bracketed(s))
		}
		break // one USB-C port on the CM3588
	}

	// Port side: roles and whether something is plugged in.
	ports, _ := filepath.Glob(Sys("class/typec/port*"))
	for _, d := range ports {
		name := filepath.Base(d)
		if !typecPortRe.MatchString(name) {
			continue
		}
		attr := func(f string) string { s, _ := readTrim(filepath.Join(d, f)); return bracketed(s) }
		partner := "no"
		if exists(Sys("class/typec/" + name + "-partner")) {
			partner = "yes"
		}
		ch <- prometheus.MustNewConstMetric(c.port, prometheus.GaugeValue, 1,
			name, attr("power_role"), attr("data_role"), attr("power_operation_mode"),
			attr("orientation"), attr("usb_power_delivery_revision"), partner)
	}
}
