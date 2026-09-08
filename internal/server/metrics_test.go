package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestMetricsEndpointMergesRegistries, /metrics çıktısının hem server'ın
// instance-başına registry'sini (bazntms_http_*) hem de paketler-arası ingest
// registry'sini (internal/metrics — bazntms_store_write_*) içerdiğini
// doğrular. Bir agent enroll edilip telemetri gönderilir; telemetri yolu
// toplu depo yazımlarını tetikler.
func TestMetricsEndpointMergesRegistries(t *testing.T) {
	ts := newTestServerWithEnroll(t)

	_, token := enrollAgent(t, ts, "metrics-agent")
	resp := sendTelemetry(t, ts, token)
	resp.Body.Close()

	mResp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("/metrics isteği: %v", err)
	}
	defer mResp.Body.Close()
	body, _ := io.ReadAll(mResp.Body)
	out := string(body)

	for _, want := range []string{
		"bazntms_http_requests_total",                                 // server registry
		`bazntms_store_write_rows_total{table="agent_iface_samples"}`, // ingest registry
		"bazntms_telemetry_decode_duration_seconds",
		"bazntms_devpoll_inflight",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics çıktısında eksik: %q", want)
		}
	}
}
