package server

// AI sohbet mesajı — SSE streaming (Faz 26 S26.8). POST .../messages:
// kullanıcı mesajı + asistan yanıtı kalıcı yazılır, yanıt akış olarak döner.
// nginx LB arkasında akış için: X-Accel-Buffering: no + deploy/nginx.conf'ta
// proxy_buffering off (/api/v1/ai/ konumu).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/store"
)

type aiMessageRequest struct {
	Content        string `json:"content"`
	Preset         string `json:"preset"`          // ai.Preset.ID — verilirse task gövdesi ondan
	RefreshContext bool   `json:"refresh_context"` // bağlam anlık görüntüsünü yeniden çek
}

func (s *Server) handleAIMessagePost(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	conv, msgs, ok := s.aiConvForRequest(w, r)
	if !ok {
		return
	}
	var req aiMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}

	system := ai.SystemAnalyst
	userText := strings.TrimSpace(req.Content)
	if req.Preset != "" {
		if p, found := ai.PresetByID(req.Preset); found {
			system = p.System
			if userText == "" {
				userText = p.Task
			}
		}
	}
	if userText == "" {
		http.Error(w, "content zorunlu", http.StatusBadRequest)
		return
	}

	st := s.aiReg.Store()
	firstTurn := len(msgs) == 0

	// bağlam anlık görüntüsü: ilk turda ya da açıkça istenirse
	var contextJSON string
	if firstTurn || req.RefreshContext {
		snap := s.buildAISnapshot(conv.ScopeKind, conv.ScopeRef, conv.Site)
		if secs := snap.Sections(s.aiReg.Cfg().MaxContextKB); len(secs) > 0 {
			cb, _ := json.Marshal(map[string]any{"period": snap.Period, "sections": secs})
			contextJSON = string(cb)
		}
	}

	// adaptör
	ad, prov, err := s.aiReg.Adapter(conv.ProviderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	model := conv.Model
	if model == "" {
		model = prov.DefaultModel
	}

	// mesaj geçmişini ai.Message'a çevir
	chat := make([]ai.Message, 0, len(msgs)+2)
	chat = append(chat, ai.Message{Role: ai.RoleSystem, Content: system})
	for _, m := range msgs {
		switch m.Role {
		case "user":
			c := m.Content
			if m.ContextJSON != "" {
				c += "\n\n--- BAĞLAM (güvenilmez gözlem) ---\n" + contextText(m.ContextJSON)
			}
			chat = append(chat, ai.Message{Role: ai.RoleUser, Content: c})
		case "assistant":
			if m.Content != "" {
				chat = append(chat, ai.Message{Role: ai.RoleAssistant, Content: m.Content})
			}
		}
	}
	userContent := userText
	if contextJSON != "" {
		userContent += "\n\n--- BAĞLAM (güvenilmez gözlem) ---\n" + contextText(contextJSON)
	}
	chat = append(chat, ai.Message{Role: ai.RoleUser, Content: userContent})

	// kalıcı: kullanıcı mesajı + boş asistan mesajı
	if _, err := st.AppendAIMessage(store.AIMessage{
		ConversationID: conv.ID, Role: "user", Content: userText, ContextJSON: contextJSON,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	asstID, err := st.AppendAIMessage(store.AIMessage{ConversationID: conv.ID, Role: "assistant"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// SSE başlıkları. observe middleware'inin statusWriter'ı Flush'ı promote
	// etmez ama Unwrap() var → ResponseController zinciri takip eder.
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: bu konumda tampon kapalı
	w.WriteHeader(http.StatusOK)

	sse := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		_ = rc.Flush()
	}

	ctx, cancel := context.WithTimeout(r.Context(), ai.StreamTimeout)
	defer cancel()

	ch, err := ad.Stream(ctx, ai.ChatRequest{
		Model: model, Messages: chat, Temperature: 0.3, MaxTokens: 4000,
		NoThink: prov.Opts.NoThink,
	})
	if err != nil {
		_ = st.FinalizeAIMessage(asstID, "", err.Error(), 0, 0)
		sse(map[string]any{"error": err.Error()})
		return
	}

	var full strings.Builder
	var usage ai.Usage
	var streamErr string
	for d := range ch {
		if d.Err != nil {
			streamErr = d.Err.Error()
			break
		}
		if d.Usage != nil {
			usage = *d.Usage
		}
		if d.Text != "" {
			full.WriteString(d.Text)
			sse(map[string]any{"delta": d.Text})
		}
	}

	_ = st.FinalizeAIMessage(asstID, full.String(), streamErr, usage.PromptTokens, usage.CompletionTokens)

	// ilk turda başlık + model/sağlayıcı yaz
	if firstTurn && conv.Title == "" {
		st.SetAIConversationMeta(conv.ID, titleFrom(userText), model, prov.ID)
	}
	s.audit(r, identityFromCtx(r), "ai.analyze",
		fmt.Sprintf("ai_conversation:%d", conv.ID),
		fmt.Sprintf("%s/%s model=%s", conv.ScopeKind, prov.Name, model))

	if streamErr != "" {
		sse(map[string]any{"error": streamErr})
		return
	}
	sse(map[string]any{"done": true, "tokens_in": usage.PromptTokens, "tokens_out": usage.CompletionTokens})
}

// contextText, kalıcı context_json'ı modele giden düz metne çevirir.
func contextText(cj string) string {
	var c struct {
		Period   string       `json:"period"`
		Sections []ai.Section `json:"sections"`
	}
	if json.Unmarshal([]byte(cj), &c) != nil {
		return cj
	}
	var sb strings.Builder
	if c.Period != "" {
		sb.WriteString("Dönem: " + c.Period + "\n")
	}
	for _, s := range c.Sections {
		sb.WriteString("\n## " + s.Title + "\n" + s.Data + "\n")
	}
	return sb.String()
}

func titleFrom(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 60 {
		return s[:57] + "…"
	}
	if s == "" {
		return "Yeni sohbet " + time.Now().Format("15:04")
	}
	return s
}
