package alert

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/sysmon"
)

// Config, tum uyarı sistemi ayarlari; UI'dan duzenlenebilir ve SQLite'ta
// JSON olarak saklanir.
type Config struct {
	Enabled     bool `json:"enabled"`
	CooldownMin int  `json:"cooldown_min"`

	// S22.8 — otomatik çözülme. Açık bir olay AutoResolveMin dakika boyunca
	// yinelenmezse (bump gelmezse) motor "resolved" işaretler. < 0 → kapalı.
	// NotifyResolve, çözülme bildirimi gönderilsin mi.
	AutoResolveMin int  `json:"auto_resolve_min"`
	NotifyResolve  bool `json:"notify_resolve"`

	// S22.9 — korelasyon. Aynı sahada CorrelateWindowSec içinde ateşlenen
	// olaylar ortak group_id alır (panelde tek kök-neden). 0/<0 → kapalı.
	CorrelateWindowSec int `json:"correlate_window_sec"`

	Bandwidth BandwidthConfig  `json:"bandwidth"`
	Ports     PortsConfig      `json:"ports"`
	NewProc   ProcConfig       `json:"new_proc"`
	NewTarget TargetConfig     `json:"new_target"`
	Anomaly   AnomalyConfig    `json:"anomaly"` // Faz 6.2: istatistiksel baseline
	Forti     FortiAlertConfig `json:"forti"`   // Faz 8.5: vpn/sdwan/oturum eşikleri
	IOC       IOCConfig        `json:"ioc"`     // Faz 6.6: tehdit istihbaratı domain eşleştirmesi

	// Severities, eşik-tabanlı uyarı türleri için operatör önem geçersiz
	// kılması (S22.7): kind → "info"|"warn"|"crit". Anahtar yoksa kindSeverity
	// varsayılanı geçerli. Anomali önemi z-büyüklüğünden türetilir (bkz.
	// AnomalyConfig.CritZ) — bu harita onu etkilemez.
	Severities map[string]string `json:"severities,omitempty"`

	Notifiers Notifiers `json:"notifiers"`
}

// severityFor, bir uyarı türünün etkin önemi: önce operatör geçersiz kılması
// (Config.Severities), sonra kindSeverity varsayılanı.
func (c Config) severityFor(kind string) string {
	if s, ok := c.Severities[kind]; ok && validSeverity(s) {
		return s
	}
	return severityForKind(kind)
}

func validSeverity(s string) bool { return s == "info" || s == "warn" || s == "crit" }

type BandwidthConfig struct {
	Enabled bool    `json:"enabled"`
	InMbps  float64 `json:"in_mbps"`
	OutMbps float64 `json:"out_mbps"`
	Seconds int     `json:"seconds"` // esik kac saniye ust uste asilirsa
}

type PortsConfig struct {
	Enabled bool  `json:"enabled"`
	Ports   []int `json:"ports"`
}

type ProcConfig struct {
	Enabled bool     `json:"enabled"`
	Ignore  []string `json:"ignore"` // bildirim uretilmeyecek surec adlari
}

type TargetConfig struct {
	Enabled    bool    `json:"enabled"`
	MinTotalMB float64 `json:"min_total_mb"` // yeni hedef icin minimum toplam transfer
}

type Notifiers struct {
	Desktop        bool   `json:"desktop"`
	GenericURL     string `json:"generic_url"`
	DiscordURL     string `json:"discord_url"`
	SlackURL       string `json:"slack_url"`
	TelegramToken  string `json:"telegram_token"`
	TelegramChatID string `json:"telegram_chat_id"`

	// Faz 6.3: kurumsal entegrasyonlar
	TeamsURL        string   `json:"teams_url"`  // Teams incoming webhook
	EmailHost       string   `json:"email_host"` // SMTP sunucu (STARTTLS otomatik)
	EmailPort       int      `json:"email_port"` // 0 → 587
	EmailFrom       string   `json:"email_from"`
	EmailTo         []string `json:"email_to"`
	EmailUser       string   `json:"email_user"`
	EmailPass       string   `json:"email_pass"`
	WebhookV2URL    string   `json:"webhook_v2_url"` // imzali webhook (HMAC-SHA256)
	WebhookV2Secret string   `json:"webhook_v2_secret"`

	// Faz 6.5: SIEM/ITSM push connector (CEF/LEEF/JSON → syslog veya HTTP)
	SIEM SIEMConfig `json:"siem"`
}

