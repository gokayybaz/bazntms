package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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

// TestProcessDetailEndpoint, Faz 23-A: GET /api/v1/agents/{id}/processes/{ad}
// süreç kimliği + uzak hedefler + canlı bağlantılar + uygulama görünürlüğü +
// zaman çizelgesini tek yanıtta döndürür; DNS/SNI verisi yokken de çalışır;
// görülmeyen süreç 404.
func TestProcessDetailEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "pd.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	id, err := st.RegisterAgent(store.Agent{Name: "pd-agent", TokenHash: store.TokenHash("pd")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	now := time.Now().Unix()
	if err := st.SaveProcessTraffic(id, now-30, []telemetry.ProcessTrafficSample{
		{PID: 5, Process: "claude", Proto: "tcp", RemoteIP: "1.2.3.4", Port: 443, BytesIn: 800, BytesOut: 120},
	}); err != nil {
		t.Fatalf("pt1: %v", err)
	}
	if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
		{PID: 5, Process: "claude", Proto: "tcp", RemoteIP: "1.2.3.4", Port: 443, BytesIn: 2000, BytesOut: 200},
	}); err != nil {
		t.Fatalf("pt2: %v", err)
	}
	if err := st.SaveAgentDNS(id, now, []telemetry.DNSSample{
		{PID: 5, Process: "claude", Domain: "api.anthropic.com", Queries: 3, Responses: 3},
	}); err != nil {
		t.Fatalf("dns: %v", err)
	}
	if err := st.ReplaceConnLatest(id, []telemetry.ConnectionSample{
		{Proto: "tcp", LocalAddr: "10.0.0.9:51000", RemoteAddr: "1.2.3.4:443", Status: "ESTABLISHED", Process: "claude"},
		{Proto: "tcp", LocalAddr: "10.0.0.9:22", RemoteAddr: "9.9.9.9:5000", Status: "ESTABLISHED", Process: "sshd"},
	}); err != nil {
		t.Fatalf("conn: %v", err)
	}

	resp := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents/"+fmt.Sprint(id)+"/processes/claude", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("200 beklenirdi: %d", resp.StatusCode)
	}
	var out struct {
		Process struct {
			Process string  `json:"process"`
			Total   uint64  `json:"total"`
			PIDs    []int64 `json:"pids"`
			RxBps   float64 `json:"rx_bps"`
		} `json:"process"`
		Remotes []struct {
			RemoteIP string `json:"remote_ip"`
			Conns    int    `json:"conns"`
		} `json:"remotes"`
		Connections   []telemetry.ConnectionSample `json:"connections"`
		AppVisibility []struct {
			Type string `json:"type"`
			Host string `json:"host"`
		} `json:"app_visibility"`
		Timeline []struct {
			Event  string `json:"event"`
			Target string `json:"target"`
		} `json:"timeline"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("çözülemedi: %v", err)
	}
	// kova1 = 800+120, kova2 = 2000+200
	if out.Process.Process != "claude" || out.Process.Total != 3120 {
		t.Fatalf("özet hatalı: %+v", out.Process)
	}
	// anlık hız = son kova (2000 bayt in) / 30 sn
	if out.Process.RxBps < 65 || out.Process.RxBps > 68 {
		t.Fatalf("rx_bps ~66.7 beklenirdi: %v", out.Process.RxBps)
	}
	// süreç trafik tablosuyla mutabık (aynı store sorgusu)
	top, _ := st.TopProcessTraffic(time.Now().Add(-time.Hour), id, 10, "")
	if len(top) != 1 || top[0].Total != out.Process.Total {
		t.Fatalf("total TopProcessTraffic ile mutabık değil: %+v vs %d", top, out.Process.Total)
	}
	if len(out.Remotes) != 1 || out.Remotes[0].RemoteIP != "1.2.3.4" || out.Remotes[0].Conns != 1 {
		t.Fatalf("uzak uç / canlı bağlantı hatalı: %+v", out.Remotes)
	}
	if len(out.Connections) != 1 || out.Connections[0].Process != "claude" {
		t.Fatalf("bağlantılar sürece kapsamlı değil: %+v", out.Connections)
	}
	if len(out.AppVisibility) != 1 || out.AppVisibility[0].Host != "api.anthropic.com" {
		t.Fatalf("uygulama görünürlüğü hatalı: %+v", out.AppVisibility)
	}
	if len(out.Timeline) == 0 || out.Timeline[0].Event != "process.first_seen" {
		t.Fatalf("zaman çizelgesi hatalı: %+v", out.Timeline)
	}

	// görülmeyen süreç → 404
	r404 := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents/"+fmt.Sprint(id)+"/processes/yok", nil)
	r404.Body.Close()
	if r404.StatusCode != http.StatusNotFound {
		t.Fatalf("görülmeyen süreç 404 beklenirdi: %d", r404.StatusCode)
	}

	// DNS/SNI olmadan da 200 (yalnız trafik olan süreç)
	if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
		{PID: 7, Process: "raw", Proto: "udp", RemoteIP: "5.5.5.5", Port: 1234, BytesIn: 10, BytesOut: 10},
	}); err != nil {
		t.Fatalf("pt raw: %v", err)
	}
	rRaw := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents/"+fmt.Sprint(id)+"/processes/raw", nil)
	defer rRaw.Body.Close()
	if rRaw.StatusCode != http.StatusOK {
		t.Fatalf("DNS/SNI'siz süreç 200 beklenirdi: %d", rRaw.StatusCode)
	}
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

// TestAgentHelloThreadsMachineID, C3 (Faz 13): handleAgentHello, hello.MachineID
// alanini RegisterOrReuseAgent'e gecirir. Cevrimici bir eslesme yeni satir
// acar (cevrimdisi -> yeniden kullanim yolu store birim testinde:
// store.TestRegisterOrReuseAgent). Burada machine_id'nin store'a "srv-01"
// satirinda yazildigini ve ikinci hello'nun (agent hala cevrimici) yeni satir
// actigini dogruluyoruz.
func TestAgentHelloThreadsMachineID(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "reuse.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	hello := func(machineID string) int64 {
		t.Helper()
		body, _ := json.Marshal(telemetry.AgentHello{Name: "srv-01", Site: "dc1", MachineID: machineID, ProtocolVersion: 1})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/hello", bytes.NewReader(body))
		req.Header.Set("X-Enroll-Token", testEnrollToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("hello: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("hello durumu: %d", resp.StatusCode)
		}
		var out struct {
			AgentID int64 `json:"agent_id"`
		}
		json.NewDecoder(resp.Body).Decode(&out)
		return out.AgentID
	}

	id1 := hello("mach-xyz")
	id2 := hello("mach-xyz") // agent hala cevrimici → yeni satir
	if id1 == id2 {
		t.Fatalf("cevrimici eslesmede yeni satir bekleniyordu: id1=%d id2=%d", id1, id2)
	}
	agents, _ := st.ListAgents(0, "")
	if len(agents) != 2 {
		t.Fatalf("iki agent satiri bekleniyordu, %d var", len(agents))
	}
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

// TestTelemetryUpdatesAgentVersion, kayitli agent hello'yu atladigi icin
// surumun her telemetri batch'inden okundugunu dogrular — agent guncellenince
// (reinstall/self-update) hub'in gosterdigi surum aksi halde donuk kalirdi.
func TestTelemetryUpdatesAgentVersion(t *testing.T) {
	ts := newTestServerWithEnroll(t)
	id, token := enrollAgent(t, ts, "agent-upgrade")

	send := func(ver string) {
		body, _ := json.Marshal(map[string]any{
			"version":          ver,
			"protocol_version": 1,
			"interfaces":       []map[string]any{{"name": "eth0", "rx_bytes": 1, "tx_bytes": 1}},
		})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/telemetry", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("telemetri: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("telemetri 200 beklenirdi: %d", resp.StatusCode)
		}
	}
	agentVersion := func() string {
		resp := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents", nil)
		defer resp.Body.Close()
		var list []struct {
			ID      int64  `json:"id"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("liste cozulemedi: %v", err)
		}
		for _, a := range list {
			if a.ID == id {
				return a.Version
			}
		}
		t.Fatalf("agent %d listede yok", id)
		return ""
	}

	send("0.2.0")
	if got := agentVersion(); got != "0.2.0" {
		t.Fatalf("ilk surum 0.2.0 beklenirdi, gelen: %q", got)
	}
	send("0.3.0") // agent guncellendi
	if got := agentVersion(); got != "0.3.0" {
		t.Fatalf("guncelleme sonrasi 0.3.0 beklenirdi, gelen: %q", got)
	}
	// surum tasimayan eski agent mevcut degeri silmemeli
	send("")
	if got := agentVersion(); got != "0.3.0" {
		t.Fatalf("bos surum mevcut degeri korumaliydi, gelen: %q", got)
	}
}

