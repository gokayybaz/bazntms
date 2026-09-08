package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

// TestFlowConversationsEndpoint, Faz 23-B: GET /api/v1/flows/conversations
// ham NetFlow'u toplar; ?by=5tuple|pair; drill-down ham akışları döndürür.
func TestFlowConversationsEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "fc.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	now := time.Now().Unix()
	if err := st.SaveFlows([]store.FlowRow{
		{Ts: now - 60, Device: "fw1", Src: "10.0.0.1", Dst: "8.8.8.8", SrcPort: 5000, DstPort: 443, Proto: "tcp", Packets: 10, Octets: 2000},
		{Ts: now - 30, Device: "fw1", Src: "8.8.8.8", Dst: "10.0.0.1", SrcPort: 443, DstPort: 5000, Proto: "tcp", Packets: 8, Octets: 6000},
		{Ts: now - 10, Device: "fw1", Src: "10.0.0.2", Dst: "1.1.1.1", SrcPort: 5001, DstPort: 53, Proto: "udp", Packets: 2, Octets: 120},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}

	var pair []struct {
		Src    string `json:"src"`
		Dst    string `json:"dst"`
		Flows  uint64 `json:"flows"`
		Octets uint64 `json:"octets"`
	}
	r := apiReq(t, http.MethodGet, ts.URL+"/api/v1/flows/conversations?window=1h&by=pair&sort=octets", nil)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("200 beklenirdi: %d", r.StatusCode)
	}
	json.NewDecoder(r.Body).Decode(&pair)
	r.Body.Close()
	if len(pair) != 2 {
		t.Fatalf("2 uç-çifti: %+v", pair)
	}
	if pair[0].Src != "10.0.0.1" || pair[0].Dst != "8.8.8.8" || pair[0].Flows != 2 || pair[0].Octets != 8000 {
		t.Fatalf("en yoğun çift hatalı: %+v", pair[0])
	}

	// 5'li → yön ayrı
	var five []map[string]any
	r5 := apiReq(t, http.MethodGet, ts.URL+"/api/v1/flows/conversations?window=1h&by=5tuple", nil)
	json.NewDecoder(r5.Body).Decode(&five)
	r5.Body.Close()
	if len(five) != 3 {
		t.Fatalf("3 benzersiz 5'li: %+v", five)
	}

	// drill-down: çift yönlü ham akışlar
	var det struct {
		Flows []map[string]any `json:"flows"`
	}
	rd := apiReq(t, http.MethodGet, ts.URL+"/api/v1/flows/conversation?window=1h&src=10.0.0.1&dst=8.8.8.8", nil)
	if rd.StatusCode != http.StatusOK {
		t.Fatalf("drill 200 beklenirdi: %d", rd.StatusCode)
	}
	json.NewDecoder(rd.Body).Decode(&det)
	rd.Body.Close()
	if len(det.Flows) != 2 {
		t.Fatalf("çift yönlü 2 ham akış: %+v", det.Flows)
	}

	// src/dst eksik → 400
	rbad := apiReq(t, http.MethodGet, ts.URL+"/api/v1/flows/conversation?src=10.0.0.1", nil)
	rbad.Body.Close()
	if rbad.StatusCode != http.StatusBadRequest {
		t.Fatalf("eksik dst 400 beklenirdi: %d", rbad.StatusCode)
	}
}
