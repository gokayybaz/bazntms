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

	// ana dongu bu noktada diskteki 401 sayacini isler; reenroll onu sifirlamali
	c.NoteAuthFailure()
	c.NoteAuthFailure()
	if got := c.AuthFailStreak(); got != 2 {
		t.Fatalf("401 sayaci 2 olmali, gelen: %d", got)
	}

	// yeniden enroll: bayat state silinir, enroll token ile hello yapilir
	fresh, err := c.Reenroll()
	if err != nil {
		t.Fatalf("Reenroll: %v", err)
	}
	if fresh.AgentID != 77 || fresh.Token != goodToken {
		t.Fatalf("yeni state hatali: %+v", fresh)
	}
	if got := c.AuthFailStreak(); got != 0 {
		t.Fatalf("reenroll sonrasi 401 sayaci sifirlanmali, gelen: %d", got)
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

// Ardisik 401 sayaci state dosyasinda yasamali: bellekte tutulunca token'i
// olmus ama sik yeniden baslayan agent (crash-loop / launchd KeepAlive /
// tekrarli kurulum) her restart'ta 0'a donuyor ve reenroll esigine hic
// ulasamiyordu.
func TestAuthFailStreakPersistsAcrossRestart(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "agent.state.json")

	// 1. oturum: bayat kimlik + iki 401
	c1 := New(Options{HubURL: "http://127.0.0.1:0", Name: "n", StateFile: stateFile})
	if err := c1.saveState(State{AgentID: 5, Token: "bayat"}); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	if n := c1.NoteAuthFailure(); n != 1 {
		t.Fatalf("ilk 401 → 1, gelen %d", n)
	}
	if n := c1.NoteAuthFailure(); n != 2 {
		t.Fatalf("ikinci 401 → 2, gelen %d", n)
	}

	// 2. oturum: yeni Client, ayni dosya — sayac korunmali, kimlik bozulmamali
	c2 := New(Options{HubURL: "http://127.0.0.1:0", Name: "n", StateFile: stateFile})
	if got := c2.AuthFailStreak(); got != 2 {
		t.Fatalf("restart sonrasi sayac 2 olmali, gelen %d", got)
	}
	if st := c2.LoadState(); st.AgentID != 5 || st.Token != "bayat" {
		t.Fatalf("kimlik alanlari bozuldu: %+v", st)
	}
	if n := c2.NoteAuthFailure(); n != 3 {
		t.Fatalf("ucuncu 401 → 3 (esik), gelen %d", n)
	}

	// telemetri basarili → sayac sifir, kimlik yerinde
	c2.ClearAuthFailure()
	if got := c2.AuthFailStreak(); got != 0 {
		t.Fatalf("ClearAuthFailure sonrasi 0 olmali, gelen %d", got)
	}
	if st := c2.LoadState(); st.AgentID != 5 || st.Token != "bayat" {
		t.Fatalf("Clear kimligi bozdu: %+v", st)
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
