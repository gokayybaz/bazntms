package server

import (
	"net/http"
	"testing"
)

// TestMockDeviceVendorGated, vendor=mock cihazın yalnızca SetMockDevices(true)
// ile kabul edildiğini doğrular (S21.2 — ölçek testi kapısı).
func TestMockDeviceVendorGated(t *testing.T) {
	ts, srv, _ := newRBACServerEx(t, "admin-pass-1", "", false)

	status, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "admin-pass-1"})
	if status != http.StatusOK {
		t.Fatalf("login: %d", status)
	}
	tok, _ := out["token"].(string)

	dev := map[string]any{"name": "mock-0001", "host": "10.99.0.1", "kind": "switch", "vendor": "mock", "poll_seconds": 60}

	// kapalı → 400
	if status, _ := postJSON(t, ts, "/api/v1/devices", tok, dev); status != http.StatusBadRequest {
		t.Fatalf("mock-devices kapalıyken 400 beklenirdi, geldi %d", status)
	}

	// açık → 200
	srv.SetMockDevices(true)
	status, out = postJSON(t, ts, "/api/v1/devices", tok, dev)
	if status != http.StatusOK {
		t.Fatalf("mock-devices açıkken 200 beklenirdi, geldi %d %v", status, out)
	}
	if _, ok := out["id"]; !ok {
		t.Fatalf("cihaz id dönmedi: %v", out)
	}
}
