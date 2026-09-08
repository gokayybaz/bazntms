package alert

// Jira Cloud entegrasyonu (Faz 22 S22.14): uyarı grubu ilk ateşlendiğinde bir
// issue oluşturulur, grup çözüldüğünde issue geçiş yapar + yorum düşülür.
// ext_ref = "jira:<PROJ-123>". API token vault ile şifreli (Manager.decrypt).

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

const jiraRefPrefix = "jira:"

// jiraFire, bir uyarı için Jira issue'su oluşturur (grup zaten biletlenmişse
// yorum ekler) ve ext_ref'i olaya yazar.
func (m *Manager) jiraFire(jc JiraConfig, ev store.AlertEvent) {
	token := m.decrypt(jc.APIToken)
	if ref, _ := m.st.GroupExtRef(ev.GroupID); strings.HasPrefix(ref, jiraRefPrefix) {
		key := strings.TrimPrefix(ref, jiraRefPrefix)
		err := jiraComment(jc, token, key, "Aynı gruba yeni uyarı: "+ev.Message)
		m.notifier.record("jira", err)
		if err == nil {
			_ = m.st.SetAlertEventExtRef(ev.ID, ref)
		}
		return
	}
	key, err := jiraCreateIssue(jc, token, ev)
	m.notifier.record("jira", err)
	if err != nil {
		return
	}
	_ = m.st.SetAlertEventExtRef(ev.ID, jiraRefPrefix+key)
}

// jiraResolve, çözülen bir olayın Jira issue'sunu kapatır.
func (m *Manager) jiraResolve(jc JiraConfig, ref, reason string) {
	key := strings.TrimPrefix(ref, jiraRefPrefix)
	token := m.decrypt(jc.APIToken)
	_ = jiraComment(jc, token, key, "bazNTMS: uyarı çözüldü — "+reason)
	trans := jc.ResolveTransition
	if trans == "" {
		trans = "Done"
	}
	if err := jiraTransition(jc, token, key, trans); err != nil {
		m.notifier.record("jira", err)
	}
}

func jiraAuth(jc JiraConfig, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(jc.Email+":"+token))
}

func jiraDo(jc JiraConfig, token, method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(jc.BaseURL, "/")+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", jiraAuth(jc, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	cl := &http.Client{Timeout: 15 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira %s %s: %d %s", method, path, resp.StatusCode, truncItem(raw, 200))
	}
	return raw, nil
}

func jiraCreateIssue(jc JiraConfig, token string, ev store.AlertEvent) (string, error) {
	it := jc.IssueType
	if it == "" {
		it = "Task"
	}
	summary := fmt.Sprintf("[%s/%s] %s", strings.ToUpper(ev.Severity), kindLabel(ev.Kind), truncItem([]byte(ev.Message), 200))
	payload := map[string]any{
		"fields": map[string]any{
			"project":   map[string]string{"key": jc.Project},
			"issuetype": map[string]string{"name": it},
			"summary":   summary,
			"description": adfDoc(fmt.Sprintf(
				"bazNTMS uyarısı\nönem: %s\ntür: %s\nkapsam: %s\nanahtar: %s\n\n%s",
				ev.Severity, ev.Kind, orDash(ev.Site), ev.Key, ev.Message)),
		},
	}
	raw, err := jiraDo(jc, token, http.MethodPost, "/rest/api/3/issue", payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.Key == "" {
		return "", fmt.Errorf("jira: yanıtta issue key yok")
	}
	return out.Key, nil
}

func jiraComment(jc JiraConfig, token, key, text string) error {
	_, err := jiraDo(jc, token, http.MethodPost, "/rest/api/3/issue/"+key+"/comment",
		map[string]any{"body": adfDoc(text)})
	return err
}

func jiraTransition(jc JiraConfig, token, key, name string) error {
	raw, err := jiraDo(jc, token, http.MethodGet, "/rest/api/3/issue/"+key+"/transitions", nil)
	if err != nil {
		return err
	}
	var tr struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	_ = json.Unmarshal(raw, &tr)
	var id string
	for _, t := range tr.Transitions {
		if strings.EqualFold(t.Name, name) {
			id = t.ID
			break
		}
	}
	if id == "" {
		return fmt.Errorf("jira: '%s' geçişi bulunamadı", name)
	}
	_, err = jiraDo(jc, token, http.MethodPost, "/rest/api/3/issue/"+key+"/transitions",
		map[string]any{"transition": map[string]string{"id": id}})
	return err
}

// adfDoc, düz metni Jira Cloud'un Atlassian Document Format zarfına sarar.
func adfDoc(text string) map[string]any {
	var content []any
	for _, line := range strings.Split(text, "\n") {
		para := map[string]any{"type": "paragraph"}
		if line != "" {
			para["content"] = []any{map[string]any{"type": "text", "text": line}}
		}
		content = append(content, para)
	}
	return map[string]any{"type": "doc", "version": 1, "content": content}
}

func truncItem(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
