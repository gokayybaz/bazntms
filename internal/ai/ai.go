package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/store"
)

type Config struct {
	BaseURL   string // ex: https://api.openai.com/v1  veya  http://localhost:11434/v1 (Ollama)
	APIKey    string
	Model     string
	MaxTokens int  // 0 = dahili varsayilanlar
	NoThink   bool // Qwen3 tarzi modellerde /no_think anahtari (dusunmeyi kapatir)
}

func ConfigFromEnv() Config {
	base := os.Getenv("LLM_BASE_URL")
	if base == "" {
		base = os.Getenv("OPENAI_BASE_URL")
	}
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	key := os.Getenv("LLM_API_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	maxTokens := 0
	if v := os.Getenv("LLM_MAX_TOKENS"); v != "" {
		fmt.Sscanf(v, "%d", &maxTokens)
	}
	noThink := os.Getenv("LLM_NO_THINK") == "1" || os.Getenv("LLM_NO_THINK") == "true"
	return Config{BaseURL: strings.TrimRight(base, "/"), APIKey: key, Model: model, MaxTokens: maxTokens, NoThink: noThink}
}

func (c Config) Enabled() bool {
	return c.APIKey != "" || strings.Contains(c.BaseURL, "localhost") || strings.Contains(c.BaseURL, "127.0.0.1")
}

type Client struct {
	cfg Config
	hc  *http.Client
}

func NewClient(cfg Config) *Client {
	// reasoning modelleri yavas olabilir; uzun timeout gerekli
	return &Client{cfg: cfg, hc: &http.Client{Timeout: 300 * time.Second}}
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// reasoning modelleri (LM Studio, DeepSeek, OpenRouter vb.) final cevabi
	// buraya yazabilir; bos content durumunda yedek olarak kullanilir:
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// AnalyzeOptions, tek analiz isteginin parametreleri.
type AnalyzeOptions struct {
	Minutes int             // analiz donemi (dakika)
	Model   string          // bos ise varsayilan model
	Chunked bool            // veriyi parca parca gonder (kucuk modeller icin)
	Geo     *geoip.Resolver // nil degilse hedefler ulke/ASN ile zenginlestirilir
}

const systemAnalyst = "Sen deneyimli bir ag guvenligi ve performans analizcisisin. " +
	"Sana bir bilgisayarin ag trafigine ait ham istatistikler (JSON) verilecek. " +
	"TURKCE, kisa ve net yaz. Madde isaretleri kullan, gereksiz uzun cumlelerden kacin. " +
	"Yalnizca verilen veriye dayan; veri olmayan konu icin spekulasyon yapma."

// Analyze, verilen donemdeki trafik verisini ozetler ve Turkce analiz dondurur.
// Chunked modda veri parcalara bolunup ayri isteklerle gonderilir; her parcadan
// kisa not alinir, en son notlar birlestirilerek final analiz uretilir. Boylece
// kucuk parametreli modellerde context asiri yuklenmez.
func (c *Client) Analyze(ctx context.Context, st *store.Store, opts AnalyzeOptions) (string, error) {
	if !c.cfg.Enabled() {
		return "", fmt.Errorf("AI yapilandirilmamis: -llm-base-url (ör. http://localhost:11434/v1) veya LLM_API_KEY ile sunucuyu baslatin")
	}
	if opts.Minutes <= 0 {
		opts.Minutes = 30
	}
	model := c.cfg.Model
	if opts.Model != "" {
		model = opts.Model
	}

	parts, err := buildDataParts(st, opts.Minutes, opts.Geo)
	if err != nil {
		return "", fmt.Errorf("veri hazirlanamadi: %w", err)
	}

	if !opts.Chunked {
		return c.chatOnce(ctx, model, systemAnalyst,
			"Analiz donemi: son "+fmt.Sprint(opts.Minutes)+" dakika.\n\n"+parts.combined(),
			"Turkce analiz yaz: 1) Trafik ozeti, 2) En cok veri transfer eden hedefler/hizmetler yorumu, "+
				"3) Olağandışı durumlar veya guvenlik endiseleri (tuhaf portlar, bilinmeyen surecler, ani trafik zirveleri), "+
				"4) Oneriler.", 3000)
	}

	// parca parca mod: her veri turu icin kisa not al
	notes := make([]string, 0, len(parts.sections))
	for i, sec := range parts.sections {
		note, err := c.chatOnce(ctx, model, systemAnalyst,
			"Analiz donemi: son "+fmt.Sprint(opts.Minutes)+" dakika.\n\nVERI PARCASI "+fmt.Sprint(i+1)+"/"+fmt.Sprint(len(parts.sections))+" ("+sec.title+"):\n"+sec.data,
			"Bu parca SADECE bir kisim veri. Genel degerlendirme yapma. "+
				"Bu parcada gordugun 2-4 maddeyi kisa Turkce not olarak yaz: onemli sayilar, dikkat ceken/anomali gorunen noktalar. "+
				"Parcada olmayan seyler hakkinda konusma.", 1500)
		if err != nil {
			return "", fmt.Errorf("parca %d/%d (%s): %w", i+1, len(parts.sections), sec.title, err)
		}
		notes = append(notes, "### "+sec.title+"\n"+note)
	}

	// final birlestirme: modele sadece notlar gider, ham veri gitmez
	return c.chatOnce(ctx, model, systemAnalyst,
		"Analiz donemi: son "+fmt.Sprint(opts.Minutes)+" dakika. Trafige ait parcali analiz notlari asagida:\n\n"+
			strings.Join(notes, "\n\n"),
		"Bu notlari birlestirip final Turkce analizi yaz: 1) Trafik ozeti, 2) Hedef/hizmet yorumlari, "+
			"3) Olağandışı durumlar veya guvenlik endiseleri, 4) Oneriler.", 2500)
}

// chatOnce, tek chat completion istegi atar.
func (c *Client) chatOnce(ctx context.Context, model, system, user, task string, maxTokens int) (string, error) {
	if c.cfg.MaxTokens > 0 {
		maxTokens = c.cfg.MaxTokens
	}
	if c.cfg.NoThink {
		// Qwen3 serisi "soft switch": sistem mesajina eklenince dusunme modu kapali
		system += " /no_think"
	}
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system + " " + task},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI servisine ulasilamadi: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		var cr chatResponse
		if json.Unmarshal(raw, &cr) == nil && cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		return "", fmt.Errorf("AI servisi hata dondu (HTTP %d): %s", resp.StatusCode, msg)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("AI yanıtı ayrıştırılamadı: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("AI yanıtı boş döndü")
	}

	msg := cr.Choices[0].Message
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		// reasoning modelleri: cevap reasoning_content alanina dusebilir
		content = strings.TrimSpace(msg.ReasoningContent)
	}
	if content == "" {
		content = strings.TrimSpace(msg.Reasoning)
	}
	content = stripThink(content)

	if content == "" {
		if cr.Choices[0].FinishReason == "length" {
			return "", fmt.Errorf("model düşünme (reasoning) aşamasında token limitini aştı: -llm-max-tokens ile limiti artırın ya da -llm-no-think ile düşünmeyi kapatın")
		}
		return "", fmt.Errorf("AI boş yanıt döndü (model çok küçük olabilir, istek reddedilmiş veya bağlam taşmış olabilir)")
	}
	return content, nil
}