// TestTelemetryReportsAttrMethod, agent'in batch'te bildirdigi süreç-atıf
// arka ucunun (Faz 20) hub'a ulaşıp /api/v1/agents yanıtında göründüğünü
// doğrular. Boş = eski agent → mevcut değer korunur.
func TestTelemetryReportsAttrMethod(t *testing.T) {
	ts := newTestServerWithEnroll(t)
	id, token := enrollAgent(t, ts, "agent-attr")

	send := func(method string) {
		b := map[string]any{"interfaces": []map[string]any{{"name": "eth0"}}}
		if method != "" {
			b["attr_method"] = method
		}
		body, _ := json.Marshal(b)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/telemetry", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("telemetri: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("telemetri 200 beklenirdi: %d", resp.StatusCode)
		}
	}
	method := func() string {
		resp := apiReq(t, http.MethodGet, ts.URL+"/api/v1/agents", nil)
		defer resp.Body.Close()
		var list []struct {
			ID         int64  `json:"id"`
			AttrMethod string `json:"attr_method"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("liste: %v", err)
		}
		for _, a := range list {
			if a.ID == id {
				return a.AttrMethod
			}
		}
		t.Fatalf("agent %d yok", id)
		return ""
	}

	send("ebpf")
	if got := method(); got != "ebpf" {
		t.Fatalf("attr_method ebpf beklenirdi: %q", got)
	}
	send("") // eski agent → koru
	if got := method(); got != "ebpf" {
		t.Fatalf("boş attr_method mevcut değeri korumalıydı: %q", got)
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
