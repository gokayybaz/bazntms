// Package ai, hub'in AI analiz altyapisidir (Faz 26). Monolit doneminde
// (d92d0fb:internal/ai) OpenAI-uyumlu tek bir istemci vardi; 9d22e7a'da "olu
// altsistem" olarak silindi. Bu paket onu geri getirir ve cok-saglayicili
// (yerel + bulut), streaming'li, kalici sohbetli bir modele yukseltir.
//
// Sinir: AI **danismandir**. Arac cagirmaz, durum degistirmez; deterministik
// motorlar (anomali, incident, health, recommend) yetkili kalir. Bkz.
// docs/decisions/0014-ai-analysis.md.
package ai

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Role, sohbet mesaji rolu.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message, tek bir sohbet mesaji.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest, tek bir tamamlama/akis istegi.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	NoThink     bool // Qwen3 tarzi modellerde dusunmeyi kapatir (/no_think)
}

// Usage, model tarafindan raporlanan token kullanimi (varsa).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Delta, streaming yanit parcasi. Err dolu ise akis biter; Done true ise
// normal bitis (Usage o an dolabilir).
type Delta struct {
	Text  string
	Usage *Usage
	Done  bool
	Err   error
}

// Kind, saglayici turu. openai/ollama/lmstudio/openai-compat hepsi ayni
// OpenAI-uyumlu tel bicimini konusur; anthropic native /v1/messages kullanir.
type Kind string

const (
	KindOpenAI       Kind = "openai"
	KindAnthropic    Kind = "anthropic"
	KindOllama       Kind = "ollama"
	KindLMStudio     Kind = "lmstudio"
	KindOpenAICompat Kind = "openai-compat"
)

// Valid, bilinen bir saglayici turu mu.
func (k Kind) Valid() bool {
	switch k {
	case KindOpenAI, KindAnthropic, KindOllama, KindLMStudio, KindOpenAICompat:
		return true
	}
	return false
}

// DefaultBaseURL, tur icin makul varsayilan taban adres (kayit sihirbazi
// bos birakilirsa). Bos = kullanici girmeli.
func DefaultBaseURL(k Kind) string {
	switch k {
	case KindOpenAI:
		return "https://api.openai.com/v1"
	case KindAnthropic:
		return "https://api.anthropic.com/v1"
	case KindOllama:
		return "http://localhost:11434/v1"
	case KindLMStudio:
		return "http://localhost:1234/v1"
	default:
		return ""
	}
}

// Opts, saglayici basina ek ayarlar (ai_providers.opts_json).
type Opts struct {
	NoThink     bool              `json:"no_think,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"` // ek istek basliklari (ör. OpenAI-Organization)
}

// Provider, bir yapilandirilmis AI saglayicisi (API anahtari COZULMUS halde).
type Provider struct {
	ID           int64
	Name         string
	Kind         Kind
	BaseURL      string
	APIKey       string
	DefaultModel string
	Opts         Opts
	Enabled      bool
}

// NeedsKey, bu saglayicinin bir API anahtari gerektirip gerektirmedigi.
// Yerel adresler (loopback / ozel ag) anahtarsiz calisir.
func (p Provider) NeedsKey() bool { return !IsLocalURL(p.BaseURL) }

// Ready, saglayicinin istek atmaya hazir olup olmadigi.
func (p Provider) Ready() bool {
	if !p.Enabled || p.BaseURL == "" || !p.Kind.Valid() {
		return false
	}
	return p.APIKey != "" || !p.NeedsKey()
}

// Adapter, tek bir saglayiciya karsi model cagrilari. openaiAdapter tum
// OpenAI-uyumlu uclari, anthropicAdapter native Messages API'yi saglar.
type Adapter interface {
	// Complete, tek parca yanit dondurur (streaming'siz).
	Complete(ctx context.Context, req ChatRequest) (string, Usage, error)
	// Stream, yanit parcalarini bir kanaldan yayar; kanal kapaninca akis biter.
	Stream(ctx context.Context, req ChatRequest) (<-chan Delta, error)
	// Models, saglayicidan mevcut model kimliklerini ceker.
	Models(ctx context.Context) ([]string, error)
	// Kind, adaptorun turu.
	Kind() Kind
}

// httpTimeout, reasoning modelleri yavas olabilir — uzun timeout.
const httpTimeout = 5 * time.Minute

// AdapterFor, saglayici turune gore uygun adaptoru kurar. hc nil ise
// dahili uzun-timeout'lu istemci kullanilir.
func AdapterFor(p Provider, hc *http.Client) Adapter {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if p.Kind == KindAnthropic {
		return &anthropicAdapter{base: base, key: p.APIKey, opts: p.Opts, hc: hc}
	}
	return &openaiAdapter{base: base, key: p.APIKey, opts: p.Opts, hc: hc}
}

// IsLocalURL, adresin loopback ya da ozel ag (RFC1918 / ULA / link-local) olup
// olmadigini soyler. ai.allow_cloud=false iken egress kilidi + "anahtar
// gerekmez" isareti bunu kullanir.
func IsLocalURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") || lower == "host.docker.internal" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// cozulmemis ad — cozup bak (kisa; kayit/dogrulama yolunda kabul edilir)
		ips, lerr := net.LookupIP(host)
		if lerr != nil || len(ips) == 0 {
			return false
		}
		for _, r := range ips {
			if !isPrivateIP(r) {
				return false
			}
		}
		return true
	}
	return isPrivateIP(ip)
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// stripThink, <think>...</think> dusunme bloklarini cevaptan ayiklar
// (DeepSeek-R1, Qwen3 vb.). d92d0fb'den birebir tasindi.
func stripThink(s string) string {
	lower := strings.ToLower(s)
	for {
		start := strings.Index(lower, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(lower[start:], "</think>")
		if end < 0 {
			// kapanmamis blok: baslangictan sonrasini at
			return strings.TrimSpace(strings.TrimSuffix(s[:start], "</think>"))
		}
		end += start + len("</think>")
		s = s[:start] + s[end:]
		lower = strings.ToLower(s)
	}
	return strings.TrimSpace(s)
}
