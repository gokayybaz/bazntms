package alert

// S22.14/S22.15: Jira + ServiceNow — httptest mock ile issue/incident yaşam
// döngüsü (canlı kimlik yok).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newTicketMgr(t *testing.T, cfg Config) (*Manager, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tkt.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(cfg, st, capture.NewEngine(), 30), st
}

func TestJiraLifecycle(t *testing.T) {
	var mu sync.Mutex
	created, commented, transitioned := 0, 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/3/issue":
			created++
			json.NewEncoder(w).Encode(map[string]string{"key": "OPS-42"})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/comment"):
			commented++
			w.WriteHeader(201)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/transitions"):
			json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]string{{"id": "31", "name": "Done"}}})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/transitions"):
			transitioned++
			w.WriteHeader(204)
		default:
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Notifiers.Jira = JiraConfig{Enabled: true, BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok", Project: "OPS"}
	m, st := newTicketMgr(t, cfg)

	m.fireCtx("ioc", "evil.com", "tehdit alan adı", fireOpts{Site: "dc1"})
	waitFor(t, func() bool {
		e, _ := st.OpenAlertEventByKey("ioc", "evil.com")
		return e != nil && e.ExtRef == "jira:OPS-42"
	})
	mu.Lock()
	if created != 1 {
		t.Fatalf("1 issue oluşturulmalıydı, %d", created)
	}
	mu.Unlock()

	// çöz → yorum + transition
	e, _ := st.OpenAlertEventByKey("ioc", "evil.com")
	m.resolveEvent(cfg, *e, "operatör kapattı")
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return transitioned == 1 })
}

func TestServiceNowCreate(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/now/table/incident" {
			json.NewDecoder(r.Body).Decode(&got)
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]string{"sys_id": "abc123", "number": "INC0001"}})
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Notifiers.ServiceNow = SNowConfig{Enabled: true, BaseURL: srv.URL, User: "u", Password: "p"}
	m, st := newTicketMgr(t, cfg)

	m.fireCtx("vpn_down", "dev1", "VPN düştü", fireOpts{Site: "dc2"})
	waitFor(t, func() bool {
		e, _ := st.OpenAlertEventByKey("vpn_down", "dev1")
		return e != nil && e.ExtRef == "snow:abc123"
	})
	if got["impact"] != "1" { // vpn_down → crit → impact 1
		t.Fatalf("crit → impact 1 bekleniyordu: %v", got["impact"])
	}
}

// routing: jira kanalı yönlendirmeyle devre dışı bırakılırsa issue açılmaz
func TestTicketRespectsRouting(t *testing.T) {
	var created int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/rest/api/3/issue" {
			mu.Lock()
			created++
			mu.Unlock()
		}
		json.NewEncoder(w).Encode(map[string]string{"key": "X-1"})
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Notifiers.Jira = JiraConfig{Enabled: true, BaseURL: srv.URL, Email: "a@b", APIToken: "t", Project: "X"}
	cfg.NotifyRoutes = []NotifyRoute{{Kind: "proc", Channels: []string{ChSlack}}} // jira YOK
	m, st := newTicketMgr(t, cfg)

	m.fireCtx("proc", "a1:nc", "yeni süreç", fireOpts{Site: "dc1"})
	time.Sleep(150 * time.Millisecond)
	if e, _ := st.OpenAlertEventByKey("proc", "a1:nc"); e == nil || e.ExtRef != "" {
		t.Fatalf("yönlendirme jira'yı dışladı → ext_ref boş olmalı: %+v", e)
	}
	mu.Lock()
	if created != 0 {
		t.Fatalf("yönlendirme dışlarken issue oluşturulmamalı, %d", created)
	}
	mu.Unlock()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("koşul zaman aşımına uğradı")
}
