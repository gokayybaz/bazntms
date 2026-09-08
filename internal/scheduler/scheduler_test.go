package scheduler

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestNextRun(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, loc) // Salı 10:00

	cases := []struct {
		spec string
		want time.Time
	}{
		{"interval:90", base.Add(90 * time.Minute)},
		{"daily:08:00", time.Date(2026, 9, 9, 8, 0, 0, 0, loc)},   // ertesi gün
		{"daily:14:30", time.Date(2026, 9, 8, 14, 30, 0, 0, loc)}, // aynı gün
		{"weekly:mon:07:00", time.Date(2026, 9, 14, 7, 0, 0, 0, loc)},
		{"monthly:1:06:00", time.Date(2026, 10, 1, 6, 0, 0, 0, loc)},
		{"monthly:31:06:00", time.Date(2026, 9, 30, 6, 0, 0, 0, loc)}, // Eylül 30 çekilir
	}
	for _, c := range cases {
		got, err := NextRun(c.spec, base)
		if err != nil {
			t.Fatalf("%s: %v", c.spec, err)
		}
		if !got.Equal(c.want) {
			t.Errorf("%s: %s, beklenen %s", c.spec, got, c.want)
		}
	}
	if _, err := NextRun("bogus:x", base); err == nil {
		t.Error("geçersiz spec hata vermeli")
	}
}

func TestSchedulerRunsAndAdvances(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sch.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	now := time.Now().Unix()
	// biri geçmişte due (kaçırılan koşu), biri gelecekte
	id, _ := st.CreateScheduledJob(store.ScheduledJob{
		Kind: "test", Spec: "interval:60", Payload: `{"x":1}`, Enabled: true,
		NextRunTs: now - 300, CreatedTs: now,
	})
	st.CreateScheduledJob(store.ScheduledJob{
		Kind: "test", Spec: "interval:60", Enabled: true, NextRunTs: now + 3600, CreatedTs: now,
	})

	var runs atomic.Int32
	var lastPayload atomic.Value
	s := New(st)
	s.tick = 20 * time.Millisecond
	s.Register("test", func(_ context.Context, payload string) error {
		runs.Add(1)
		lastPayload.Store(payload)
		return nil
	})
	s.Start()
	defer s.Stop()

	deadline := time.After(2 * time.Second)
	for runs.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("iş çalışmadı")
		case <-time.After(10 * time.Millisecond):
		}
	}
	time.Sleep(60 * time.Millisecond) // ikinci sweep de gelsin

	if n := runs.Load(); n != 1 {
		t.Fatalf("kaçırılan iş 1 kez çalışmalı (gelecekteki henüz değil), %d", n)
	}
	if lastPayload.Load() != `{"x":1}` {
		t.Fatalf("payload geçmedi: %v", lastPayload.Load())
	}
	// next_run_ts ileriye kaydı mı?
	jobs, _ := st.ListScheduledJobs()
	for _, j := range jobs {
		if j.ID == id {
			if j.NextRunTs <= now {
				t.Fatalf("next_run_ts ileriye kaymadı: %d (now %d)", j.NextRunTs, now)
			}
			if j.LastStatus != "ok" {
				t.Fatalf("last_status: %q", j.LastStatus)
			}
		}
	}
}

func TestSchedulerLeaderGate(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "sch2.db"))
	t.Cleanup(func() { st.Close() })
	st.CreateScheduledJob(store.ScheduledJob{Kind: "test", Spec: "interval:60", Enabled: true, NextRunTs: time.Now().Unix() - 10, CreatedTs: time.Now().Unix()})

	var runs atomic.Int32
	s := New(st)
	s.tick = 20 * time.Millisecond
	s.SetLeaderCheck(func() bool { return false })
	s.Register("test", func(context.Context, string) error { runs.Add(1); return nil })
	s.Start()
	defer s.Stop()
	time.Sleep(200 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("lider değilken çalışmamalı, %d", runs.Load())
	}
}
