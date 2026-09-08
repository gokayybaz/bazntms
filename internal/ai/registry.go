package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Registry, yapilandirilmis AI saglayicilarini yonetir ve onlara adaptor kurar.
// store (ai_providers) + vault crypter'a baglanir. Sohbet gecmisi yonetimi ve
// mesaj kaliciligi server katmaninda (internal/server/ai.go) — Registry yalniz
// saglayici cozumleme + egress kapisi + tek-atislik analiz sunar. aijob
// (gecelik) ve triage da bunu kullanir.
//
// context.go / openai.go / anthropic.go store import ETMEZ; yalniz bu dosya.

// Crypter, API anahtarlarini sifreler/cozer (vault.Vault saglar).
type Crypter interface {
	Encrypt(plain string) (string, error)
	Decrypt(enc string) (string, error)
}

// Config, hub AI ayarlari (config.HubConfig.AI'dan turer).
type Config struct {
	Enabled           bool
	AllowCloud        bool  // false → loopback/ozel-ag disi base URL reddi
	MaxContextKB      int   // 0 → 24
	DefaultProviderID int64 // 0 → ilk etkin saglayici
	RedactContext     bool
}

// Registry.
type Registry struct {
	st  store.AIStore
	cr  Crypter
	cfg func() Config
	hc  *http.Client
}

func NewRegistry(st store.AIStore, cr Crypter, cfg func() Config) *Registry {
	return &Registry{st: st, cr: cr, cfg: cfg, hc: &http.Client{Timeout: httpTimeout}}
}

func (r *Registry) Enabled() bool { return r.cfg().Enabled }

// Cfg, guncel AI ayarlari.
func (r *Registry) Cfg() Config { return r.cfg() }

// Store, alta yatan AIStore (server handler'lari sohbet CRUD icin kullanir).
func (r *Registry) Store() store.AIStore { return r.st }

// decode, saklanan bir kaydi cozulmus API anahtariyla ai.Provider'a cevirir.
func (r *Registry) decode(rec store.AIProvider) (Provider, error) {
	key := rec.APIKeyEnc
	if key != "" && r.cr != nil {
		dec, err := r.cr.Decrypt(key)
		if err != nil {
			return Provider{}, fmt.Errorf("saglayici %q anahtari cozulemedi: %w", rec.Name, err)
		}
		key = dec
	}
	var opts Opts
	if rec.OptsJSON != "" {
		_ = json.Unmarshal([]byte(rec.OptsJSON), &opts)
	}
	return Provider{
		ID: rec.ID, Name: rec.Name, Kind: Kind(rec.Kind), BaseURL: rec.BaseURL,
		APIKey: key, DefaultModel: rec.DefaultModel, Opts: opts, Enabled: rec.Enabled,
	}, nil
}

// Provider, id ile cozulmus saglayici (API anahtari duz). id=0 → varsayilan.
func (r *Registry) Provider(id int64) (Provider, error) {
	if id == 0 {
		return r.DefaultProvider()
	}
	rec, err := r.st.AIProviderByID(id)
	if err != nil {
		return Provider{}, err
	}
	if rec == nil {
		return Provider{}, fmt.Errorf("saglayici bulunamadi: %d", id)
	}
	return r.decode(*rec)
}

// DefaultProvider, yapilandirilmis varsayilan ya da ilk etkin saglayici.
func (r *Registry) DefaultProvider() (Provider, error) {
	list, err := r.st.ListAIProviders()
	if err != nil {
		return Provider{}, err
	}
	want := r.cfg().DefaultProviderID
	var firstEnabled *store.AIProvider
	for i := range list {
		if want != 0 && list[i].ID == want {
			return r.decode(list[i])
		}
		if firstEnabled == nil && list[i].Enabled {
			firstEnabled = &list[i]
		}
	}
	if firstEnabled == nil {
		return Provider{}, fmt.Errorf("etkin AI saglayicisi yok — Yonetim > AI Saglayici'dan ekleyin")
	}
	return r.decode(*firstEnabled)
}

