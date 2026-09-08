package store

import (
	"database/sql"
	"time"
)

// --- AI analiz (Faz 26) ---
//
// Monolit döneminde internal/ai vardı (d92d0fb), 9d22e7a'da silindi. Geri
// getirilip çoklu-sağlayıcı + kalıcı sohbet modeline yükseltildi. Şema 0021.
// api_key_enc vault ile şifreli — server katmanı şifreler/çözer (devices.go
// deseni); store düz metni asla görmez, olduğu gibi saklar.

type AIProvider struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"` // openai|anthropic|ollama|lmstudio|openai-compat
	BaseURL      string `json:"base_url"`
	APIKeyEnc    string `json:"-"` // vault-şifreli; API'ye asla düz sızmaz
	DefaultModel string `json:"default_model"`
	OptsJSON     string `json:"opts_json"`
	Enabled      bool   `json:"enabled"`
	CreatedBy    string `json:"created_by"`
	CreatedTs    int64  `json:"created_ts"`
	UpdatedTs    int64  `json:"updated_ts"`
}

type AIConversation struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	CreatedBy  string `json:"created_by"`
	Site       string `json:"site"`
	ScopeKind  string `json:"scope_kind"` // fleet|agent|incident|anomaly|device
	ScopeRef   string `json:"scope_ref"`
	ProviderID int64  `json:"provider_id"`
	Model      string `json:"model"`
	Source     string `json:"source"` // user|nightly|triage
	CreatedTs  int64  `json:"created_ts"`
	UpdatedTs  int64  `json:"updated_ts"`
	Archived   bool   `json:"archived"`
}

type AIMessage struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	Role           string `json:"role"` // system|user|assistant
	Content        string `json:"content"`
	ContextJSON    string `json:"context_json,omitempty"`
	TokensIn       int    `json:"tokens_in"`
	TokensOut      int    `json:"tokens_out"`
	Error          string `json:"error,omitempty"`
	CreatedTs      int64  `json:"created_ts"`
}

type AIConversationFilter struct {
	ScopeKind string
	ScopeRef  string
	Site      string // "" = tümü (global admin); dolu = yalnız o saha + fleet
	Source    string
	Archived  bool // true → yalnız arşivlenmiş; false → yalnız aktif
	Limit     int
}

// --- sağlayıcılar ---

const aiProviderCols = `id, name, kind, base_url, api_key_enc, default_model,
	opts_json, enabled, created_by, created_ts, updated_ts`

func scanAIProvider(sc interface{ Scan(...any) error }) (AIProvider, error) {
	var p AIProvider
	var enabled int64
	err := sc.Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.APIKeyEnc, &p.DefaultModel,
		&p.OptsJSON, &enabled, &p.CreatedBy, &p.CreatedTs, &p.UpdatedTs)
	p.Enabled = enabled != 0
	return p, err
}