// DefaultConfig, ilk calistirma icin makul ayarlar.
func DefaultConfig() Config {
	return Config{
		Enabled:            true,
		CooldownMin:        10,
		AutoResolveMin:     15,
		NotifyResolve:      true,
		CorrelateWindowSec: 120,
		Bandwidth: BandwidthConfig{
			Enabled: true, InMbps: 100, OutMbps: 50, Seconds: 10,
		},
		Ports: PortsConfig{
			Enabled: true, Ports: []int{23, 4444, 1337, 31337},
		},
		NewProc: ProcConfig{
			Enabled: true, Ignore: []string{"bazntms", "mDNSResponder", "rapportd", "ControlCenter"},
		},
		NewTarget: TargetConfig{
			Enabled: true, MinTotalMB: 10,
		},
		Anomaly:   DefaultAnomalyConfig(),
		Forti:     DefaultFortiAlertConfig(),
		IOC:       DefaultIOCConfig(),
		Notifiers: Notifiers{Desktop: true},
	}
}

// Manager, kurallari periyodik degerlendirir ve olaylari kaydedip bildirir.
type Manager struct {
	mu     sync.Mutex
	cfg    Config
	st     store.Store
	engine *capture.Engine

	bwInCount  int
	bwOutCount int
	lastFire   map[string]time.Time // key: kind|key -> cooldown

	// agentBw, her online agent icin ardisik bant genisligi esik-asimi
	// sayaci (checkAgentBandwidth) — anahtar agent ID.
	agentBw map[int64]*agentBwCounter

	// telemetryInterval, agent filosunun "online" penceresini hesaplamak
	// icin (bkz. server.go'daki ayni formul) — saniye.
	telemetryInterval int

	stopCh chan struct{}
	doneCh chan struct{}
	tickN  int

	notifier *Notifier
	ioc      IOCMatcher // -ioc-file yüklendiyse; nil ise IOC kontrolü pasif

	// isLeader, çoklu controller replikasında yalnız liderin değerlendirmesi
	// için (C1, Faz 15). nil → daima lider (tek replika / dev). Lider değilken
	// motor "sıcak" kalır ama hiçbir kural değerlendirmez.
	isLeader func() bool

	// silences, aktif bakım pencerelerinin önbelleği (S22.10) — run() döngüsü
	// periyodik, POST/DELETE sonrası RefreshSilences() ile tazeler.
	silenceMu sync.RWMutex
	silences  []store.AlertSilence
}

// SetLeaderCheck, değerlendirme öncesi çağrılacak liderlik denetimini bağlar.
func (m *Manager) SetLeaderCheck(fn func() bool) { m.isLeader = fn }

func (m *Manager) leading() bool { return m.isLeader == nil || m.isLeader() }

type agentBwCounter struct{ in, out int }

func NewManager(cfg Config, st store.Store, engine *capture.Engine, telemetryInterval int) *Manager {
	if telemetryInterval <= 0 {
		telemetryInterval = 30
	}
	return &Manager{
		cfg:               cfg,
		st:                st,
		engine:            engine,
		lastFire:          map[string]time.Time{},
		agentBw:           map[int64]*agentBwCounter{},
		telemetryInterval: telemetryInterval,
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		notifier:          NewNotifier(),
	}
}

func (m *Manager) Start() {
	go m.run()
}

