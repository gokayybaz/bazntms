package server

// Faz 25-C: denetim kaydı v2 — durum farkı yakalama, sır maskesi, süzgeçler.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestRedactAuditJSON(t *testing.T) {
	in := map[string]any{
		"role":     "admin",
		"password": "hunter2",
		"nested": map[string]any{
			"api_token": "sk-secret",
			"host":      "10.0.0.1",
		},
		"list": []any{
			map[string]any{"community": "public", "name": "sw1"},
		},
		"empty_secret": "",
	}
	out := redactAuditJSON(in)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "sk-secret") || strings.Contains(out, `"public"`) {
		t.Fatalf("sır sızdı: %s", out)
	}
	if !strings.Contains(out, "admin") || !strings.Contains(out, "10.0.0.1") || !strings.Contains(out, "sw1") {
		t.Fatalf("sır olmayan alan kayboldu: %s", out)
	}
	if !strings.Contains(out, secretMask) {
		t.Fatalf("maske uygulanmadı: %s", out)
	}
	if redactAuditJSON(nil) != "" {
		t.Fatal("nil → boş string olmalı")
	}
}

// auditEvents, denetim listesini dizi olarak çeker (handleAuditList dizi döner,
// getJSON map bekler → burada elle).
func auditEvents(t *testing.T, ts *httptest.Server, token, query string) []store.AuditEvent {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/audit"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("audit istek: %v", err)
	}
	defer resp.Body.Close()
	var out []store.AuditEvent
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("audit decode: %v", err)
	}
	return out
}

func TestAuditV2CapturesDiffAndFilters(t *testing.T) {
	ts, _, _ := newRBACServerEx(t, "legacy-pass-1", "", false)

	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pass-1"})
	adminTok, _ := out["token"].(string)

	// kullanıcı oluştur → güncelle (rol değişimi)
	status, out := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "bob", "password": "bob-password-1", "role": "viewer", "site": "dc1",
	})
	if status != http.StatusOK {
		t.Fatalf("kullanıcı oluşturma: %d %v", status, out)
	}
	uid := int64(out["id"].(float64))

	status, _ = putJSON(t, ts, "/api/v1/users/"+strconv.FormatInt(uid, 10), adminTok, map[string]any{"role": "analyst"})
	if status != http.StatusOK {
		t.Fatalf("kullanıcı güncelleme: %d", status)
	}

	// user.update denetim kaydı: before/after + actor_type + request_id + result
	evs := auditEvents(t, ts, adminTok, "?action=user.update")
	if len(evs) != 1 {
		t.Fatalf("user.update: 1 kayıt beklenirdi, gelen %d", len(evs))
	}
	e := evs[0]
	if e.ActorType != "legacy" {
		t.Errorf("actor_type=legacy beklenirdi: %q", e.ActorType)
	}
	if e.Result != "ok" {
		t.Errorf("result=ok beklenirdi: %q", e.Result)
	}
	if e.RequestID == "" {
		t.Error("request_id boş")
	}
	if !strings.Contains(e.BeforeJSON, `"viewer"`) || !strings.Contains(e.AfterJSON, `"analyst"`) {
		t.Errorf("durum farkı yanlış: before=%s after=%s", e.BeforeJSON, e.AfterJSON)
	}

	// action prefix süzgeci
	evs = auditEvents(t, ts, adminTok, "?action=user.*")
	if len(evs) < 2 {
		t.Fatalf("user.* : ≥2 beklenirdi (create+update), gelen %d", len(evs))
	}

	// actor süzgeci
	evs = auditEvents(t, ts, adminTok, "?actor=admin")
	if len(evs) == 0 {
		t.Fatal("actor=admin: kayıt yok")
	}
	for _, ev := range evs {
		if !strings.Contains(ev.Username, "admin") {
			t.Fatalf("actor süzgeci sızdırdı: %q", ev.Username)
		}
	}

	// denied → result=denied (analyst'in yönetim ucu)
	_, out = postJSON(t, ts, "/api/login", "", map[string]string{"username": "bob", "password": "bob-password-1"})
	bobTok, _ := out["token"].(string)
	if st2, _ := getJSON(t, ts, "/api/v1/users", bobTok); st2 != http.StatusForbidden {
		t.Fatalf("bob analyst users 403 beklenirdi: %d", st2)
	}
	evs = auditEvents(t, ts, adminTok, "?result=denied")
	if len(evs) == 0 || evs[0].Action != "denied" {
		t.Fatalf("result=denied kaydı yok: %+v", evs)
	}

	// zincir hâlâ sağlam
	_, vout := getJSON(t, ts, "/api/v1/audit/verify", adminTok)
	if vout["ok"] != true {
		t.Fatalf("v2 kayıtlardan sonra zincir bozuk: %v", vout)
	}
}
