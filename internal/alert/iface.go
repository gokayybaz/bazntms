package alert

// Arayüz kapasite/kullanım uyarısı (Faz 23-C): SNMP arayüz verimi güvenilir
// hızla (ifSpeed / ifHighSpeed) karşılaştırılır. Bir arayüz yön (rx/tx) bazında
// eşiği SustainSec boyunca (kontrol 60 sn'de bir → SustainSec/60 ardışık geçiş)
// aşarsa "iface_util" uyarısı üretilir. Loopback/tünel sınıfları atlanır
// (yanıltıcı). Kullanım altına düşünce sayaç sıfırlanır → mevcut yaşam
// döngüsü otomatik-çözülme olayı kapatır.

import (
	"fmt"
	"strings"

	"github.com/gokayybaz/bazntms/internal/store"
)

// IfaceConfig, arayüz kullanım uyarısı ayarları (config JSON'da "iface").
type IfaceConfig struct {
	Enabled    bool    `json:"enabled"`
	WarnPct    float64 `json:"warn_pct"`    // uyarı eşiği (%), vars. 70
	CritPct    float64 `json:"crit_pct"`    // kritik eşiği (%), vars. 90
	SustainSec int     `json:"sustain_sec"` // eşik bu süre boyunca aşılmalı, vars. 300
}

func DefaultIfaceConfig() IfaceConfig {
	return IfaceConfig{Enabled: true, WarnPct: 70, CritPct: 90, SustainSec: 300}
}

// (eksik "iface" bölümü NormalizeConfig'te varsayılanla doldurulur — anomaly.go)

// checkIfaceUtil, dakikada bir çağrılır (Manager.run).
func (m *Manager) checkIfaceUtil(cfg Config) {
	ic := cfg.Iface
	if !ic.Enabled || (ic.WarnPct <= 0 && ic.CritPct <= 0) {
		m.ifaceOver = map[string]int{}
		return
	}
	need := ic.SustainSec / 60
	if need < 1 {
		need = 1
	}
	warnThr := ic.WarnPct
	if warnThr <= 0 {
		warnThr = ic.CritPct
	}

	devices, err := m.st.ListDevices("")
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, d := range devices {
		if !d.Enabled {
			continue
		}
		ifaces, err := m.st.LatestDeviceIfaces(d.ID)
		if err != nil {
			continue
		}
		for _, r := range ifaces {
			if store.IfaceAlertSkip[r.Class] || r.SpeedBitsPS == 0 || r.OperStatus != 1 {
				continue
			}
			for _, dir := range [2]struct {
				name string
				pct  float64
			}{{"rx", r.RxUtilPct}, {"tx", r.TxUtilPct}} {
				if dir.pct < 0 {
					continue
				}
				key := fmt.Sprintf("%d|%d|%s", d.ID, r.IfIndex, dir.name)
				seen[key] = true
				if dir.pct < warnThr {
					delete(m.ifaceOver, key)
					continue
				}
				m.ifaceOver[key]++
				if m.ifaceOver[key] != need {
					continue
				}
				sev := "warn"
				if ic.CritPct > 0 && dir.pct >= ic.CritPct {
					sev = "crit"
				}
				label := r.Name
				if label == "" {
					label = fmt.Sprintf("if%d", r.IfIndex)
				}
				m.fireCtx("iface_util", key,
					fmt.Sprintf("%s %s (%s) kullanımı %d dk boyunca %%%.0f — eşik %%%.0f",
						d.Name, label, strings.ToUpper(dir.name), ic.SustainSec/60, dir.pct, warnThr),
					fireOpts{Site: d.Site, Severity: sev})
			}
		}
	}
	// cihaz silindi / arayüz kayboldu → sayaçları temizle
	for k := range m.ifaceOver {
		if !seen[k] {
			delete(m.ifaceOver, k)
		}
	}
}