func (m *Manager) Stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *Manager) run() {
	defer close(m.doneCh)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.Lock()
			m.tickN++
			consEvery := m.tickN%5 == 1
			cfg := m.cfg
			m.mu.Unlock()

			if !cfg.Enabled {
				continue
			}
			// bakım penceresi önbelleği (S22.10): liderlikten bağımsız — devir
			// anında güncel olsun. 20 sn'de bir + ilk tick.
			if m.tickN == 1 || m.tickN%20 == 5 {
				m.RefreshSilences()
			}
			// C1: çoklu replikada yalnız lider değerlendirir (alarm çift
			// ateşlenmesin). Lider değilken bant genişliği sayaçları sıfırlanır
			// ki devralınca geçmiş kalıntısıyla tetiklenmesin.
			if !m.leading() {
				m.bwInCount, m.bwOutCount = 0, 0
				continue
			}
			snap := m.engine.Snapshot()

			m.checkBandwidth(cfg, snap)
			if consEvery {
				cons := sysmon.ListConnections()
				m.checkPorts(cfg, cons)
				m.checkNewProcess(cfg, cons)
				m.checkNewTarget(cfg, snap)

				// agent filosu: hub'in kendi yerel yakalamasindan (yukarida)
				// AYRI olarak, uzak agent'lardan raporlanan baglanti/verim
				// verisi uzerinde de ayni port/surec/bant genisligi
				// kurallarini calistir (bkz. checkAgent* — "target" (yeni
				// hedef) kurali agent tarafinda henuz desteklenmiyor, cunku
				// agent'lar hub'in TopEndpoints'i gibi hedef-basi kumulatif
				// bayt toplami raporlamiyor).
				window := time.Duration(2*m.telemetryInterval) * time.Second
				if agents, err := m.st.ListAgents(window, ""); err == nil {
					m.checkAgentPorts(cfg, agents)
					m.checkAgentNewProcess(cfg, agents)
					m.checkAgentBandwidth(cfg, agents)
				}
			}
			// anomali baseline'i: lider-kapili saatlik rebuild (ilk tick'te de).
			// S22.1 — degerlendirme basina canli LAG taramasi yerine materyalize
			// anomaly_baseline tablosu.
			if cfg.Anomaly.Enabled && (m.tickN == 1 || m.tickN%3600 == 1) {
				m.rebuildAnomalyBaseline(cfg)
			}
			// anomali degerlendirmesi: 5 dakikada bir (Faz 6.2)
			if m.tickN%300 == 1 {
				m.checkAnomaly(cfg)
			}
			// FortiGate uyarilari: dakikada bir (Faz 8.5)
			if m.tickN%60 == 1 {
				m.checkForti(cfg)
			}
			// IOC / tehdit istihbarati domain eslestirmesi: 30 sn'de bir (Faz 6.6)
			if m.tickN%30 == 1 {
				m.checkIOC(cfg)
			}
			// otomatik çözülme (S22.8): dakikada bir yinelenmeyen açık olayları kapat
			if m.tickN%60 == 30 {
				m.sweepAutoResolve(cfg)
			}
		}
	}
}

// --- kurallar ---

func (m *Manager) checkBandwidth(cfg Config, snap *capture.Snapshot) {
	if !cfg.Bandwidth.Enabled || !snap.Running {
		m.bwInCount, m.bwOutCount = 0, 0
		return
	}
	need := cfg.Bandwidth.Seconds
	if need <= 0 {
		need = 10
	}
	if inMbps := snap.BpsIn / 1e6; cfg.Bandwidth.InMbps > 0 && inMbps >= cfg.Bandwidth.InMbps {
		m.bwInCount++
		if m.bwInCount == need {
			m.fire("bw", "in", fmt.Sprintf("İndirme hızı %d saniyedir eşik üzerinde: %.1f Mbps", need, inMbps))
		}
	} else {
		m.bwInCount = 0
	}
	if outMbps := snap.BpsOut / 1e6; cfg.Bandwidth.OutMbps > 0 && outMbps >= cfg.Bandwidth.OutMbps {
		m.bwOutCount++
		if m.bwOutCount == need {
			m.fire("bw", "out", fmt.Sprintf("Gönderme hızı %d saniyedir eşik üzerinde: %.1f Mbps", need, outMbps))
		}
	} else {
		m.bwOutCount = 0
	}
}

func (m *Manager) checkPorts(cfg Config, cons []sysmon.Connection) {
	if !cfg.Ports.Enabled {
		return
	}
	set := map[int]struct{}{}
	for _, p := range cfg.Ports.Ports {
		set[p] = struct{}{}
	}
	for _, c := range cons {
		if c.RemoteAddr == "" {
			continue
		}
		port := remotePort(c.RemoteAddr)
		if _, ok := set[port]; !ok {
			continue
		}
		m.fire("port", fmt.Sprint(port),
			fmt.Sprintf("Şüpheli porta bağlantı: uzak %s (%s) — yerel %s", c.RemoteAddr, c.Process, c.LocalAddr))
	}
}

