package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPercentileInterpolation(t *testing.T) {
	var s stats
	// 1000 örnek: 970'i ~40ms, 30'u ~500ms → p50/p95 hızlı kümede,
	// p99 (990. örnek) yavaş kümede olmalı.
	for i := 0; i < 970; i++ {
		s.observe(40 * time.Millisecond)
	}
	for i := 0; i < 30; i++ {
		s.observe(500 * time.Millisecond)
	}
	p50 := s.percentile(0.50)
	p95 := s.percentile(0.95)
	p99 := s.percentile(0.99)
	if p50 < 20 || p50 > 50 {
		t.Errorf("p50 = %.1f, ~40 bekleniyordu", p50)
	}
	if p95 < 20 || p95 > 55 {
		t.Errorf("p95 = %.1f, hızlı kümede kalmalıydı", p95)
	}
	if p99 < 300 || p99 > 750 {
		t.Errorf("p99 = %.1f, yavaş kümede olmalıydı", p99)
	}
}

func TestPercentileEmpty(t *testing.T) {
	var s stats
	if s.percentile(0.95) != 0 {
		t.Fatal("boş histogramda 0 beklenir")
	}
}

func TestStatsReset(t *testing.T) {
	var s stats
	s.start = time.Now().Add(-time.Minute)
	s.sent.Store(500)
	s.observe(30 * time.Millisecond)
	s.flowRecords.Store(1000)
	s.reset()
	if s.sent.Load() != 0 || s.flowRecords.Load() != 0 || s.percentile(0.5) != 0 {
		t.Fatal("reset sayaçları sıfırlamadı")
	}
	if time.Since(s.start) > time.Second {
		t.Fatal("reset start'ı yenilemedi")
	}
}

func TestSummarizeAndWrite(t *testing.T) {
	var s stats
	s.start = time.Now().Add(-10 * time.Second)
	s.enrolled.Store(50)
	s.sent.Store(1700)
	s.fail(true)
	s.fail(false)
	for i := 0; i < 100; i++ {
		s.observe(45 * time.Millisecond)
	}
	s.flowDatagrams.Store(200)
	s.flowRecords.Store(4800)

	sm := s.summarize("mixed")
	if sm.Agent == nil || sm.Flow == nil {
		t.Fatal("agent + flow özeti bekleniyordu")
	}
	if sm.Agent.Sent != 1700 || sm.Agent.ErrDial != 1 || sm.Agent.ErrHTTP != 1 {
		t.Errorf("agent özeti hatalı: %+v", *sm.Agent)
	}
	if sm.Agent.RPS < 100 || sm.Agent.RPS > 250 {
		t.Errorf("rps ~170 bekleniyordu, %.1f", sm.Agent.RPS)
	}
	if sm.Flow.Records != 4800 {
		t.Errorf("flow records: %d", sm.Flow.Records)
	}

	p := filepath.Join(t.TempDir(), "sum.json")
	if err := writeSummary(p, sm); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	var back summary
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("özet JSON geçersiz: %v", err)
	}
	if back.Mode != "mixed" || back.Agent.Sent != 1700 {
		t.Errorf("round-trip hatalı: %+v", back)
	}
}
