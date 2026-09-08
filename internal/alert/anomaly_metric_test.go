package alert

// S22.4: bps dışı metrikler — DNS sorgu hızı anomalisi (tünelleme/DGA/exfil
// erken sinyali). Baseline ~2 sorgu/sn iken pencerede ~40 sorgu/sn → uyarı.

import (
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func seedDNS(t *testing.T, st store.Store, agentID, startTs int64, n int, stepSecs int64, qBase uint64) {
	t.Helper()
	for i := 0; i < n; i++ {
		q := qBase + uint64(i%7)*5 // baseline'ın std'si > 0 olsun
		if err := st.SaveAgentDNS(agentID, startTs+int64(i)*stepSecs, []telemetry.DNSSample{
			{Process: "curl", Domain: "example.com", Queries: q, Responses: q},
		}); err != nil {
			t.Fatalf("dns ornek: %v", err)
		}
	}
}

func TestAnomalyDNSQpsMetric(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()
	aid, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})

	// baseline: geçmiş 2 gün aynı saat, dakikada ~120 sorgu (~2 sorgu/sn)
	for day := 2; day >= 1; day-- {
		hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 1, 0, 0, now.Location())
		seedDNS(t, st, aid, hs.Unix(), 45, 60, 120)
	}
	// mevcut pencere: dakikada ~2400 sorgu (~40 sorgu/sn) — baseline'in ~20x
	seedDNS(t, st, aid, now.Unix()-240, 5, 60, 2400)

	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	cfg.Anomaly.MinSamples = 20
	cfg.Anomaly.Metrics = []string{"dns_qps"}
	m.rebuildAnomalyBaseline(cfg)
	m.checkAnomaly(cfg)

	var dnsAnoms []store.AlertEvent
	for _, e := range m.RecentEvents(20) {
		if e.Kind == "anomaly" && strings.HasPrefix(e.Key, "dns_qps:") {
			dnsAnoms = append(dnsAnoms, e)
		}
	}
	if len(dnsAnoms) == 0 {
		t.Fatalf("DNS sorgu hızı anomalisi bekleniyordu, üretilmedi: %+v", m.RecentEvents(20))
	}
	if !strings.Contains(dnsAnoms[0].Message, "DNS sorgu hızı") || !strings.Contains(dnsAnoms[0].Message, "sorgu/sn") {
		t.Fatalf("mesaj metrik etiketi/birimini taşımalı: %q", dnsAnoms[0].Message)
	}
}

func seedL7(t *testing.T, st store.Store, agentID, startTs int64, n int, stepSecs int64, hitsBase uint64) {
	t.Helper()
	for i := 0; i < n; i++ {
		h := hitsBase + uint64(i%7)*5
		if err := st.SaveL7(agentID, startTs+int64(i)*stepSecs, []telemetry.L7Sample{
			{Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.2.3.4", Bytes: 1000, Count: h},
		}); err != nil {
			t.Fatalf("l7 ornek: %v", err)
		}
	}
}

// TestAnomalyL7QpsMetric, Faz 24-D: L7 (TLS SNI / HTTP Host) gözlem hızı
// anomalisi — endpoint-temas sıçraması ≈ alışılmadık / yeni hedef aktivitesi.
func TestAnomalyL7QpsMetric(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()
	aid, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})

	for day := 2; day >= 1; day-- {
		hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 1, 0, 0, now.Location())
		seedL7(t, st, aid, hs.Unix(), 45, 60, 120) // ~2 gözlem/sn
	}
	seedL7(t, st, aid, now.Unix()-240, 5, 60, 2400) // ~40 gözlem/sn

	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	cfg.Anomaly.MinSamples = 20
	cfg.Anomaly.Metrics = []string{"l7_qps"}
	m.rebuildAnomalyBaseline(cfg)
	m.checkAnomaly(cfg)

	var anoms []store.AlertEvent
	for _, e := range m.RecentEvents(20) {
		if e.Kind == "anomaly" && strings.HasPrefix(e.Key, "l7_qps:") {
			anoms = append(anoms, e)
		}
	}
	if len(anoms) == 0 {
		t.Fatalf("L7 gözlem hızı anomalisi bekleniyordu: %+v", m.RecentEvents(20))
	}
	if !strings.Contains(anoms[0].Message, "L7 endpoint teması") || !strings.Contains(anoms[0].Message, "gözlem/sn") {
		t.Fatalf("mesaj metrik etiketi/birimini taşımalı: %q", anoms[0].Message)
	}
}
