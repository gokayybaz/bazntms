package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

const testEnrollToken = "test-enroll-token"

func newTestServerWithEnroll(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agents.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", ai.NewClient(ai.Config{}), mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func helloReq(t *testing.T, ts *httptest.Server, enrollToken, name string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name, "site": "test"})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/hello", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("istek olusturulamadi: %v", err)
	}
	req.Header.Set("X-Enroll-Token", enrollToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("istek basarisiz: %v", err)
	}
	return resp
}

// TestAgentHelloWrongToken, gecersiz enroll token'in 401 dondurdugunu dogrular
// (rate-limit esigine ulasmadan once).
func TestAgentHelloWrongToken(t *testing.T) {
	ts := newTestServerWithEnroll(t)
	resp := helloReq(t, ts, "yanlis-token", "agent1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("401 beklenirdi, gelen: %d", resp.StatusCode)
	}
}

// TestAgentHelloRateLimit, ard arda yanlis enroll token denemelerinin
// (maxAttempts esiginden sonra) 429 ile bloklandigini, DOGRU token'in bile
// blok suresince kabul edilmedigini dogrular — auth.go'daki login
// rate-limit'iyle ayni davranis.
func TestAgentHelloRateLimit(t *testing.T) {
	ts := newTestServerWithEnroll(t)

	for i := 0; i < maxAttempts; i++ {
		resp := helloReq(t, ts, "yanlis-token", "agent1")
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("deneme %d: 401 beklenirdi, gelen: %d", i, resp.StatusCode)
		}
	}

	// esik asildi: bir sonraki deneme (dogru token olsa bile) bloklanmali
	resp := helloReq(t, ts, testEnrollToken, "agent1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("429 beklenirdi, gelen: %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("Retry-After basligi eksik")
	}
}

// TestAgentHelloSuccessResetsAttempts, basarili bir enrollment sonrasi o
// IP'nin deneme sayacinin sifirlandigini (esik asilmis gibi hemen
// bloklanmadigini) dogrular.
func TestAgentHelloSuccessResetsAttempts(t *testing.T) {
	ts := newTestServerWithEnroll(t)

	for i := 0; i < maxAttempts-1; i++ {
		resp := helloReq(t, ts, "yanlis-token", "agent1")
		resp.Body.Close()
	}

	resp := helloReq(t, ts, testEnrollToken, "agent1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esik altinda basarili enrollment 200 donmeliydi, gelen: %d", resp.StatusCode)
	}

	// basarili denemeden sonra sayac sifirlanmis olmali — hemen ardindan
	// gelen bir yanlis deneme bloklanmamali (401, 429 degil)
	resp2 := helloReq(t, ts, "yanlis-token", "agent2")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("basari sonrasi sayac sifirlanmali, 401 beklenirdi ama gelen: %d", resp2.StatusCode)
	}
}
