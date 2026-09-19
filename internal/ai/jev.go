package ai

// Jev, TypeSafe AI'nin "System One" modelidir (Faz 27): metin uretmez,
// state+questions alip sema garantili tipli/olasiliksal cevaplar dondurur
// (POST /v1/systemone). Mevcut Adapter arayuzune (Complete/Stream/Models)
// UYMAZ — bu yuzden ayri, kucuk bir istemci; ai_providers/Registry'ye
// KATILMAZ (aksi halde DefaultProvider() bir Jev satirini sohbet
// saglayicisi sanabilir). AI danisman sinirinin (bkz. docs/decisions/
// 0014-ai-analysis.md) bir uzantisidir: yalniz gating/skor icin kullanilir,
// hicbir motorun yerine gecmez.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	jevDefaultBaseURL = "https://api.typesafe.ai/v1"
	jevDefaultModel   = "jev-latest"
)

// JevQuestionType, Jev'in desteklediği üç soru türü.
type JevQuestionType string

const (
	JevChoice JevQuestionType = "choice"
	JevScore  JevQuestionType = "score"
	JevNoul   JevQuestionType = "noul"
)

// JevQuestion, tek bir yapılandırılmış karar sorusu.
type JevQuestion struct {
	Type         JevQuestionType `json:"type"`
	Instructions string          `json:"instructions"`
	// Criteria: Choice → map[string]string (seçenek→açıklama), Score →
	// []string (sıralı seviyeler), Noul → opsiyonel map[string]string.
	Criteria any `json:"criteria,omitempty"`
}

// JevAnswer, bir soruya karşılık gelen tipli/olasılıklı cevap.
type JevAnswer struct {
	Type          JevQuestionType    `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

type jevRequest struct {
	State     any                    `json:"state"`
	Model     string                 `json:"model"`
	Questions map[string]JevQuestion `json:"questions"`
}

type jevUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type jevResponse struct {
	Model   string               `json:"model"`
	Answers map[string]JevAnswer `json:"answers"`
	Usage   jevUsage             `json:"usage"`
}

// JevStatus, Yönetim UI'sı için Jev'in yapılandırma/durum özeti — sır
// İÇERMEZ (API anahtarı yok). Faz 27 S27.7.
type JevStatus struct {
	FlagOn        bool    `json:"flag_on"`     // -ai-jev bayrağı verildi mi
	AllowCloud    bool    `json:"allow_cloud"` // -ai-allow-cloud
	Active        bool    `json:"active"`      // FlagOn && AllowCloud
	BaseURL       string  `json:"base_url"`
	Model         string  `json:"model"`
	MinConfidence float64 `json:"min_confidence"`
}

// JevClient, TypeSafe AI'nin /v1/systemone ucuna karsi kucuk bir istemci.
type JevClient struct {
	baseURL string
	apiKey  string
	model   string
	hc      *http.Client
}

// NewJevClient, bos baseURL/model icin makul varsayilanlarla bir istemci
// kurar. hc nil ise kisa timeout'lu dahili istemci kullanilir — bu, gating
// amacli bir on-filtre oldugu icin sohbetin (StreamTimeout) aksine kisa
// tutulur.
func NewJevClient(baseURL, apiKey, model string, hc *http.Client) *JevClient {
	if baseURL == "" {
		baseURL = jevDefaultBaseURL
	}
	if model == "" {
		model = jevDefaultModel
	}
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &JevClient{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), apiKey: apiKey, model: model, hc: hc}
}

// Decide, state ve sorulari Jev'e gonderir, cevap haritasini dondurur.
func (c *JevClient) Decide(ctx context.Context, state any, questions map[string]JevQuestion) (map[string]JevAnswer, Usage, error) {
	body, err := json.Marshal(jevRequest{State: state, Model: c.model, Questions: questions})
	if err != nil {
		return nil, Usage{}, fmt.Errorf("jev istegi kodlanamadi: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/systemone", bytes.NewReader(body))
	if err != nil {
		return nil, Usage{}, fmt.Errorf("jev istegi kurulamadi: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("jev servisine ulasilamadi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return nil, Usage{}, fmt.Errorf("jev servisi hata dondu (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out jevResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, Usage{}, fmt.Errorf("jev yaniti ayristirilamadi: %w", err)
	}
	usage := Usage{PromptTokens: out.Usage.InputTokens, CompletionTokens: out.Usage.OutputTokens}
	return out.Answers, usage, nil
}