func (m *Manager) checkNewProcess(cfg Config, cons []sysmon.Connection) {
	if !cfg.NewProc.Enabled {
		return
	}
	if n, err := m.st.CountAlertSeen("proc"); err == nil && n == 0 {
		// ilk calistirma: mevcut surecleri sessizce taban cizgisi yap
		for _, c := range cons {
			if c.Process != "" {
				m.markSeen("proc", c.Process)
			}
		}
		return
	}
	ignore := map[string]struct{}{}
	for _, p := range cfg.NewProc.Ignore {
		ignore[p] = struct{}{}
	}
	for _, c := range cons {
		if c.Process == "" {
			continue
		}
		if _, skip := ignore[c.Process]; skip {
			continue
		}
		seen, _ := m.st.IsAlertSeen("proc", c.Process)
		if seen {
			continue
		}
		m.markSeen("proc", c.Process)
		m.fire("proc", c.Process, fmt.Sprintf("Yeni süreç ağa çıktı: %s (pid %d)", c.Process, c.PID))
	}
}

func (m *Manager) checkNewTarget(cfg Config, snap *capture.Snapshot) {
	if !cfg.NewTarget.Enabled {
		return
	}
	if n, err := m.st.CountAlertSeen("target"); err == nil && n == 0 {
		for _, e := range snap.TopEndpoints {
			m.markSeen("target", e.IP)
		}
		return
	}
	minBytes := uint64(cfg.NewTarget.MinTotalMB * 1024 * 1024)
	for _, e := range snap.TopEndpoints {
		if e.Local || e.Total < minBytes {
			continue
		}
		seen, _ := m.st.IsAlertSeen("target", e.IP)
		if seen {
			continue
		}
		m.markSeen("target", e.IP)
		name := e.IP
		if e.Hostname != "" {
			name = fmt.Sprintf("%s (%s)", e.Hostname, e.IP)
		}
		m.fire("target", e.IP, fmt.Sprintf("Yeni hedefle trafik: %s — toplam %.1f MB", name, float64(e.Total)/1024/1024))
	}
}

// --- agent filosu kurallari (checkNewProcess/checkPorts/checkBandwidth'in
// uzak agent'lardan raporlanan veri uzerindeki karsiligi) ---

// checkAgentPorts, her online agent'in son bildirdigi baglanti listesinde
// supheli portlari arar (checkPorts ile ayni kural, hub'in kendi
// sysmon.ListConnections'i yerine agent'in LatestAgentConnections'i).
func (m *Manager) checkAgentPorts(cfg Config, agents []store.AgentWithRates) {
	if !cfg.Ports.Enabled {
		return
	}
	set := map[int]struct{}{}
	for _, p := range cfg.Ports.Ports {
		set[p] = struct{}{}
	}
	for _, a := range agents {
		if !a.Online {
			continue
		}
		for _, c := range m.st.LatestAgentConnections(a.ID) {
			if c.RemoteAddr == "" {
				continue
			}
			port := remotePort(c.RemoteAddr)
			if _, ok := set[port]; !ok {
				continue
			}
			m.fireCtx("port", a.Name+":"+fmt.Sprint(port),
				fmt.Sprintf("Şüpheli porta bağlantı — agent %s: uzak %s (%s) — yerel %s", a.Name, c.RemoteAddr, c.Process, c.LocalAddr),
				fireOpts{Site: a.Site})
		}
	}
}

// checkAgentNewProcess, her online agent icin ayri bir "gorulmusluk" tabani
// tutar (seenKind agent adiyla scope'lanir) — bir agent'ta gorulen surec
// digerlerini sessize almaz, her agent kendi taban cizgisini olusturur.
func (m *Manager) checkAgentNewProcess(cfg Config, agents []store.AgentWithRates) {
	if !cfg.NewProc.Enabled {
		return
	}
	ignore := map[string]struct{}{}
	for _, p := range cfg.NewProc.Ignore {
		ignore[p] = struct{}{}
	}
	for _, a := range agents {
		if !a.Online {
			continue
		}
		seenKind := "agent-proc:" + a.Name
		cons := m.st.LatestAgentConnections(a.ID)
		if n, err := m.st.CountAlertSeen(seenKind); err == nil && n == 0 {
			// bu agent icin ilk degerlendirme: mevcut surecleri sessizce
			// taban cizgisi yap (agent yeni eklendiginde alarm firtinasi olmasin)
			for _, c := range cons {
				if c.Process != "" {
					m.markSeen(seenKind, c.Process)
				}
			}
			continue
		}
		for _, c := range cons {
			if c.Process == "" {
				continue
			}
			if _, skip := ignore[c.Process]; skip {
				continue
			}
			seen, _ := m.st.IsAlertSeen(seenKind, c.Process)
			if seen {
				continue
			}
			m.markSeen(seenKind, c.Process)
			m.fireCtx("proc", a.Name+":"+c.Process,
				fmt.Sprintf("Yeni süreç ağa çıktı — agent %s: %s (pid %d)", a.Name, c.Process, c.PID),
				fireOpts{Site: a.Site})
		}
	}
}

