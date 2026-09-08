// Package threatintel, sağlayıcı-bağımsız tehdit istihbaratı adaptörüdür
// (Faz 24-E). Gözlemlenen uzak IP / alan adları bir veya daha çok sağlayıcıya
// sorulur; sonuç normalize edilir ve önbelleklenir.
//
// **OTO-BLOK YOK** — bazNTMS observability-first. Kötücül bir hedef görülürse
// yalnızca uyarı/olay üretilir; incident motoru bunu korele eder.
package threatintel

import (
	"net"
	"strings"
	"sync"
	"time"
)

// Reputation, normalize edilmiş itibar.
type Reputation string

const (
	Trusted    Reputation = "trusted"
	Neutral    Reputation = "neutral"
	Suspicious Reputation = "suspicious"
	Malicious  Reputation = "malicious"
	Unknown    Reputation = "unknown"
)

func repRank(r Reputation) int {
	switch r {
	case Malicious:
		return 4
	case Suspicious:
		return 3
	case Trusted:
		return 2
	case Neutral:
		return 1
	default:
		return 0
	}
}

// Indicator, bir IP / alan adının normalize edilmiş tehdit görünümü.
type Indicator struct {
	Indicator  string     `json:"indicator"`
	Type       string     `json:"type"` // ip | domain
	Reputation Reputation `json:"reputation"`
	Confidence int        `json:"confidence"` // 0-100
	Categories []string   `json:"categories,omitempty"`
	Source     string     `json:"source,omitempty"`
	FirstSeen  int64      `json:"first_seen,omitempty"`
	LastSeen   int64      `json:"last_seen,omitempty"`
	RawRef     string     `json:"raw_ref,omitempty"`
}

// Bad, itibar suspicious ya da malicious mı (uyarı üretmeye değer).
func (i Indicator) Bad() bool { return i.Reputation == Suspicious || i.Reputation == Malicious }

// Provider, tek bir tehdit istihbaratı kaynağı.
type Provider interface {
	Name() string
	// Lookup, verilen göstergeyi (typ: "ip" | "domain") sorar. ok=false →
	// sağlayıcı bu gösterge hakkında bilgi taşımıyor (Unknown).
	Lookup(indicator, typ string) (Indicator, bool)
}

type cached struct {
	ind Indicator
	at  time.Time
}

// Service, sağlayıcıları sırayla sorgular ve en şiddetli itibarı seçer;
// sonuçları TTL ile önbellekler.
type Service struct {
	providers []Provider
	ttl       time.Duration

	mu    sync.RWMutex
	cache map[string]cached
}

const cacheMax = 50_000

// New, servisi kurar. ttl <= 0 → 1 saat.
func New(ttl time.Duration, providers ...Provider) *Service {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Service{providers: providers, ttl: ttl, cache: map[string]cached{}}
}

// Providers, kayıtlı sağlayıcı adları.
func (s *Service) Providers() []string {
	out := make([]string, len(s.providers))
	for i, p := range s.providers {
		out[i] = p.Name()
	}
	return out
}

// Enabled, en az bir sağlayıcı var mı.
func (s *Service) Enabled() bool { return len(s.providers) > 0 }

// IP, bir IP'yi sorar. Özel/loopback IP'ler her zaman Unknown.
func (s *Service) IP(ip string) Indicator {
	if p := net.ParseIP(ip); p == nil || p.IsPrivate() || p.IsLoopback() || p.IsLinkLocalUnicast() {
		return Indicator{Indicator: ip, Type: "ip", Reputation: Unknown}
	}
	return s.lookup(ip, "ip")
}

// Domain, bir alan adını sorar.
func (s *Service) Domain(domain string) Indicator {
	d := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if d == "" || !strings.Contains(d, ".") {
		return Indicator{Indicator: domain, Type: "domain", Reputation: Unknown}
	}
	return s.lookup(d, "domain")
}

func (s *Service) lookup(indicator, typ string) Indicator {
	key := typ + "|" + indicator
	s.mu.RLock()
	c, ok := s.cache[key]
	s.mu.RUnlock()
	if ok && time.Since(c.at) < s.ttl {
		return c.ind
	}

	best := Indicator{Indicator: indicator, Type: typ, Reputation: Unknown}
	for _, p := range s.providers {
		ind, hit := p.Lookup(indicator, typ)
		if !hit {
			continue
		}
		ind.Indicator, ind.Type = indicator, typ
		if ind.Source == "" {
			ind.Source = p.Name()
		}
		if repRank(ind.Reputation) > repRank(best.Reputation) {
			best = ind
		}
	}

	s.mu.Lock()
	if len(s.cache) >= cacheMax {
		s.cache = map[string]cached{}
	}
	s.cache[key] = cached{ind: best, at: time.Now()}
	s.mu.Unlock()
	return best
}