// stripThink, <think>...</think> dusunme bloklarini cevaptan ayiklar.
func stripThink(s string) string {
	lower := strings.ToLower(s)
	for {
		start := strings.Index(lower, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(lower[start:], "</think>")
		if end < 0 {
			// kapanmamis think blogu: baslangictan sonrasini at
			s = strings.TrimSuffix(s[:start], "</think>") + ""
			return strings.TrimSpace(s)
		}
		end += start + len("</think>")
		s = s[:start] + s[end:]
		lower = strings.ToLower(s)
	}
	return strings.TrimSpace(s)
}

func (c *Client) Model() string { return c.cfg.Model }
func (c *Client) Enabled() bool { return c.cfg.Enabled() }

type ModelInfo struct {
	ID string `json:"id"`
}

// ListModels, OpenAI-uyumlu /models ucundan mevcut modelleri ceker.
// Ollama, LM Studio, llama.cpp server ve vLLM bu ucu destekler.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model servisine ulasilamadi (%s): %w", c.cfg.BaseURL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model listesi alinamadi (HTTP %d): %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Data == nil {
		parsed.Data = []ModelInfo{}
	}
	return parsed.Data, nil
}

// dataParts, modele gonderilecek veriyi bagimsiz bolumlere ayirir.
// Chunked modda her bolum ayri istekle gonderilir; tek seferde modda
// hepsi birlestirilerek gonderilir.
type dataParts struct {
	sections []struct {
		title string
		data  string
	}
}

func (d dataParts) combined() string {
	var sb strings.Builder
	for _, s := range d.sections {
		sb.WriteString("## " + s.title + "\n")
		sb.WriteString(s.data)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// buildDataParts, DB'den donemi okuyup kompakt JSON bolumleri uretir.
func buildDataParts(st *store.Store, minutes int, geo *geoip.Resolver) (dataParts, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	totals, err := st.PeriodTotals(since)
	if err != nil {
		return dataParts{}, err
	}
	endpoints, err := st.TopEndpointsSince(since, 10)
	if err != nil {
		return dataParts{}, err
	}
	if geo != nil && geo.Enabled() {
		for i := range endpoints {
			info := geo.Lookup(endpoints[i].IP)
			endpoints[i].Country = info.Country
			endpoints[i].ASN = info.ASN
		}
	}
	protocols, err := st.ProtocolTotals(since)
	if err != nil {
		return dataParts{}, err
	}
	processes, err := st.TopProcessesSince(since, 10)
	if err != nil {
		return dataParts{}, err
	}
	domains, err := st.TopDomainsSince(since, 15)
	if err != nil {
		return dataParts{}, err
	}

	totalGB := (totals.AvgBpsIn + totals.AvgBpsOut) * float64(totals.Seconds) / 8 / 1e9

	marshal := func(v any) string {
		out, err := json.MarshalIndent(v, "", " ")
		if err != nil {
			return "{}"
		}
		return string(out)
	}

	summary := map[string]any{
		"donem_dakika":          minutes,
		"ornek_sayisi":          totals.Samples,
		"ortalama_indirme_bps":  round(totals.AvgBpsIn),
		"ortalama_gonderme_bps": round(totals.AvgBpsOut),
		"zirve_indirme_bps":     round(totals.PeakBpsIn),
		"zirve_gonderme_bps":    round(totals.PeakBpsOut),
		"toplam_transfer_gb":    round(totalGB),
		"protokol_dagilimi":     protocols,
	}
	endpointsData := map[string]any{
		"en_yogun_hedefler": endpoints,
	}
	processesData := map[string]any{
		"en_aktif_surecler": processes,
	}
	dnsData := map[string]any{
		"en_cok_sorgulanan_domainler": domains,
	}

	var parts dataParts
	parts.sections = append(parts.sections,
		struct{ title, data string }{"Trafik Ozeti ve Protokoller", marshal(summary)},
		struct{ title, data string }{"En Yogun Hedefler", marshal(endpointsData)},
		struct{ title, data string }{"En Aktif Surecler", marshal(processesData)},
		struct{ title, data string }{"DNS Sorgulari", marshal(dnsData)},
	)
	return parts, nil
}

func round(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
