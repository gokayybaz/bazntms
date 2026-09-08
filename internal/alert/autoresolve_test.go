package alert

// S22.8: otomatik çözülme — TTL (yinelenmeyen olay) + anomali koşul-tabanlı.

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAutoResolveTTL(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "ar.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine(), 30)

	old := time.Now().Add(-20 * time.Minute).Unix()
	id, _ := st.InsertAlertEvent(store.AlertEvent{Ts: old, Kind: "bw", Key: "agent-in:x", Message: "yüksek", FirstTs: old, LastTs: old})
	fresh := time.Now().Unix()
	st.InsertAlertEvent(store.AlertEvent{Ts: fresh, Kind: "bw", Key: "agent-in:y", Message: "yüksek", FirstTs: fresh, LastTs: fresh})

	cfg := DefaultConfig() // AutoResolveMin 15
	m.sweepAutoResolve(cfg)

	if o, _ := st.OpenAlertEventByKey("bw", "agent-in:x"); o != nil {
		t.Fatalf("eski olay çözülmeliydi: %+v", o)
	}
	if o, _ := st.OpenAlertEventByKey("bw", "agent-in:y"); o == nil {
		t.Fatalf("taze olay açık kalmalıydı")
	}
	for _, e := range m.RecentEvents(10) {
		if e.ID == id && (e.State != "resolved" || e.ResolvedTs == 0) {
			t.Fatalf("çözülme alanları: %+v", e)
		}
	}

	// AutoResolveMin < 0 → kapalı
	old2 := time.Now().Add(-time.Hour).Unix()
	st.InsertAlertEvent(store.AlertEvent{Ts: old2, Kind: "bw", Key: "agent-in:z", Message: "x", FirstTs: old2, LastTs: old2})
	cfg.AutoResolveMin = -1
	m.sweepAutoResolve(cfg)
	if o, _ := st.OpenAlertEventByKey("bw", "agent-in:z"); o == nil {
		t.Fatalf("AutoResolveMin<0 iken çözülme olmamalı")
	}
}

func TestAnomalyConditionResolve(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	curBucket := store.SeasonalBucket(time.Now(), "hourly")

	// önceden ateşlenmiş gibi açık bir anomali olayı — mevcut kova dilimi
	key := fmt.Sprintf("bps:fleet::%d", curBucket)
	st.InsertAlertEvent(store.AlertEvent{Ts: time.Now().Unix(), Kind: "anomaly", Key: key, Message: "eski sapma", State: "firing"})
	// farklı kova dilimi — dokunulmamalı
	otherKey := fmt.Sprintf("bps:fleet::%d", (curBucket+13)%24)
	st.InsertAlertEvent(store.AlertEvent{Ts: time.Now().Unix(), Kind: "anomaly", Key: otherKey, Message: "başka dilim", State: "firing"})

	// hiç telemetri yok → aday yok → mevcut kovanın açık olayı koşul-çözülür
	m.checkAnomaly(cfg)

	if o, _ := st.OpenAlertEventByKey("anomaly", key); o != nil {
		t.Fatalf("aday olmayan açık anomali (bu kova) çözülmeliydi: %+v", o)
	}
	if o, _ := st.OpenAlertEventByKey("anomaly", otherKey); o == nil {
		t.Fatalf("başka kova dilimi olayı dokunulmamalı")
	}
}
