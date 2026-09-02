package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
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
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// TestTelemetryReplyCarriesPolicy, kayitli agent enrollment'i tekrarlamadigi
// (hello'yu atladigi) icin hub politikasinin — telemetri araligi + PCAP izni —
// her telemetri yanitiyla agent'a iletildigini dogrular. Bu olmadan agent
// restart sonrasi PCAP iznini kaybediyordu (surec trafigi tablosu bosaliyordu).
func TestTelemetryReplyCarriesPolicy(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "agents.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 45, true, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	_, token := enrollAgent(t, ts, "pcap-agent")
	resp := sendTelemetry(t, ts, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("telemetri 200 donmeliydi, gelen: %d", resp.StatusCode)
	}
	var reply telemetry.TelemetryReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		t.Fatalf("yanit cozulemedi: %v", err)
	}
	if reply.Interval != 45 {
		t.Errorf("interval=45 beklenirdi, gelen: %d", reply.Interval)
	}
	if reply.PCAPEnabled == nil || !*reply.PCAPEnabled {
		t.Errorf("pcap_enabled=true beklenirdi, gelen: %v", reply.PCAPEnabled)
	}
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

// --- filo yonetimi (list/detail/history/rename/delete) ---

// enrollAgent, testEnrollToken ile bir agent kaydeder ve donen agent_id +
// agent_token'i dondurur.
func enrollAgent(t *testing.T, ts *httptest.Server, name string) (int64, string) {
	t.Helper()
	resp := helloReq(t, ts, testEnrollToken, name)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enrollment basarisiz: %d", resp.StatusCode)
	}
	var out struct {
		AgentID    int64  `json:"agent_id"`
		AgentToken string `json:"agent_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("yanit cozulemedi: %v", err)
	}
	return out.AgentID, out.AgentToken
}

// sendTelemetry, agent adina ornek bir telemetri batch'i gonderir (agent'i
// "online" durumuna getirir — handleAgentsList/Detail bunu kullanir).
func sendTelemetry(t *testing.T, ts *httptest.Server, agentToken string) *http.Response {
	t.Helper()
	batch := map[string]any{
		"interfaces": []map[string]any{
			{"name": "eth0", "rx_bytes": 1000, "tx_bytes": 500, "rx_packets": 10, "tx_packets": 5},
		},
		"connections": []map[string]any{
			{"proto": "tcp", "local_addr": "10.0.0.1:5000", "remote_addr": "1.2.3.4:443", "status": "ESTABLISHED"},
		},
	}
	body, _ := json.Marshal(batch)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/telemetry", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("istek olusturulamadi: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("telemetri istegi basarisiz: %v", err)
	}
	return resp
}

func apiReq(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("istek olusturulamadi: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("istek basarisiz: %v", err)
	}
	return resp
}

// TestAgentTelemetryRequiresToken, Bearer agent token'i olmadan/gecersizken
// telemetri ucunun 401 dondurdugunu dogrular.
func TestAgentTelemetryRequiresToken(t *testing.T) {
	ts := newTestServerWithEnroll(t)
	resp := sendTelemetry(t, ts, "gecersiz-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("401 beklenirdi, gelen: %d", resp.StatusCode)
	}
}

// TestAgentLifecycle, enroll → telemetri → list → detail → history → rename
// → delete akisinin ucdan uca dogru calistigini dogrular (Faz 8 UI'daki
// agent yonetimi butonlarinin arkasindaki tam yol).
func TestAgentLifecycle(t *testing.T) {
	ts := newTestServerWithEnroll(t)
	id, token := enrollAgent(t, ts, "agent-lifecycle")

	tResp := sendTelemetry(t, ts, token)
	tResp.Body.Close()
	if tResp.StatusCode != http.StatusOK {
		t.Fatalf("telemetri 200 beklenirdi, gelen: %d", tResp.StatusCode)
	}

	// list
	listResp := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents", nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("liste 200 beklenirdi, gelen: %d", listResp.StatusCode)
	}
	var list []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Online bool   `json:"online"`
		Conns  int    `json:"conns"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("liste cozulemedi: %v", err)
	}
	found := false
	for _, a := range list {
		if a.ID == id {
			found = true
			if !a.Online {
				t.Error("telemetri gonderilmis agent online gorunmeliydi")
			}
		}
	}
	if !found {
		t.Fatalf("agent %d listede bulunamadi: %+v", id, list)
	}

	// detail
	detailResp := apiReq(t, http.MethodGet, fmt.Sprintf("%s/api/v1/agents/%d", ts.URL, id), nil)
	defer detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("detay 200 beklenirdi, gelen: %d", detailResp.StatusCode)
	}
	var detail struct {
		Agent struct {
			Name string `json:"name"`
		} `json:"agent"`
		Connections []map[string]any `json:"connections"`
	}
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("detay cozulemedi: %v", err)
	}
	if detail.Agent.Name != "agent-lifecycle" {
		t.Fatalf("beklenen isim agent-lifecycle, gelen: %s", detail.Agent.Name)
	}
	if len(detail.Connections) != 1 {
		t.Fatalf("1 baglanti beklenirdi, gelen: %d", len(detail.Connections))
	}

	// history
	histResp := apiReq(t, http.MethodGet, fmt.Sprintf("%s/api/v1/agents/%d/history?minutes=60", ts.URL, id), nil)
	defer histResp.Body.Close()
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("history 200 beklenirdi, gelen: %d", histResp.StatusCode)
	}

	// rename
	renResp := apiReq(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/agents/%d", ts.URL, id), map[string]string{"name": "yeni-ad"})
	defer renResp.Body.Close()
	if renResp.StatusCode != http.StatusOK {
		t.Fatalf("rename 200 beklenirdi, gelen: %d", renResp.StatusCode)
	}
	detail2Resp := apiReq(t, http.MethodGet, fmt.Sprintf("%s/api/v1/agents/%d", ts.URL, id), nil)
	defer detail2Resp.Body.Close()
	var detail2 struct {
		Agent struct {
			Name string `json:"name"`
		} `json:"agent"`
	}
	json.NewDecoder(detail2Resp.Body).Decode(&detail2)
	if detail2.Agent.Name != "yeni-ad" {
		t.Fatalf("rename sonrasi isim yeni-ad olmali, gelen: %s", detail2.Agent.Name)
	}

	// rename: bos isim reddedilmeli
	badRenResp := apiReq(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/agents/%d", ts.URL, id), map[string]string{"name": "  "})
	defer badRenResp.Body.Close()
	if badRenResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bos isim icin 400 beklenirdi, gelen: %d", badRenResp.StatusCode)
	}

	// delete
	delResp := apiReq(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/agents/%d", ts.URL, id), nil)
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete 200 beklenirdi, gelen: %d", delResp.StatusCode)
	}

	// delete sonrasi telemetri artik kabul edilmemeli (token gecersiz)
	postDelResp := sendTelemetry(t, ts, token)
	defer postDelResp.Body.Close()
	if postDelResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("silinen agent'in token'i artik gecersiz olmali, gelen: %d", postDelResp.StatusCode)
	}
}
