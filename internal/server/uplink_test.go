package server

// Canlı Akış gruplama: agent → switch/AP uplink atama endpoint'i.

import (
	"net/http"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAgentSetUplink(t *testing.T) {
	ts, _, st := newRBACServerEx(t, "admin-uplink-pw", "", false)
	_, tok := login(t, ts, "admin-uplink-pw") // legacy → admin (PermManageAgents)

	agentID, err := st.RegisterAgent(store.Agent{Name: "loadgen-0001", TokenHash: store.TokenHash("a")})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	sw, err := st.AddDevice(store.Device{Name: "kat1-sw", Kind: "switch", PollSeconds: 60})
	if err != nil {
		t.Fatalf("add switch: %v", err)
	}
	other, err := st.AddDevice(store.Device{Name: "misc", Kind: "other", PollSeconds: 60})
	if err != nil {
		t.Fatalf("add other: %v", err)
	}

	// happy path: switch ata
	if code, out := putJSON(t, ts, "/api/v1/agents/1/uplink", tok, map[string]any{"device_id": sw}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("uplink ata: %d %v", code, out)
	}
	if a, _ := st.AgentByID(agentID); a.UplinkDeviceID == nil || *a.UplinkDeviceID != sw {
		t.Fatalf("uplink DB'ye yazılmadı: %+v", a)
	}

	// yanlış tür (other) → 400
	if code, _ := putJSON(t, ts, "/api/v1/agents/1/uplink", tok, map[string]any{"device_id": other}); code != http.StatusBadRequest {
		t.Fatalf("other türü reddedilmeliydi: %d", code)
	}

	// olmayan cihaz → 400
	if code, _ := putJSON(t, ts, "/api/v1/agents/1/uplink", tok, map[string]any{"device_id": 9999}); code != http.StatusBadRequest {
		t.Fatalf("olmayan cihaz reddedilmeliydi: %d", code)
	}

	// kaldır (null) → 200, nil
	if code, _ := putJSON(t, ts, "/api/v1/agents/1/uplink", tok, map[string]any{"device_id": nil}); code != http.StatusOK {
		t.Fatalf("uplink kaldır: %d", code)
	}
	if a, _ := st.AgentByID(agentID); a.UplinkDeviceID != nil {
		t.Fatalf("uplink kaldırılmadı: %v", *a.UplinkDeviceID)
	}

	// yetkisiz (token yok) → 401/403
	if code, _ := putJSON(t, ts, "/api/v1/agents/1/uplink", "", map[string]any{"device_id": sw}); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("yetkisiz istek geçmemeliydi: %d", code)
	}
}

// TestDeviceAddUnmanaged, host'suz "yönetilmeyen" switch/AP kaydına izin
// verildiğini ve Enabled=false olduğunu (poll edilmeyeceğini) doğrular.
func TestDeviceAddUnmanaged(t *testing.T) {
	ts, _, st := newRBACServerEx(t, "admin-dev-pw", "", false)
	_, tok := login(t, ts, "admin-dev-pw")

	code, out := postJSON(t, ts, "/api/v1/devices", tok, map[string]any{"name": "kat2-ap", "kind": "ap"})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("host'suz AP eklenemedi: %d %v", code, out)
	}
	devs, _ := st.ListDevices("")
	if len(devs) != 1 || devs[0].Enabled {
		t.Fatalf("host'suz cihaz Enabled=false olmalı: %+v", devs)
	}

	// host'suz + geçersiz tür → 400
	if code, _ := postJSON(t, ts, "/api/v1/devices", tok, map[string]any{"name": "x", "kind": "server"}); code != http.StatusBadRequest {
		t.Fatalf("host'suz geçersiz tür reddedilmeliydi: %d", code)
	}
}
