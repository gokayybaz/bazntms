// Package incident, ilişkili uyarıları deterministik kurallarla bir olaya
// (incident) toplar (Faz 24-B). AI/LLM YOK — sabit kurallar + açıklanabilir
// risk skoru. Lider-kapılı: çoklu controller replikasında yalnız lider
// değerlendirir. Bkz. docs/decisions/0011-incident-engine.md.
package incident

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Config, incident motoru ayarları (alert config JSON'da "incident").
type Config struct {
	Enabled        bool `json:"enabled"`
	ShortWindowMin int  `json:"short_window_min"` // kural 1/2/4 (vars. 5)
	LongWindowMin  int  `json:"long_window_min"`  // kural 3/5 (vars. 10)
	Rule5MinAlerts int  `json:"rule5_min_alerts"` // kural 5 eşiği (vars. 3)
}

func DefaultConfig() Config {
	return Config{Enabled: true, ShortWindowMin: 5, LongWindowMin: 10, Rule5MinAlerts: 3}
}

// Notifier, yeni açılan / önemi yükselen incident'ı bildirir. isNew=false →
// mevcut incident tazelendi (severity/risk arttı).
type Notifier func(in store.Incident, isNew bool)

type Engine struct {
	st       store.Store
	cfg      func() Config
	isLeader func() bool
	notify   Notifier
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func New(st store.Store, cfg func() Config) *Engine {
	return &Engine{
		st: st, cfg: cfg,
		interval: 30 * time.Second,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (e *Engine) SetLeaderCheck(fn func() bool) { e.isLeader = fn }
func (e *Engine) SetNotifier(fn Notifier)       { e.notify = fn }

func (e *Engine) Start() { go e.run() }
func (e *Engine) Stop()  { close(e.stopCh); <-e.doneCh }

func (e *Engine) run() {
	defer close(e.doneCh)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-t.C:
			if e.isLeader != nil && !e.isLeader() {
				continue
			}
			if err := e.Evaluate(time.Now()); err != nil {
				slog.Warn("incident degerlendirme hatasi", "err", err)
			}
		}
	}
}

// Evaluate, `now`'a göre korelasyon penceresindeki uyarıları değerlendirir.
func (e *Engine) Evaluate(now time.Time) error {
	cfg := e.cfg()
	if !cfg.Enabled {
		return nil
	}
	short := dur(cfg.ShortWindowMin, 5)
	long := dur(cfg.LongWindowMin, 10)
	min5 := cfg.Rule5MinAlerts
	if min5 < 2 {
		min5 = 3
	}

	// en geniş pencere kadar geriye bak
	alerts, err := e.st.AlertEventsSince(now.Add(-long).Unix())
	if err != nil {
		return err
	}

	// agent bazında grupla (agent_id=0 → korelasyon dışı, bkz. ADR)
	byAgent := map[int64][]store.AlertEvent{}
	for _, a := range alerts {
		if a.AgentID == 0 || a.State == "silenced" {
			continue
		}
		byAgent[a.AgentID] = append(byAgent[a.AgentID], a)
	}

	for agentID, cluster := range byAgent {
		sort.Slice(cluster, func(i, j int) bool { return cluster[i].LastTs < cluster[j].LastTs })
		for _, m := range matchRules(agentID, cluster, short, long, min5) {
			if err := e.upsert(m); err != nil {
				slog.Warn("incident upsert hatasi", "rule", m.rule, "agent", agentID, "err", err)
			}
		}
	}
	return nil
}

// upsert, bir kural eşleşmesini incident'a yazar (yeni ya da bump).
func (e *Engine) upsert(m match) error {
	open, err := e.st.OpenIncidentByCorrelation(m.key)
	if err != nil {
		return err
	}
	var incID int64
	var isNew bool
	var raised bool

	if open == nil {
		incID, err = e.st.CreateIncident(store.Incident{
			Title:             m.title,
			Severity:          m.severity,
			Status:            "open",
			Site:              m.site,
			AgentID:           m.agentID,
			CorrelationKey:    m.key,
			CorrelationReason: m.reason,
			Summary:           m.summary,
			RiskScore:         m.risk,
			FirstSeen:         m.firstTs,
			LastSeen:          m.lastTs,
		})
		if err != nil {
			return err
		}
		isNew = true
	} else {
		incID = open.ID
		// önem yükseldi mi / risk arttı mı → bildirilmeye değer
		raised = sevRank(m.severity) > sevRank(open.Severity) || m.risk > open.RiskScore
		if err := e.st.BumpIncident(incID, m.lastTs, m.severity, m.risk, m.reason, m.summary); err != nil {
			return err
		}
	}

	for _, ev := range m.evidence {
		if err := e.st.AddIncidentEvidence(incID, ev); err != nil {
			slog.Warn("kanıt eklenemedi", "incident", incID, "err", err)
		}
	}

	if e.notify != nil && (isNew || raised) {
		if in, _, err := e.st.IncidentByID(incID); err == nil && in != nil {
			e.notify(*in, isNew)
		}
	}
	if isNew {
		slog.Info("incident açıldı", "id", incID, "kural", m.rule, "agent", m.agentID, "severity", m.severity, "risk", m.risk)
	}
	return nil
}

func dur(min, def int) time.Duration {
	if min <= 0 {
		min = def
	}
	return time.Duration(min) * time.Minute
}

func sevRank(s string) int {
	switch s {
	case "crit":
		return 3
	case "warn":
		return 2
	default:
		return 1
	}
}

// maxSev, iki önemden yükseğini döndürür.
func maxSev(a, b string) string {
	if sevRank(a) >= sevRank(b) {
		return a
	}
	return b
}

func evidenceFor(a store.AlertEvent) store.IncidentEvidence {
	return store.IncidentEvidence{
		Kind: "alert", Ref: fmt.Sprint(a.ID), Ts: a.LastTs,
		Summary: fmt.Sprintf("[%s/%s] %s", a.Kind, a.Severity, a.Message),
	}
}