// checkAgentBandwidth, checkBandwidth'in agent filosu karsiligi: her online
// agent'in TUM arayuzlerinin toplam rx/tx bps'ini ayni Mbps esikleriyle
// karsilastirir. "Ardisik N saniye" sayaci burada ~5sn'lik consEvery
// tur araligiyla ilerler (checkBandwidth'in 1sn'lik hub-yerel sayacindan
// farkli olcekte) — cfg.Bandwidth.Seconds bu yuzden agent yolunda "N
// ardisik degerlendirme" anlamina gelir, tam N saniye degil.
func (m *Manager) checkAgentBandwidth(cfg Config, agents []store.AgentWithRates) {
	if !cfg.Bandwidth.Enabled {
		return
	}
	need := cfg.Bandwidth.Seconds
	if need <= 0 {
		need = 10
	}
	seen := map[int64]bool{}
	for _, a := range agents {
		seen[a.ID] = true
		if !a.Online {
			continue
		}
		var rx, tx float64
		for _, r := range a.Rates {
			rx += r.RxBps
			tx += r.TxBps
		}
		c := m.agentBw[a.ID]
		if c == nil {
			c = &agentBwCounter{}
			m.agentBw[a.ID] = c
		}
		if inMbps := rx * 8 / 1e6; cfg.Bandwidth.InMbps > 0 && inMbps >= cfg.Bandwidth.InMbps {
			c.in++
			if c.in == need {
				m.fireCtx("bw", "agent-in:"+a.Name,
					fmt.Sprintf("Agent %s: indirme hızı %d ardışık kontrolde eşik üzerinde: %.1f Mbps", a.Name, need, inMbps),
					fireOpts{Site: a.Site})
			}
		} else {
			c.in = 0
		}
		if outMbps := tx * 8 / 1e6; cfg.Bandwidth.OutMbps > 0 && outMbps >= cfg.Bandwidth.OutMbps {
			c.out++
			if c.out == need {
				m.fireCtx("bw", "agent-out:"+a.Name,
					fmt.Sprintf("Agent %s: gönderme hızı %d ardışık kontrolde eşik üzerinde: %.1f Mbps", a.Name, need, outMbps),
					fireOpts{Site: a.Site})
			}
		} else {
			c.out = 0
		}
	}
	// offline'a dusen/silinen agent'larin sayaclarini temizle (bellek sizintisi olmasin)
	for id := range m.agentBw {
		if !seen[id] {
			delete(m.agentBw, id)
		}
	}
}

// markSeen, alert dedup taban çizgisine bir anahtar ekler. Hata kritik
// değil (en fazla ileride tekrar uyarı) — loglanıp geçilir.
func (m *Manager) markSeen(kind, key string) {
	if err := m.st.MarkAlertSeen(kind, key); err != nil {
		log.Printf("alert dedup kaydi yazilamadi [%s]: %v", kind, err)
	}
}

// fireOpts, fireCtx'e opsiyonel bağlam: sıfır değerleri kind'den türetilir.
type fireOpts struct {
	Site     string
	Severity string // "" → severityForKind(kind)
}

// kindSeverity, uyarı türü → varsayılan önem (S22.6). S22.7 z-büyüklüğüne göre
// dinamik yükseltme ekler.
var kindSeverity = map[string]string{
	"ioc":              "crit",
	"vpn_down":         "crit",
	"port":             "crit", // şüpheli port = güçlü sinyal
	"bw":               "warn",
	"anomaly":          "warn",
	"sdwan_sla_breach": "warn",
	"high_sessions":    "warn",
	"sla_breach":       "crit",
	"proc":             "info",
	"target":           "info",
}

func severityForKind(kind string) string {
	if s, ok := kindSeverity[kind]; ok {
		return s
	}
	return "warn"
}

// fire, fireCtx'in bağlamsız kısayolu.
func (m *Manager) fire(kind, key, message string) { m.fireCtx(kind, key, message, fireOpts{}) }

