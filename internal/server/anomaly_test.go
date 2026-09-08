package server

// S22.5: GET /api/v1/anomaly/{baseline,active} — panel uçları.

import (
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

func TestAnomalyEndpoints(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "anom.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	now := time.Now()
	bucket := store.SeasonalBucket(now, alert.DefaultAnomalyConfig().Seasonality)

	// filo bps baseline: mevcut kova, ort 8000 ± 200 (n bol)
	if err := st.SaveAnomalyBaseline([]store.AnomalyBaselineRow{
		{Dim: "fleet", Metric: "bps", Key: "", Bucket: bucket, N: 500, Mean: 8000, M2: 500 * 200 * 200},
	}); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	// mevcut pencerede ani yükseliş → agent arayüz telemetrisi ~1.6 Mbit/sn
	aid, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})
	var b uint64 = 10_000_000
	for i := 0; i < 8; i++ {
		if err := st.SaveIfaceSamples(aid, now.Unix()-int64((8-i)*30), []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: b},
		}); err != nil {
			t.Fatalf("iface: %v", err)
		}
		b += 6_000_000
	}

	// baseline ucu
	code, body := getJSON(t, ts, "/api/v1/anomaly/baseline?dim=fleet&metric=bps", "")
	if code != http.StatusOK {
		t.Fatalf("baseline: %d", code)
	}
	rows, _ := body["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("1 baseline satırı bekleniyordu: %+v", body)
	}

	// active ucu — sapma görünmeli
	code, body = getJSON(t, ts, "/api/v1/anomaly/active", "")
	if code != http.StatusOK {
		t.Fatalf("active: %d", code)
	}
	devs, _ := body["deviations"].([]any)
	if len(devs) == 0 {
		t.Fatalf("aktif sapma bekleniyordu: %+v", body)
	}
	d0 := devs[0].(map[string]any)
	if d0["dim"] != "fleet" || d0["metric"] != "bps" {
		t.Fatalf("sapma alanları: %+v", d0)
	}
	if z, _ := d0["z"].(float64); z < 3 {
		t.Fatalf("z >= 3 bekleniyordu, %v", z)
	}
}
