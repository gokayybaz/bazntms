package server

// AI analiz uçları (Faz 26). Sohbet + sağlayıcı yönetimi + tek-atışlık analiz.
// Sohbet = PermAnalyze; sağlayıcı CRUD = PermGlobalAdmin. api_key hiçbir GET
// yanıtında düz görünmez. Motor: internal/ai. Bkz. docs/decisions/0014.

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

// SetAIRegistry, AI motorunu bağlar (main.go, ai.enabled ise). nil → uçlar 503.
func (s *Server) SetAIRegistry(r *ai.Registry) { s.aiReg = r }

func (s *Server) aiReady(w http.ResponseWriter) bool {
	if s.aiReg == nil || !s.aiReg.Enabled() {
		http.Error(w, "AI analiz kapalı (ai.enabled)", http.StatusServiceUnavailable)
		return false
	}
	return true
}

// --- durum + presetler ---

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	if s.aiReg == nil || !s.aiReg.Enabled() {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	provs, _ := s.aiReg.Store().ListAIProviders()
	out := make([]map[string]any, 0, len(provs))
	var defModel string
	def, derr := s.aiReg.DefaultProvider()
	if derr == nil {
		defModel = def.DefaultModel
	}
	for _, p := range provs {
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "kind": p.Kind, "enabled": p.Enabled,
			"default_model": p.DefaultModel,
		})
	}
	writeJSON(w, map[string]any{
		"enabled":            true,
		"providers":          out,
		"default_provider":   defaultProviderID(def, derr),
		"default_model":      defModel,
		"has_ready_provider": derr == nil,
	})
}

func defaultProviderID(p ai.Provider, err error) int64 {
	if err != nil {
		return 0
	}
	return p.ID
}

func (s *Server) handleAIPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"presets": ai.Presets()})
}

// --- sağlayıcı CRUD (PermGlobalAdmin) ---

type aiProviderRequest struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"` // düz metin → vault; boş = değiştirme
	DefaultModel string   `json:"default_model"`
	Enabled      bool     `json:"enabled"`
	NoThink      bool     `json:"no_think"`
	MaxTokens    int      `json:"max_tokens"`
	Temperature  *float64 `json:"temperature"`
}

func (req aiProviderRequest) toRecord(id int64, by string) (store.AIProvider, error) {
	opts := ai.Opts{NoThink: req.NoThink, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	ob, _ := json.Marshal(opts)
	rec := store.AIProvider{
		ID: id, Name: strings.TrimSpace(req.Name), Kind: req.Kind,
		BaseURL: req.BaseURL, DefaultModel: req.DefaultModel,
		OptsJSON: string(ob), Enabled: req.Enabled, CreatedBy: by,
	}
	if rec.Name == "" {
		return rec, fmt.Errorf("name zorunlu")
	}
	return rec, nil
}

func (s *Server) handleAIProvidersList(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	provs, err := s.aiReg.Store().ListAIProviders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, len(provs))
	for i, p := range provs {
		out[i] = map[string]any{
			"id": p.ID, "name": p.Name, "kind": p.Kind, "base_url": p.BaseURL,
			"default_model": p.DefaultModel, "enabled": p.Enabled, "opts_json": p.OptsJSON,
			"has_key":    p.APIKeyEnc != "",
			"created_by": p.CreatedBy, "updated_ts": p.UpdatedTs,
			"is_local": ai.IsLocalURL(p.BaseURL),
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleAIProviderCreate(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	var req aiProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	rec, err := req.toRecord(0, usernameOf(ident))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := s.aiReg.SaveProvider(rec, req.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.auditDiff(r, ident, "ai.provider.create", fmt.Sprintf("ai_provider:%d", id), rec.Name, nil, req)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleAIProviderUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	var req aiProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	rec, err := req.toRecord(id, usernameOf(ident))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.aiReg.SaveProvider(rec, req.APIKey); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.auditDiff(r, ident, "ai.provider.update", fmt.Sprintf("ai_provider:%d", id), rec.Name, nil, req)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAIProviderDelete(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if err := s.aiReg.Store().DeleteAIProvider(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "ai.provider.delete", fmt.Sprintf("ai_provider:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAIProviderTest(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res := s.aiReg.TestProvider(ctx, id)
	s.audit(r, identityFromCtx(r), "ai.provider.test", fmt.Sprintf("ai_provider:%d", id), boolStr(res.OK))
	writeJSON(w, res)
}

func (s *Server) handleAIProviderModels(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	models, err := s.aiReg.Models(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"models": models})
}

// --- konuşmalar (PermAnalyze) ---

func (s *Server) handleAIConversationsList(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	q := r.URL.Query()
	f := store.AIConversationFilter{
		ScopeKind: q.Get("scope"),
		ScopeRef:  q.Get("ref"),
		Source:    q.Get("source"),
		Archived:  q.Get("archived") == "1" || q.Get("archived") == "true",
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		f.Site = scope
	}
	list, err := s.aiReg.Store().ListAIConversations(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

type aiConversationRequest struct {
	ScopeKind  string `json:"scope_kind"`
	ScopeRef   string `json:"scope_ref"`
	ProviderID int64  `json:"provider_id"`
	Model      string `json:"model"`
	Title      string `json:"title"`
}

func (s *Server) handleAIConversationCreate(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	var req aiConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	c := store.AIConversation{
		Title: req.Title, CreatedBy: usernameOf(ident), Site: SiteScope(ident),
		ScopeKind: req.ScopeKind, ScopeRef: req.ScopeRef,
		ProviderID: req.ProviderID, Model: req.Model, Source: "user",
	}
	id, err := s.aiReg.Store().CreateAIConversation(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleAIConversationGet(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	conv, msgs, ok := s.aiConvForRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, map[string]any{"conversation": conv, "messages": msgs})
}

func (s *Server) handleAIConversationArchive(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	conv, _, ok := s.aiConvForRequest(w, r)
	if !ok {
		return
	}
	if err := s.aiReg.Store().SetAIConversationArchived(conv.ID, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// aiConvForRequest, {id}'yi çözer + site kapsamını doğrular.
func (s *Server) aiConvForRequest(w http.ResponseWriter, r *http.Request) (*store.AIConversation, []store.AIMessage, bool) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return nil, nil, false
	}
	conv, msgs, err := s.aiReg.Store().AIConversationByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, nil, false
	}
	if conv == nil {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return nil, nil, false
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" && conv.Site != "" && conv.Site != scope {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return nil, nil, false
	}
	return conv, msgs, true
}

func usernameOf(id *Identity) string {
	if id == nil {
		return ""
	}
	return id.Username
}

func boolStr(b bool) string {
	if b {
		return "ok"
	}
	return "fail"
}
