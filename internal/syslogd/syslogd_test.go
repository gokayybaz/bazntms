package syslogd

import (
	"testing"
	"time"
)

func TestParseRFC3164(t *testing.T) {
	line := "<134>Aug 28 20:14:33 rt-01 kernel: DROP IN=eth0 SRC=1.2.3.4"
	ev := ParseRFC3164(line, "10.0.0.9", testNow())
	if ev.Severity != 6 || ev.Host != "rt-01" || ev.Tag != "kernel" {
		t.Fatalf("alanlar hatali: %+v", ev)
	}
	if ev.Message != "DROP IN=eth0 SRC=1.2.3.4" {
		t.Fatalf("mesaj hatali: %q", ev.Message)
	}
}

func TestParseRFC3164WithTagPID(t *testing.T) {
	line := "<11>Aug 28 20:14:33 fw-02 sshd[4213]: Failed password for root"
	ev := ParseRFC3164(line, "10.0.0.9", testNow())
	if ev.Tag != "sshd" || ev.Severity != 3 {
		t.Fatalf("tag/severity hatali: %+v", ev)
	}
}

func TestParseRFC3164NonStandard(t *testing.T) {
	// format bozuksa tum satir mesaj olarak korunur
	ev := ParseRFC3164("rastgele satir", "10.0.0.9", testNow())
	if ev.Severity != 7 || ev.Message != "rastgele satir" || ev.Host != "10.0.0.9" {
		t.Fatalf("fallback hatali: %+v", ev)
	}
}

func testNow() time.Time { return time.Date(2026, 8, 28, 20, 0, 0, 0, time.UTC) }
