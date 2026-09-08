package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAggregateGeo(t *testing.T) {
	eps := []store.EndpointDelta{
		{IP: "8.8.8.8", BytesIn: 100, BytesOut: 50},
		{IP: "8.8.4.4", BytesIn: 200, BytesOut: 0},    // ayni ulke (US)
		{IP: "212.156.4.4", BytesIn: 10, BytesOut: 5}, // TR
		{IP: "10.0.0.1", BytesIn: 999, BytesOut: 999}, // ozel IP -> lookup bos
		{IP: "1.2.3.4", BytesIn: 7, BytesOut: 3},      // XX -> merkez yok, atlanir
	}
	lookup := func(ip string) string {
		switch ip {
		case "8.8.8.8", "8.8.4.4":
			return "US"
		case "212.156.4.4":
			return "TR"
		case "1.2.3.4":
			return "XX"
		}
		return ""
	}

	out := aggregateGeo(eps, lookup)
	if len(out) != 2 {
		t.Fatalf("2 ulke beklenirdi (US, TR), gelen %d: %+v", len(out), out)
	}
	// bytes'a gore azalan: US once
	if out[0].Country != "US" || out[0].Bytes != 350 || out[0].Sessions != 2 {
		t.Errorf("US yanlis: %+v", out[0])
	}
	if out[1].Country != "TR" || out[1].Bytes != 15 || out[1].Sessions != 1 {
		t.Errorf("TR yanlis: %+v", out[1])
	}
	if out[0].Name == "" || out[0].Lat == 0 {
		t.Errorf("US merkez koordinati doldurulmadi: %+v", out[0])
	}
}

func TestAggregateGeoNilLookup(t *testing.T) {
	eps := []store.EndpointDelta{{IP: "8.8.8.8", BytesIn: 1}}
	out := aggregateGeo(eps, func(string) string { return "" })
	if len(out) != 0 {
		t.Fatalf("GeoIP kaynagi yokken bos liste beklenirdi: %+v", out)
	}
}

// TestEnrichEndpoint, Faz 23-E: GET /api/v1/enrich — IP özel/genel + alan
// normalize/kayıtlı-alan. GeoIP kaynağı yok → ülke/ASN boş ama endpoint çalışır.
func TestEnrichEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	var out struct {
		IP struct {
			Private bool `json:"private"`
		} `json:"ip"`
		Domain struct {
			Normalized  string `json:"normalized"`
			Registrable string `json:"registrable"`
		} `json:"domain"`
	}
	r := apiReq(t, http.MethodGet, ts.URL+"/api/v1/enrich?ip=10.1.2.3&domain=API.Anthropic.com.", nil)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("200: %d", r.StatusCode)
	}
	json.NewDecoder(r.Body).Decode(&out)
	r.Body.Close()
	if !out.IP.Private {
		t.Fatalf("RFC1918 → private beklenirdi: %+v", out.IP)
	}
	if out.Domain.Normalized != "api.anthropic.com" || out.Domain.Registrable != "anthropic.com" {
		t.Fatalf("alan normalize/kayıtlı hatalı: %+v", out.Domain)
	}

	// ip ve domain yok → 400
	rbad := apiReq(t, http.MethodGet, ts.URL+"/api/v1/enrich", nil)
	rbad.Body.Close()
	if rbad.StatusCode != http.StatusBadRequest {
		t.Fatalf("boş sorgu 400 beklenirdi: %d", rbad.StatusCode)
	}
}
