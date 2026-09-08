package alert

// IOC / tehdit istihbaratı eşleştirmesi: agent'ların gözlemlediği L7 (TLS SNI /
// HTTP Host) ve DNS alan adları bir kara listeye bakılır. İmza tabanlı tam DPI
// yerine düşük maliyetli "bilinen kötü domain'e temas" tespiti.

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gokayybaz/bazntms/internal/threatintel"
)

// IOCConfig, tehdit istihbaratı kontrolü ayarları (config JSON'da "ioc").
type IOCConfig struct {
	Enabled bool `json:"enabled"`
}

func DefaultIOCConfig() IOCConfig { return IOCConfig{Enabled: true} }

// SetThreatIntel, tehdit istihbaratı servisini takar (Faz 24-E — sağlayıcı-
// bağımsız; -ioc-file verildiyse localfile sağlayıcısıyla). nil → IOC kontrolü
// pasif.
func (m *Manager) SetThreatIntel(ti *threatintel.Service) {
	m.mu.Lock()
	m.ti = ti
	m.mu.Unlock()
}

// checkIOC, ~30 sn'de bir çağrılır (Manager.run). Son telemetri penceresindeki
// L7/DNS alan adlarını tehdit istihbaratına sorar; suspicious/malicious başına
// (agent, domain) anahtarıyla cooldown'a tabi bir "ioc" uyarısı üretir.
// **Oto-blok yok** — yalnızca uyarı; incident motoru (24-B) bunu korele eder.
func (m *Manager) checkIOC(cfg Config) {
	if !cfg.IOC.Enabled {
		return
	}
	m.mu.Lock()
	ti := m.ti
	m.mu.Unlock()
	if ti == nil || !ti.Enabled() {
		return
	}

	window := time.Duration(3*m.telemetryInterval) * time.Second
	seen, err := m.st.RecentAgentDomains(time.Now().Add(-window))
	if err != nil {
		slog.Debug("IOC: RecentAgentDomains hatası", "err", err)
		return
	}
	for _, s := range seen {
		ind := ti.Domain(s.Domain)
		if !ind.Bad() {
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
		if ind.RawRef != "" && ind.RawRef != s.Domain {
			match = fmt.Sprintf("%s (kural: %s)", s.Domain, ind.RawRef)
		}
		sev := "crit"
		if ind.Reputation == threatintel.Suspicious {
			sev = "warn"
		}
		m.fireCtx("ioc",
			fmt.Sprintf("%d|%s", s.AgentID, s.Domain),
			fmt.Sprintf("Tehdit eşleşmesi [%s/%s]: %s — agent %s, süreç %s, kaynak %s",
				ind.Reputation, ind.Source, match, s.AgentName, proc, src),
			fireOpts{AgentID: s.AgentID, Severity: sev})
	}
}
