// Package aijob, gecelik AI filo analizi zamanlanmış işidir (Faz 26-E).
// internal/scheduler'a "ai_report" türü olarak kaydedilir; lider-kapılı
// çalışır. Sonuç bir sohbet oturumuna (source=nightly) yazılır ve alıcı
// varsa e-postalanır.
package aijob

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/scheduler"
	"github.com/gokayybaz/bazntms/internal/store"
)

// MailFn, üretilen analizi e-postalar. nil → e-posta atlanır.
type MailFn func(to []string, subject string, htmlBody []byte) error

// Handler, "ai_report" iş türü işleyicisi.
func Handler(reg *ai.Registry, snapshot func(scope, ref, site string) ai.Snapshot, recipients []string, mail MailFn) scheduler.Handler {
	return func(ctx context.Context, _ string) error {
		if reg == nil || !reg.Enabled() {
			return fmt.Errorf("ai kapalı")
		}
		ad, prov, err := reg.Adapter(0)
		if err != nil {
			return err
		}
		snap := snapshot("fleet", "", "")
		out, usage, err := ai.Analyze(ctx, ad, prov.DefaultModel, ai.SystemAnalyst, snap,
			ai.TaskFleetSummary, true /* chunked — küçük yerel modeller */, reg.Cfg().MaxContextKB)
		if err != nil {
			return err
		}

		st := reg.Store()
		cid, err := st.CreateAIConversation(store.AIConversation{
			Title:      "Gecelik filo analizi — " + time.Now().Format("2006-01-02"),
			CreatedBy:  "ai-nightly",
			ScopeKind:  "fleet",
			Source:     "nightly",
			ProviderID: prov.ID,
			Model:      prov.DefaultModel,
		})
		if err != nil {
			return err
		}
		_, _ = st.AppendAIMessage(store.AIMessage{ConversationID: cid, Role: "user", Content: "[gecelik otomatik analiz]"})
		_, _ = st.AppendAIMessage(store.AIMessage{
			ConversationID: cid, Role: "assistant", Content: out,
			TokensIn: usage.PromptTokens, TokensOut: usage.CompletionTokens,
		})

		if len(recipients) > 0 && mail != nil {
			body := []byte("<pre style=\"font:13px/1.5 monospace;white-space:pre-wrap\">" + html.EscapeString(out) + "</pre>")
			if err := mail(recipients, "bazNTMS — Gecelik AI Filo Analizi", body); err != nil {
				return fmt.Errorf("e-posta: %w", err)
			}
		}
		return nil
	}
}

// EnsureJob, "ai_report" scheduled_jobs satırını bir kez oluşturur (varsa
// dokunmaz — kullanıcı spec'i panelden değiştirmiş olabilir).
func EnsureJob(st store.SchedulerStore, spec string) error {
	jobs, err := st.ListScheduledJobs()
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if j.Kind == "ai_report" {
			return nil
		}
	}
	next, err := scheduler.NextRun(spec, time.Now())
	if err != nil {
		return fmt.Errorf("geçersiz spec %q: %w", spec, err)
	}
	_, err = st.CreateScheduledJob(store.ScheduledJob{
		Kind: "ai_report", Spec: spec, Payload: "{}", Enabled: true,
		NextRunTs: next.Unix(), CreatedBy: "system",
	})
	return err
}