func (s *sqlStore) ListAIProviders() ([]AIProvider, error) {
	rows, err := s.db.Query(s.q(`SELECT ` + aiProviderCols + ` FROM ai_providers ORDER BY name`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AIProvider{}
	for rows.Next() {
		p, err := scanAIProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *sqlStore) AIProviderByID(id int64) (*AIProvider, error) {
	p, err := scanAIProvider(s.db.QueryRow(s.q(`SELECT `+aiProviderCols+` FROM ai_providers WHERE id = ?`), id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *sqlStore) CreateAIProvider(p AIProvider) (int64, error) {
	now := time.Now().Unix()
	if p.OptsJSON == "" {
		p.OptsJSON = "{}"
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO ai_providers
		(name, kind, base_url, api_key_enc, default_model, opts_json, enabled, created_by, created_ts, updated_ts)
		VALUES (?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		p.Name, p.Kind, p.BaseURL, p.APIKeyEnc, p.DefaultModel, p.OptsJSON,
		btoi(p.Enabled), p.CreatedBy, now, now).Scan(&id)
	return id, err
}

// UpdateAIProvider, sağlayıcıyı günceller. APIKeyEnc boş ise anahtar
// DEĞİŞTİRİLMEZ (kullanıcı formu boş bıraktı = "aynı kalsın").
func (s *sqlStore) UpdateAIProvider(p AIProvider) error {
	now := time.Now().Unix()
	if p.OptsJSON == "" {
		p.OptsJSON = "{}"
	}
	q := `UPDATE ai_providers SET name=?, kind=?, base_url=?, default_model=?,
		opts_json=?, enabled=?, updated_ts=?`
	args := []any{p.Name, p.Kind, p.BaseURL, p.DefaultModel, p.OptsJSON, btoi(p.Enabled), now}
	if p.APIKeyEnc != "" {
		q += `, api_key_enc=?`
		args = append(args, p.APIKeyEnc)
	}
	q += ` WHERE id=?`
	args = append(args, p.ID)
	_, err := s.db.Exec(s.q(q), args...)
	return err
}

func (s *sqlStore) DeleteAIProvider(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM ai_providers WHERE id = ?`), id)
	return err
}

// --- konuşmalar & mesajlar ---

const aiConvCols = `id, title, created_by, site, scope_kind, scope_ref,
	provider_id, model, source, created_ts, updated_ts, archived`

func scanAIConv(sc interface{ Scan(...any) error }) (AIConversation, error) {
	var c AIConversation
	var archived int64
	err := sc.Scan(&c.ID, &c.Title, &c.CreatedBy, &c.Site, &c.ScopeKind, &c.ScopeRef,
		&c.ProviderID, &c.Model, &c.Source, &c.CreatedTs, &c.UpdatedTs, &archived)
	c.Archived = archived != 0
	return c, err
}

func (s *sqlStore) CreateAIConversation(c AIConversation) (int64, error) {
	now := time.Now().Unix()
	if c.CreatedTs == 0 {
		c.CreatedTs = now
	}
	c.UpdatedTs = now
	if c.ScopeKind == "" {
		c.ScopeKind = "fleet"
	}
	if c.Source == "" {
		c.Source = "user"
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO ai_conversations
		(title, created_by, site, scope_kind, scope_ref, provider_id, model, source, created_ts, updated_ts, archived)
		VALUES (?,?,?,?,?,?,?,?,?,?,0) RETURNING id`),
		c.Title, c.CreatedBy, c.Site, c.ScopeKind, c.ScopeRef, c.ProviderID, c.Model,
		c.Source, c.CreatedTs, c.UpdatedTs).Scan(&id)
	return id, err
}

func (s *sqlStore) ListAIConversations(f AIConversationFilter) ([]AIConversation, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + aiConvCols + ` FROM ai_conversations WHERE archived = ?`
	args := []any{btoi(f.Archived)}
	if f.ScopeKind != "" {
		q += ` AND scope_kind = ?`
		args = append(args, f.ScopeKind)
	}
	if f.ScopeRef != "" {
		q += ` AND scope_ref = ?`
		args = append(args, f.ScopeRef)
	}
	if f.Source != "" {
		q += ` AND source = ?`
		args = append(args, f.Source)
	}
	// site kapsamı: dolu ise o saha VEYA saha-üstü (site='') konuşmalar
	if f.Site != "" {
		q += ` AND (site = ? OR site = '')`
		args = append(args, f.Site)
	}
	q += ` ORDER BY updated_ts DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AIConversation{}
	for rows.Next() {
		c, err := scanAIConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *sqlStore) AIConversationByID(id int64) (*AIConversation, []AIMessage, error) {
	c, err := scanAIConv(s.db.QueryRow(s.q(`SELECT `+aiConvCols+` FROM ai_conversations WHERE id = ?`), id))
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.Query(s.q(`SELECT id, conversation_id, role, content, context_json,
		tokens_in, tokens_out, error, created_ts FROM ai_messages
		WHERE conversation_id = ? ORDER BY created_ts ASC, id ASC`), id)
	if err != nil {
		return &c, nil, err
	}
	defer rows.Close()
	msgs := []AIMessage{} // JSON'da hiç null olmasın (UI .filter/.length çağırır)
	for rows.Next() {
		var m AIMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ContextJSON,
			&m.TokensIn, &m.TokensOut, &m.Error, &m.CreatedTs); err != nil {
			return &c, msgs, err
		}
		msgs = append(msgs, m)
	}
	return &c, msgs, rows.Err()
}

// AppendAIMessage, bir mesaj ekler ve konuşmanın updated_ts'ini ilerletir.
func (s *sqlStore) AppendAIMessage(m AIMessage) (int64, error) {
	now := time.Now().Unix()
	if m.CreatedTs == 0 {
		m.CreatedTs = now
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO ai_messages
		(conversation_id, role, content, context_json, tokens_in, tokens_out, error, created_ts)
		VALUES (?,?,?,?,?,?,?,?) RETURNING id`),
		m.ConversationID, m.Role, m.Content, m.ContextJSON, m.TokensIn, m.TokensOut,
		m.Error, m.CreatedTs).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, _ = s.db.Exec(s.q(`UPDATE ai_conversations SET updated_ts = ? WHERE id = ?`), now, m.ConversationID)
	return id, nil
}

// FinalizeAIMessage, streaming biten bir asistan mesajının gövdesini yazar
// (AppendAIMessage ile boş oluşturulmuş satırı tamamlar).
func (s *sqlStore) FinalizeAIMessage(id int64, content, errStr string, tokensIn, tokensOut int) error {
	_, err := s.db.Exec(s.q(`UPDATE ai_messages SET content = ?, error = ?, tokens_in = ?, tokens_out = ?
		WHERE id = ?`), content, errStr, tokensIn, tokensOut, id)
	return err
}

// SetAIConversationMeta, ilk alışverişten sonra başlık/model/sağlayıcıyı yazar.
func (s *sqlStore) SetAIConversationMeta(id int64, title, model string, providerID int64) error {
	_, err := s.db.Exec(s.q(`UPDATE ai_conversations SET title = ?, model = ?, provider_id = ?, updated_ts = ?
		WHERE id = ?`), title, model, providerID, time.Now().Unix(), id)
	return err
}

func (s *sqlStore) SetAIConversationArchived(id int64, archived bool) error {
	_, err := s.db.Exec(s.q(`UPDATE ai_conversations SET archived = ?, updated_ts = ? WHERE id = ?`),
		btoi(archived), time.Now().Unix(), id)
	return err
}

// DeleteAIConversation, kalıcı siler (mesajlar dahil — Go cascade, DeleteAgent
// deseni). Yalnız admin / prune; UI arşivler.
func (s *sqlStore) DeleteAIConversation(id int64) error {
	if _, err := s.db.Exec(s.q(`DELETE FROM ai_messages WHERE conversation_id = ?`), id); err != nil {
		return err
	}
	_, err := s.db.Exec(s.q(`DELETE FROM ai_conversations WHERE id = ?`), id)
	return err
}

// PruneAIConversations, `before`'dan eski arşivlenmiş konuşmaları (ve
// mesajlarını) siler. Silinen konuşma id'lerini döndürür.
func (s *sqlStore) PruneAIConversations(before int64) ([]int64, error) {
	rows, err := s.db.Query(s.q(`SELECT id FROM ai_conversations WHERE archived = 1 AND updated_ts < ?`), before)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.DeleteAIConversation(id); err != nil {
			return ids, err
		}
	}
	return ids, nil
}