// checkEgress, ai.allow_cloud=false iken loopback/ozel-ag disi adresi reddeder.
func (r *Registry) checkEgress(p Provider) error {
	if r.cfg().AllowCloud {
		return nil
	}
	if !IsLocalURL(p.BaseURL) {
		return fmt.Errorf("ai.allow_cloud kapali — yalnizca yerel model adreslerine izin var (%s reddedildi)", p.BaseURL)
	}
	return nil
}

// Adapter, bir saglayici icin (egress kapisindan gecmis) adaptor dondurur.
func (r *Registry) Adapter(id int64) (Adapter, Provider, error) {
	p, err := r.Provider(id)
	if err != nil {
		return nil, Provider{}, err
	}
	if !p.Ready() {
		return nil, p, fmt.Errorf("saglayici %q hazir degil (kapali ya da anahtar eksik)", p.Name)
	}
	if err := r.checkEgress(p); err != nil {
		return nil, p, err
	}
	return AdapterFor(p, r.hc), p, nil
}

// --- saglayici yonetimi (server PermGlobalAdmin handler'lari cagirir) ---

// SaveProvider, bir saglayici kaydi olusturur/gunceller. plainKey bos ve
// id!=0 ise mevcut anahtar korunur (kullanici formu bos birakti).
func (r *Registry) SaveProvider(rec store.AIProvider, plainKey string) (int64, error) {
	if !Kind(rec.Kind).Valid() {
		return 0, fmt.Errorf("gecersiz saglayici turu: %q", rec.Kind)
	}
	if rec.BaseURL == "" {
		rec.BaseURL = DefaultBaseURL(Kind(rec.Kind))
	}
	rec.BaseURL = strings.TrimRight(strings.TrimSpace(rec.BaseURL), "/")
	// egress kilidi kayıt anında da uygulanır (S26.20): ai.allow_cloud=false
	// iken loopback/özel-ağ dışı bir adres kaydedilemez.
	if !r.cfg().AllowCloud && rec.Enabled && !IsLocalURL(rec.BaseURL) {
		return 0, fmt.Errorf("ai.allow_cloud kapalı — yalnızca yerel model adresleri kabul edilir (%s reddedildi)", rec.BaseURL)
	}
	if plainKey != "" {
		if r.cr == nil {
			return 0, fmt.Errorf("vault yok — API anahtari sifrelenemez")
		}
		enc, err := r.cr.Encrypt(plainKey)
		if err != nil {
			return 0, err
		}
		rec.APIKeyEnc = enc
	} else {
		rec.APIKeyEnc = "" // update: dokunma
	}
	if rec.ID == 0 {
		return r.st.CreateAIProvider(rec)
	}
	return rec.ID, r.st.UpdateAIProvider(rec)
}

// TestResult, bir saglayici baglanti testinin sonucu.
type TestResult struct {
	OK        bool     `json:"ok"`
	LatencyMS int64    `json:"latency_ms"`
	Models    []string `json:"models,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// TestProvider, saglayiciya kucuk bir istek atar: once model listesi, o
// desteklenmezse tek satirlik bir completion.
func (r *Registry) TestProvider(ctx context.Context, id int64) TestResult {
	ad, p, err := r.Adapter(id)
	if err != nil {
		return TestResult{Error: err.Error()}
	}
	start := time.Now()
	models, merr := ad.Models(ctx)
	if merr == nil {
		return TestResult{OK: true, LatencyMS: time.Since(start).Milliseconds(), Models: models}
	}
	model := p.DefaultModel
	if model == "" && len(models) > 0 {
		model = models[0]
	}
	_, _, cerr := ad.Complete(ctx, ChatRequest{
		Model: model, MaxTokens: 16, Temperature: 0,
		Messages: []Message{{Role: RoleUser, Content: "ping"}},
	})
	if cerr != nil {
		return TestResult{Error: cerr.Error(), LatencyMS: time.Since(start).Milliseconds()}
	}
	return TestResult{OK: true, LatencyMS: time.Since(start).Milliseconds()}
}

// Models, bir saglayicinin canli model listesi (dropdown doldurma).
func (r *Registry) Models(ctx context.Context, id int64) ([]string, error) {
	ad, _, err := r.Adapter(id)
	if err != nil {
		return nil, err
	}
	return ad.Models(ctx)
}
