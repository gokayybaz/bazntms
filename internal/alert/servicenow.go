package alert

// ServiceNow Table API (incident) entegrasyonu (Faz 22 S22.15). Uyarı grubu
// ilk ateşlendiğinde bir incident açılır, grup çözüldüğünde state=6 (Resolved).
// ext_ref = "snow:<sys_id>". Parola vault ile şifreli (Manager.decrypt).
//
// "Temel destek": alan adları out-of-box incident tablosuna göre. Özelleştirilmiş
// instance'larda impact/urgency/state kodları farklı olabilir — docs/ALERTING.md.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

const snowRefPrefix = "snow:"

// severity → (impact, urgency) — ServiceNow 1=high .. 3=low.
var snowImpact = map[string][2]string{
	"crit": {"1", "1"},
	"warn": {"2", "2"},
	"info": {"3", "3"},
}

func (m *Manager) snowFire(sc SNowConfig, ev store.AlertEvent) {
	pw := m.decrypt(sc.Password)
	if ref, _ := m.st.GroupExtRef(ev.GroupID); strings.HasPrefix(ref, snowRefPrefix) {
		err := snowUpdate(sc, pw, strings.TrimPrefix(ref, snowRefPrefix), map[string]any{
			"work_notes": "Aynı gruba yeni uyarı: " + ev.Message,
		})
		m.notifier.record("servicenow", err)
		if err == nil {
			_ = m.st.SetAlertEventExtRef(ev.ID, ref)
		}
		return
	}
	iu := snowImpact[ev.Severity]
	if iu[0] == "" {
		iu = [2]string{"2", "2"}
	}
	sysID, err := snowCreate(sc, pw, map[string]any{
		"short_description": fmt.Sprintf("[bazNTMS/%s] %s", ev.Kind, truncItem([]byte(ev.Message), 150)),
		"description": fmt.Sprintf("önem: %s\ntür: %s\nkapsam: %s\nanahtar: %s\n\n%s",
			ev.Severity, ev.Kind, orDash(ev.Site), ev.Key, ev.Message),
		"impact":   iu[0],
		"urgency":  iu[1],
		"category": "network",
	})
	m.notifier.record("servicenow", err)
	if err == nil && sysID != "" {
		_ = m.st.SetAlertEventExtRef(ev.ID, snowRefPrefix+sysID)
	}
}

func (m *Manager) snowResolve(sc SNowConfig, ref, reason string) {
	sysID := strings.TrimPrefix(ref, snowRefPrefix)
	err := snowUpdate(sc, m.decrypt(sc.Password), sysID, map[string]any{
		"state":       "6", // Resolved
		"close_code":  "Resolved by caller",
		"close_notes": "bazNTMS: uyarı çözüldü — " + reason,
	})
	if err != nil {
		m.notifier.record("servicenow", err)
	}
}

func snowDo(sc SNowConfig, pw, method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(sc.BaseURL, "/")+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(sc.User+":"+pw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("servicenow %s %s: %d %s", method, path, resp.StatusCode, truncItem(raw, 200))
	}
	return raw, nil
}

func snowCreate(sc SNowConfig, pw string, fields map[string]any) (string, error) {
	raw, err := snowDo(sc, pw, http.MethodPost, "/api/now/table/incident", fields)
	if err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			SysID  string `json:"sys_id"`
			Number string `json:"number"`
		} `json:"result"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.Result.SysID, nil
}

func snowUpdate(sc SNowConfig, pw, sysID string, fields map[string]any) error {
	_, err := snowDo(sc, pw, http.MethodPatch, "/api/now/table/incident/"+sysID, fields)
	return err
}
