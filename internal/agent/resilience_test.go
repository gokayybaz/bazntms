package agent

// S21.15: offline kuyruk taşması + kesinti sonrası replay.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestOfflineQueueOverflowAndReplay(t *testing.T) {
	var up atomic.Bool
	var posts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/hello" {
			json.NewEncoder(w).Encode(telemetry.HubReply{Accepted: true, AgentID: 7, AgentToken: "tok"})
			return
		}
		if !up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		posts.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "interval": 30})
	}))
	defer srv.Close()

	c := New(Options{
		HubURL: srv.URL, EnrollToken: "e", Name: "n",
		StateFile: filepath.Join(t.TempDir(), "agent.state.json"),
	})
	st, err := c.Enroll()
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// hub down: 150 gönderim → kuyruk 100'de sınırlanır, 50 eski atılır
	for i := 0; i < 150; i++ {
		_ = c.Send(st, telemetry.TelemetryBatch{TS: int64(1000 + i)})
	}
	if got := c.DroppedBatches(); got != 50 {
		t.Fatalf("atılan batch = %d, beklenen 50", got)
	}
	if q := len(c.loadQueue()); q != maxQueuedBatches {
		t.Fatalf("kuyruk uzunluğu = %d, beklenen %d", q, maxQueuedBatches)
	}

	// hub geri geldi: tek Send tüm kuyruğu + yeni batch'i boşaltır
	up.Store(true)
	posts.Store(0)
	if err := c.Send(st, telemetry.TelemetryBatch{TS: 9999}); err != nil {
		t.Fatalf("recover Send: %v", err)
	}
	if got := posts.Load(); got != int64(maxQueuedBatches)+1 {
		t.Fatalf("replay POST sayısı = %d, beklenen %d", got, maxQueuedBatches+1)
	}
	if q := len(c.loadQueue()); q != 0 {
		t.Fatalf("replay sonrası kuyruk boş olmalı, %d kaldı", q)
	}

	// en yeni batch korunmuş olmalı (en eskiler atıldı)
	// 150 gönderimde TS 1000..1149; kuyrukta son 100 (1050..1149) + replay'de 9999
}
