package agent

// Hub veritabani sifirlaninca (ya da kayit elle silinince) kayitli agent
// token'i telemetri ucunda 401 doner. Agent bunu ErrUnauthorized olarak
// gormeli ve Reenroll ile enroll token'i kullanarak yeni bir kimlik almali —
// aksi halde veriyi sonsuza dek diske yigar (bkz. ana dongudeki authFails
// sayaci).

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestReenrollOn401(t *testing.T) {
	const goodToken = "yeni-token-xyz"
	helloCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agent/hello":
			helloCount++
			if r.Header.Get("X-Enroll-Token") != "enroll-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(telemetry.HubReply{
				Accepted: true, AgentID: 77, AgentToken: goodToken, TelemetryIntervalSeconds: 30,
			})
		case "/api/v1/agent/telemetry":
			if r.Header.Get("Authorization") != "Bearer "+goodToken {
				w.WriteHeader(http.StatusUnauthorized) // bayat token
				return
			}
			_ = json.NewEncoder(w).Encode(telemetry.TelemetryReply{OK: true, Interval: 30})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	stateFile := filepath.Join(t.TempDir(), "agent.state.json")
	c := New(Options{HubURL: srv.URL, EnrollToken: "enroll-secret", Name: "reenroll-test", StateFile: stateFile})

	// bayat kimlik: onceki hub'dan kalma, artik gecersiz
	stale := State{AgentID: 33, Token: "bayat-token"}
	if err := c.saveState(stale); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	// telemetri gonderimi bayat token'la 401 → ErrUnauthorized (jenerik HTTP
	// hatasi degil — cagiran taraf ayirt edebilmeli)
	if err := c.Send(stale, telemetry.TelemetryBatch{TS: 1}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ErrUnauthorized beklendi, gelen: %v", err)
	}

	// yeniden enroll: bayat state silinir, enroll token ile hello yapilir
	fresh, err := c.Reenroll()
	if err != nil {
		t.Fatalf("Reenroll: %v", err)
	}
	if fresh.AgentID != 77 || fresh.Token != goodToken {
		t.Fatalf("yeni state hatali: %+v", fresh)
	}
	if helloCount != 1 {
		t.Fatalf("hello tam 1 kez cagirilmali, gelen: %d", helloCount)
	}
	if got := c.LoadState(); got != fresh {
		t.Fatalf("disk state tazelenmemis: %+v (beklenen %+v)", got, fresh)
	}

	// yeni token'la telemetri (kuyrukta bekleyen bayat batch dahil) gecer
	if err := c.Send(fresh, telemetry.TelemetryBatch{TS: 2}); err != nil {
		t.Fatalf("yeni token'la telemetri: %v", err)
	}
}

func TestReenrollWithoutEnrollTokenFails(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "agent.state.json")
	c := New(Options{HubURL: "http://127.0.0.1:0", Name: "n", StateFile: stateFile})
	if err := c.saveState(State{AgentID: 1, Token: "t"}); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	if _, err := c.Reenroll(); err == nil {
		t.Fatal("enroll token yokken Reenroll hata dondurmeli")
	}
	// state korunur — koru bilgiyi kaybetme (elle mudahale sansi)
	if got := c.LoadState(); got.Token != "t" {
		t.Fatalf("enroll token yokken state silinmemeli, gelen: %+v", got)
	}
	_ = os.Remove(stateFile)
}
