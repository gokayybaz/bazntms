package alert

// FortiGate uyarıları (Faz 8.5): VPN tünel düşüşü, SD-WAN SLA ihlali ve
// oturum eşiği. Mevcut cooldown + bildirim kanalları otomatik kullanılır.

import (
	"fmt"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// FortiAlertConfig, FortiGate uyarı ayarları (config JSON'da "forti").
// Sıfır eşikler = kapalı; VPNDown legacy configlerde varsayılanla açılır.
type FortiAlertConfig struct {
	VPNDown        bool    `json:"vpn_down"`
	SDWANLatencyMs float64 `json:"sdwan_latency_ms"`
	SDWANJitterMs  float64 `json:"sdwan_jitter_ms"`
	SDWANLossPct   float64 `json:"sdwan_loss_pct"`
	MaxSessions    int     `json:"max_sessions"`
}

func DefaultFortiAlertConfig() FortiAlertConfig {
	return FortiAlertConfig{
		VPNDown:        true,
		SDWANLatencyMs: 200,
		SDWANJitterMs:  50,
		SDWANLossPct:   5,
		MaxSessions:    0, // eşik cihaz modeline göre değişir: varsayılan kapalı
	}
}

// NormalizeFortiConfig, eski JSON configlerinde eksik "forti" bölümünü
// varsayılanla doldurur.
func NormalizeFortiConfig(cfg Config) Config {
	if cfg.Forti == (FortiAlertConfig{}) {
		cfg.Forti = DefaultFortiAlertConfig()
	}
	return cfg
}

// checkForti, her dakika çağrılır (Manager.run'dan).
func (m *Manager) checkForti(cfg Config) {
	fa := cfg.Forti
	if !fa.VPNDown && fa.SDWANLatencyMs <= 0 && fa.SDWANJitterMs <= 0 &&
		fa.SDWANLossPct <= 0 && fa.MaxSessions <= 0 {
		return
	}

	// 1) VPN tünelleri down
	if fa.VPNDown {
		if downs, err := m.st.FortiVPNsDown(10 * time.Minute); err == nil {
			for _, r := range downs {
				m.fire("vpn_down",
					fmt.Sprintf("%d|%s|%s", r.DeviceID, r.Kind, r.Name),
					fmt.Sprintf("VPN down: %s (%s, vdom %s, peer %s) — cihaz %s",
						r.Name, r.Kind, orEmpty(r.VDOM), orEmpty(r.Peer), r.DeviceName))
			}
		}
	}

	// 2) SD-WAN SLA ihlali — her (cihaz, health-check, member) için en güncel örnek
	if fa.SDWANLatencyMs > 0 || fa.SDWANJitterMs > 0 || fa.SDWANLossPct > 0 {
		if rows, err := m.st.RecentFortiSDWANAll(time.Now().Add(-10 * time.Minute)); err == nil {
			latest := map[string]store.SDWANRow{}
			for _, r := range rows {
				key := fmt.Sprintf("%d|%s|%s", r.DeviceID, r.HealthCheck, r.Member)
				if cur, ok := latest[key]; !ok || r.Ts > cur.Ts {
					latest[key] = r
				}
			}
			for key, r := range latest {
				var reasons []string
				if fa.SDWANLatencyMs > 0 && r.LatencyMs >= fa.SDWANLatencyMs {
					reasons = append(reasons, fmt.Sprintf("gecikme %.0f ms", r.LatencyMs))
				}
				if fa.SDWANJitterMs > 0 && r.JitterMs >= fa.SDWANJitterMs {
					reasons = append(reasons, fmt.Sprintf("jitter %.0f ms", r.JitterMs))
				}
				if fa.SDWANLossPct > 0 && r.PacketLossPct >= fa.SDWANLossPct {
					reasons = append(reasons, fmt.Sprintf("paket kaybı %.1f%%", r.PacketLossPct))
				}
				if len(reasons) == 0 {
					continue
				}
				msg := fmt.Sprintf("SD-WAN SLA ihlali: %s [%s] (%s) — %s — cihaz %s",
					r.Member, r.HealthCheck, r.State, joinReasons(reasons), r.DeviceName)
				m.fire("sdwan_sla_breach", key, msg)
			}
		}
	}

	// 3) Oturum eşiği — her cihaz için en güncel örnek
	if fa.MaxSessions > 0 {
		if rows, err := m.st.RecentDeviceResourcesAll(time.Now().Add(-10 * time.Minute)); err == nil {
			latest := map[string]store.ResourceRow{}
			for _, r := range rows {
				key := fmt.Sprint(r.DeviceID)
				if cur, ok := latest[key]; !ok || r.Ts > cur.Ts {
					latest[key] = r
				}
			}
			for _, r := range latest {
				if r.Sessions >= int64(fa.MaxSessions) {
					m.fire("high_sessions", fmt.Sprint(r.DeviceID),
						fmt.Sprintf("Oturum sayısı eşiğin üzerinde: %d ≥ %d — cihaz %s",
							r.Sessions, fa.MaxSessions, r.DeviceName))
				}
			}
		}
	}
}

func orEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func joinReasons(reasons []string) string {
	out := ""
	for i, r := range reasons {
		if i > 0 {
			out += ", "
		}
		out += r
	}
	return out
}
