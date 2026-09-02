package driver

// Sessiz SNMP arızası tespiti (snmpNoTelemetryErr) birim testleri.

import (
	"errors"
	"strings"
	"testing"
)

func TestSNMPNoTelemetryErr(t *testing.T) {
	timeout := errors.New("request timeout (after 1 retries)")

	tests := []struct {
		name    string
		walkOK  int
		sysOK   bool
		cause   error
		wantSub string // hata mesajında geçmesi gereken alt dize
		wrapped error  // errors.Is ile eşleşmesi beklenen (nil = kontrol etme)
	}{
		{
			name:    "hicbir yanit yok — cause var",
			walkOK:  0,
			sysOK:   false,
			cause:   timeout,
			wantSub: "yanıt vermiyor",
			wrapped: timeout,
		},
		{
			name:    "hicbir yanit yok — cause nil",
			walkOK:  0,
			sysOK:   false,
			cause:   nil,
			wantSub: "yanıt vermiyor",
		},
		{
			name:    "sysDescr geldi ama IF-MIB bos",
			walkOK:  0,
			sysOK:   true,
			cause:   nil,
			wantSub: "IF-MIB",
		},
		{
			name:    "bazi kolonlar yurudu ama arayuz cikmadi",
			walkOK:  3,
			sysOK:   false,
			cause:   timeout,
			wantSub: "IF-MIB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := snmpNoTelemetryErr("10.0.0.1", tt.walkOK, tt.sysOK, tt.cause)
			if err == nil {
				t.Fatal("hata bekleniyordu, nil döndü")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("mesaj %q alt dizesini içermeli: %v", tt.wantSub, err)
			}
			if !strings.Contains(err.Error(), "10.0.0.1") {
				t.Fatalf("mesaj host'u içermeli: %v", err)
			}
			if tt.wrapped != nil && !errors.Is(err, tt.wrapped) {
				t.Fatalf("errors.Is(%v, cause) bekleniyordu", err)
			}
		})
	}
}