// fireCtx, olayı kaydeder ve bildirir. S22.6 yaşam döngüsü: aynı (kind,key)
// için açık bir olay varsa yeni satır yerine tekrar sayacı artırılır (sessizce
// — bildirim seli olmasın). Açık olay yoksa cooldown yeni-olay bildirimini
// kısar, sonra insert + Deliver.
func (m *Manager) fireCtx(kind, key, message string, opt fireOpts) {
	m.mu.Lock()
	cfg := m.cfg
	n := m.notifier
	m.mu.Unlock()

	now := time.Now().Unix()
	if open, err := m.st.OpenAlertEventByKey(kind, key); err != nil {
		log.Printf("acik uyari sorgusu hatasi: %v", err)
	} else if open != nil {
		if err := m.st.BumpAlertEvent(open.ID, now, message); err != nil {
			log.Printf("uyari tekrar sayaci hatasi: %v", err)
		}
		return
	}

	// S22.10: aktif bakım penceresi → kayıt tut (state='silenced'), bildirme.
	if reason, ok := m.silenced(kind, opt.Site, key); ok {
		if _, err := m.st.InsertAlertEvent(store.AlertEvent{
			Ts: now, Kind: kind, Key: key, Message: message,
			Severity: cfg.severityFor(kind), State: "silenced", Site: opt.Site,
			Count: 1, FirstTs: now, LastTs: now, Note: "susturuldu: " + reason,
		}); err != nil {
			log.Printf("susturulan uyari kaydi hatasi: %v", err)
		}
		return
	}

	cooldownKey := kind + "|" + key
	m.mu.Lock()
	if t, ok := m.lastFire[cooldownKey]; ok && time.Since(t) < time.Duration(cfg.CooldownMin)*time.Minute {
		m.mu.Unlock()
		return
	}
	m.lastFire[cooldownKey] = time.Now()
	m.mu.Unlock()

	sev := opt.Severity
	if sev == "" {
		sev = cfg.severityFor(kind)
	}
	ev := store.AlertEvent{
		Ts: now, Kind: kind, Key: key, Message: message,
		Severity: sev, State: "firing", Site: opt.Site,
		Count: 1, FirstTs: now, LastTs: now,
		GroupID: m.correlate(cfg, opt.Site, now),
	}
	id, err := m.st.InsertAlertEvent(ev)
	if err != nil {
		log.Printf("uyari kaydi hatasi: %v", err)
		return
	}
	ev.ID = id
	log.Printf("UYARI [%s/%s] %s", kind, sev, message)

	if n != nil {
		n.Deliver(cfg.Notifiers, ev)
	}
}

// RefreshSilences, aktif bakım penceresi önbelleğini DB'den tazeler. run()
// döngüsü periyodik çağırır; sunucu POST/DELETE /api/v1/alerts/silences
// sonrası hemen çağırır.
func (m *Manager) RefreshSilences() {
	sl, err := m.st.ListAlertSilences(true, time.Now().Unix())
	if err != nil {
		log.Printf("susturma listesi okunamadi: %v", err)
		return
	}
	m.silenceMu.Lock()
	m.silences = sl
	m.silenceMu.Unlock()
}

// silenced, bir uyarının aktif bir bakım penceresine uyup uymadığı.
func (m *Manager) silenced(kind, site, key string) (string, bool) {
	now := time.Now().Unix()
	m.silenceMu.RLock()
	defer m.silenceMu.RUnlock()
	for _, s := range m.silences {
		if s.Active(now) && s.Matches(kind, site, key) {
			return s.Reason, true
		}
	}
	return "", false
}

// correlate, yeni bir olay için korelasyon grubu belirler (S22.9): aynı sahada
// CorrelateWindowSec içinde açık bir olay varsa onun group_id'sini (yoksa
// oluşturup peer'lara da atayarak) döndürür. Site "" veya pencere kapalıysa "".
func (m *Manager) correlate(cfg Config, site string, now int64) string {
	if site == "" || cfg.CorrelateWindowSec <= 0 {
		return ""
	}
	peers, err := m.st.OpenAlertEventsBySiteSince(site, now-int64(cfg.CorrelateWindowSec))
	if err != nil || len(peers) == 0 {
		return ""
	}
	for _, p := range peers {
		if p.GroupID != "" {
			return p.GroupID
		}
	}
	// gruba henüz bağlanmamış eş(ler) var → en eskisini çapa yapıp yeni grup kur
	gid := "g-" + strconv.FormatInt(peers[0].ID, 36)
	for _, p := range peers {
		if err := m.st.SetAlertEventGroup(p.ID, gid); err != nil {
			log.Printf("korelasyon grup atama hatasi: %v", err)
		}
	}
	return gid
}

