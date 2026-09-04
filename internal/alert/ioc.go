package alert

// IOC / tehdit istihbaratı eşleştirmesi: agent'ların gözlemlediği L7 (TLS SNI /
// HTTP Host) ve DNS alan adları bir kara listeye bakılır. İmza tabanlı tam DPI
// yerine düşük maliyetli "bilinen kötü domain'e temas" tespiti.

import (
	"fmt"
	"log/slog"
	"time"
)

// IOCConfig, IOC kontrolü ayarları (config JSON'da "ioc").
type IOCConfig struct {
	Enabled bool `json:"enabled"`
}

func DefaultIOCConfig() IOCConfig { return IOCConfig{Enabled: true} }

// IOCMatcher, bir alan adının (veya üst alanının) kara listede olup olmadığını
// söyler. internal/ioc.List bunu uygular; hub'da -ioc-file verilmemişse nil.
type IOCMatcher interface {
	Match(domain string) (rule string, hit bool)
	Count() int
}

// SetIOC, eşleştiriciyi takar (hub açılışında -ioc-file yüklendiyse).
func (m *Manager) SetIOC(x IOCMatcher) {
	m.mu.Lock()
	m.ioc = x
	m.mu.Unlock()
}

// checkIOC, ~30 sn'de bir çağrılır (Manager.run). Son telemetri penceresindeki
// L7/DNS alan adlarını IOC listesine karşı eşleştirir; eşleşme başına
// (agent, domain) anahtarıyla cooldown'a tabi bir "ioc" uyarısı üretir.
func (m *Manager) checkIOC(cfg Config) {
	if !cfg.IOC.Enabled {
		return
	}
	m.mu.Lock()
	ioc := m.ioc
	m.mu.Unlock()
	if ioc == nil || ioc.Count() == 0 {
		return
	}

	window := time.Duration(3*m.telemetryInterval) * time.Second
	seen, err := m.st.RecentAgentDomains(time.Now().Add(-window))
	if err != nil {
		slog.Debug("IOC: RecentAgentDomains hatası", "err", err)
		return
	}
	slog.Debug("IOC taraması", "domain_gozlem", len(seen), "liste", ioc.Count())
	for _, s := range seen {
		rule, hit := ioc.Match(s.Domain)
		if !hit {
			continue
		}
		src := "TLS SNI / HTTP Host"
		if s.Source == "dns" {
			src = "DNS sorgusu"
		}
		proc := s.Process
		if proc == "" {
			proc = "?"
		}
		match := s.Domain
		if rule != "" && rule != s.Domain {
			match = fmt.Sprintf("%s (kural: %s)", s.Domain, rule)
		}
		m.fire("ioc",
			fmt.Sprintf("%d|%s", s.AgentID, s.Domain),
			fmt.Sprintf("IOC eşleşmesi: %s — agent %s, süreç %s, kaynak %s",
				match, s.AgentName, proc, src))
	}
}
