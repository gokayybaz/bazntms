// Package scheduler, hub-içi lider-kapılı zamanlanmış iş çalıştırıcısıdır
// (Faz 22 S22.18). Harici bağımlılık yok; alert.Manager ve devpoll ile aynı
// lider-seçim (C1) desenini kullanır. İlk tüketici zamanlanmış rapor teslimi
// (S22.19), sonra günlük SLA değerlendirmesi (S22.21).
//
// Kaçırılan koşu: lider bir süre düşükse next_run_ts geçmişte kalır; devir
// alan lider işi BİR KEZ çalıştırır ve next_run_ts'i ilk gelecek koşuma
// kaydırır (çift-teslim > hiç-teslim). Bkz. docs/decisions/0009-scheduled-jobs.md.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Handler, bir iş türünü çalıştırır. payload iş kaydındaki payload_json.
type Handler func(ctx context.Context, payload string) error

type Scheduler struct {
	st       store.SchedulerStore
	leader   func() bool
	handlers map[string]Handler
	tick     time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func New(st store.SchedulerStore) *Scheduler {
	return &Scheduler{
		st:       st,
		handlers: map[string]Handler{},
		tick:     30 * time.Second,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Register, bir iş türü için handler bağlar (Start'tan önce).
func (s *Scheduler) Register(kind string, h Handler) { s.handlers[kind] = h }

// SetLeaderCheck, çalıştırma öncesi liderlik denetimi (C1). nil → daima lider.
func (s *Scheduler) SetLeaderCheck(fn func() bool) { s.leader = fn }

func (s *Scheduler) leading() bool { return s.leader == nil || s.leader() }

func (s *Scheduler) Start() { go s.run() }

func (s *Scheduler) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *Scheduler) run() {
	defer close(s.doneCh)
	t := time.NewTicker(s.tick)
	defer t.Stop()
	// ilk kontrolü hemen yap
	s.sweep()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.sweep()
		}
	}
}

func (s *Scheduler) sweep() {
	if !s.leading() {
		return
	}
	now := time.Now()
	due, err := s.st.DueScheduledJobs(now.Unix())
	if err != nil {
		slog.Warn("zamanlanmış iş sorgusu hatası", "err", err)
		return
	}
	for _, j := range due {
		s.runJob(j, now)
	}
}

func (s *Scheduler) runJob(j store.ScheduledJob, now time.Time) {
	h := s.handlers[j.Kind]
	// bir sonraki koşum: her zaman şimdiden sonra (kaçırılan koşuları atla)
	next, nerr := NextRun(j.Spec, now)
	status := "ok"
	if h == nil {
		status = "handler yok: " + j.Kind
		slog.Warn("zamanlanmış iş: handler kayıtlı değil", "kind", j.Kind, "id", j.ID)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := h(ctx, j.Payload); err != nil {
			status = "hata: " + err.Error()
			slog.Warn("zamanlanmış iş hatası", "kind", j.Kind, "id", j.ID, "err", err)
		} else {
			slog.Info("zamanlanmış iş çalıştı", "kind", j.Kind, "id", j.ID, "sonraki", next.Format(time.RFC3339))
		}
		cancel()
	}
	nextTs := int64(0)
	if nerr == nil {
		nextTs = next.Unix()
	} else {
		status = "spec hatası: " + nerr.Error()
	}
	if err := s.st.MarkScheduledJobRun(j.ID, now.Unix(), nextTs, status); err != nil {
		slog.Warn("zamanlanmış iş kaydedilemedi", "id", j.ID, "err", err)
	}
}