// sweepAutoResolve, AutoResolveMin dakikadır yinelenmeyen açık olayları
// çözüldü işaretler (S22.8). run() içinde dakikada bir, lider-kapılı.
func (m *Manager) sweepAutoResolve(cfg Config) {
	if cfg.AutoResolveMin < 0 {
		return
	}
	mins := cfg.AutoResolveMin
	if mins == 0 {
		mins = 15
	}
	cutoff := time.Now().Add(-time.Duration(mins) * time.Minute).Unix()
	stale, err := m.st.OpenAlertEventsStale(cutoff)
	if err != nil {
		log.Printf("otomatik cozulme sorgusu hatasi: %v", err)
		return
	}
	for _, e := range stale {
		m.resolveEvent(cfg, e, fmt.Sprintf("koşul %d dk yinelenmedi", mins))
	}
}

// resolveEvent, bir olayı çözüldü işaretler, (etkinse) bildirir ve cooldown'ı
// temizler ki koşul tekrarlarsa yeni olay oluşabilsin.
func (m *Manager) resolveEvent(cfg Config, e store.AlertEvent, reason string) {
	now := time.Now().Unix()
	if err := m.st.ResolveAlertEvent(e.ID, now); err != nil {
		log.Printf("uyari cozulme hatasi: %v", err)
		return
	}
	log.Printf("UYARI ÇÖZÜLDÜ [%s] %s (%s)", e.Kind, e.Key, reason)
	m.mu.Lock()
	delete(m.lastFire, e.Kind+"|"+e.Key)
	n := m.notifier
	m.mu.Unlock()
	if cfg.NotifyResolve && n != nil {
		e.State = "resolved"
		e.ResolvedTs = now
		e.Message = "[ÇÖZÜLDÜ] " + e.Message + " — " + reason
		n.Deliver(cfg.Notifiers, e)
	}
}

// --- bakım pencereleri (server icin, S22.10) ---

func (m *Manager) ListSilences(activeOnly bool) ([]store.AlertSilence, error) {
	return m.st.ListAlertSilences(activeOnly, time.Now().Unix())
}

func (m *Manager) AddSilence(sl store.AlertSilence) (int64, error) {
	id, err := m.st.AddAlertSilence(sl)
	if err == nil {
		m.RefreshSilences()
	}
	return id, err
}

func (m *Manager) DeleteSilence(id int64) error {
	err := m.st.DeleteAlertSilence(id)
	m.RefreshSilences()
	return err
}

// --- config erisimi (server icin) ---

func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) UpdateConfig(cfg Config) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := m.st.SaveAlertConfig(string(raw)); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

func (m *Manager) RecentEvents(n int) []store.AlertEvent {
	evs, err := m.st.RecentAlertEvents(n)
	if err != nil {
		return []store.AlertEvent{}
	}
	return evs
}

// --- bildirim kanalı durumu (D3, S12.8) ---

// NotifierStatus, her bildirim kanalının son teslim denemesinin sonucunu döndürür.
func (m *Manager) NotifierStatus() map[string]ChannelStatus {
	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n == nil {
		return map[string]ChannelStatus{}
	}
	return n.Status()
}

// TestNotifiers, güncel yapılandırmadaki tüm etkin kanallara sentetik bir
// uyarı gönderir (senkron) ve sonuçları döndürür.
func (m *Manager) TestNotifiers() map[string]ChannelStatus {
	m.mu.Lock()
	cfg := m.cfg
	n := m.notifier
	m.mu.Unlock()
	if n == nil {
		return map[string]ChannelStatus{}
	}
	return n.Test(cfg.Notifiers)
}

// SetNotifyFailHook, kanal başına teslim hatasında çağrılacak metrik
// kancasını Notifier'a iletir (server Prometheus counter'ı).
func (m *Manager) SetNotifyFailHook(fn func(channel string)) {
	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n != nil {
		n.SetFailHook(fn)
	}
}

// remotePort, "1.2.3.4:443" formatindan portu cikarir.
func remotePort(addr string) int {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			n := 0
			for _, ch := range addr[i+1:] {
				if ch < '0' || ch > '9' {
					return 0
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	return 0
}
