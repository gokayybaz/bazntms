package ai

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Triager, yeni açılan incident'lar için otomatik AI triyaj notu üretir
// (Faz 26-E). Saatlik hız-sınırlı — LLM maliyet/gürültü koruması. AI
// DANIŞMAN: not incident'e iliştirilen bir konuşmaya yazılır, otomatik
// aksiyon YOK; deterministik korelasyon + risk skoru yetkili kalır.
//
// Lider denetimi gerekmez: çağıran (incident motoru notifier'ı) yalnız lider
// replikada ateşlenir.
type Triager struct {
	reg      *Registry
	snapshot func(scope, ref, site string) Snapshot
	minSev   string
	maxPerHr int

	mu     sync.Mutex
	window []time.Time // son 1 saatteki triyaj zamanları
}

func NewTriager(reg *Registry, snapshot func(scope, ref, site string) Snapshot, minSeverity string, maxPerHour int) *Triager {
	if minSeverity == "" {
		minSeverity = "crit"
	}
	if maxPerHour <= 0 {
		maxPerHour = 10
	}
	return &Triager{reg: reg, snapshot: snapshot, minSev: minSeverity, maxPerHr: maxPerHour}
}

// Enqueue, bir incident için triyaj başlatır (asenkron, hız-sınırlı). nil
// Triager'da no-op — çağıran tarafın kontrol etmesine gerek yok.
func (t *Triager) Enqueue(in store.Incident) {
	if t == nil || t.reg == nil || !t.reg.Enabled() {
		return
	}
	if sevRank(in.Severity) < sevRank(t.minSev) {
		return
	}
	if !t.allow() {
		slog.Warn("AI triyaj hız sınırı — atlandı", "incident", in.ID, "limit_saat", t.maxPerHr)
		return
	}
	go t.run(in)
}

func (t *Triager) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cut := time.Now().Add(-time.Hour)
	kept := t.window[:0]
	for _, ts := range t.window {
		if ts.After(cut) {
			kept = append(kept, ts)
		}
	}
	t.window = kept
	if len(t.window) >= t.maxPerHr {
		return false
	}
	t.window = append(t.window, time.Now())
	return true
}

func (t *Triager) run(in store.Incident) {
	ctx, cancel := context.WithTimeout(context.Background(), StreamTimeout)
	defer cancel()

	ad, prov, err := t.reg.Adapter(0)
	if err != nil {
		slog.Warn("AI triyaj sağlayıcı", "err", err)
		return
	}
	ref := strconv.FormatInt(in.ID, 10)
	snap := t.snapshot("incident", ref, in.Site)
	out, usage, err := Analyze(ctx, ad, prov.DefaultModel, SystemTriage, snap,
		"Bu olayı (incident) triyaj et: ne olmuş görünüyor, en olası açıklama + karşıt "+
			"olasılıklar, operatörün ilk bakacağı 2-3 somut yer, aciliyet değerlendirmesi.",
		false, t.reg.Cfg().MaxContextKB)
	if err != nil {
		slog.Warn("AI triyaj analizi", "incident", in.ID, "err", err)
		return
	}

	st := t.reg.Store()
	cid, err := st.CreateAIConversation(store.AIConversation{
		Title:      "Otomatik triyaj — olay #" + ref,
		CreatedBy:  "ai-triage",
		Site:       in.Site,
		ScopeKind:  "incident",
		ScopeRef:   ref,
		ProviderID: prov.ID,
		Model:      prov.DefaultModel,
		Source:     "triage",
	})
	if err != nil {
		slog.Warn("AI triyaj konuşması", "err", err)
		return
	}
	_, _ = st.AppendAIMessage(store.AIMessage{ConversationID: cid, Role: "user", Content: "[otomatik triyaj — yeni kritik olay]"})
	_, _ = st.AppendAIMessage(store.AIMessage{
		ConversationID: cid, Role: "assistant", Content: out,
		TokensIn: usage.PromptTokens, TokensOut: usage.CompletionTokens,
	})
	slog.Info("AI triyaj notu üretildi", "incident", in.ID, "conversation", cid)
}

func sevRank(s string) int {
	switch s {
	case "crit":
		return 2
	case "warn":
		return 1
	default:
		return 0
	}
}
