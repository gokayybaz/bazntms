package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

// TestWSTickCarriesFleet, /ws tick mesajının filo özetini (agent sayıları +
// bit/sn) taşıdığını doğrular — dağıtık kurulumda panoya "sıfır gecikme"
// canlı metrik sağlayan yol.
func TestWSTickCarriesFleet(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "ws.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})
	st.TouchAgent(1, "v", 1, "1.1.1.1")

	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer c.Close()

	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 4; i++ {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		var msg struct {
			Type  string `json:"type"`
			Fleet *struct {
				AgentsTotal  int `json:"agents_total"`
				AgentsOnline int `json:"agents_online"`
			} `json:"fleet"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.Type != "tick" {
			continue
		}
		if msg.Fleet == nil {
			continue // ilk tick önbellek dolmadan gelmiş olabilir
		}
		if msg.Fleet.AgentsTotal != 1 || msg.Fleet.AgentsOnline != 1 {
			t.Fatalf("fleet özeti hatalı: %+v", msg.Fleet)
		}
		return
	}
	t.Fatal("tick mesajında fleet özeti gelmedi")
}

// TestWSCheckOrigin, B5: origin izin listesi ayarlıyken yabancı origin'den
// gelen WS handshake reddedilir; same-origin / izinli origin kabul edilir.
func TestWSCheckOrigin(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "wso.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", "", 30, false, nil, nil, nil)
	srv.SetWSOrigins([]string{"https://ntms.example.com", "panel.internal"})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dial := func(origin string) (*http.Response, error) {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		c, resp, err := websocket.DefaultDialer.Dial(wsURL, h)
		if c != nil {
			c.Close()
		}
		return resp, err
	}

	// yabancı origin → 403
	if resp, err := dial("https://evil.example.org"); err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("yabancı origin reddedilmeliydi: err=%v resp=%v", err, resp)
	}
	// izinli origin → OK
	if _, err := dial("https://ntms.example.com"); err != nil {
		t.Fatalf("izinli origin kabul edilmeliydi: %v", err)
	}
	// port farklı ama host izinli → OK
	if _, err := dial("http://panel.internal:9000"); err != nil {
		t.Fatalf("host izinli (port fark etmez): %v", err)
	}
	// Origin başlığı yok (tarayıcı dışı) → OK
	if _, err := dial(""); err != nil {
		t.Fatalf("Origin'siz istek kabul edilmeliydi: %v", err)
	}
	// same-origin (httptest sunucusunun kendi adresi) → OK
	if _, err := dial(ts.URL); err != nil {
		t.Fatalf("same-origin kabul edilmeliydi: %v", err)
	}
}

// TestWSCheckOriginDefaultAllowsAll, izin listesi ayarlanmadıysa (nil) tüm
// origin'ler kabul edilir (bugünkü davranış, geriye uyum).
func TestWSCheckOriginDefaultAllowsAll(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "wsd.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	h := http.Header{}
	h.Set("Origin", "https://whatever.example.org")
	c, _, err := websocket.DefaultDialer.Dial(wsURL, h)
	if err != nil {
		t.Fatalf("varsayılan modda her origin kabul edilmeliydi: %v", err)
	}
	c.Close()
}
