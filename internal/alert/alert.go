package alert

import (
	"encoding/json"
	"fmt"
	"log"
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

	Bandwidth BandwidthConfig  `json:"bandwidth"`
	Ports     PortsConfig      `json:"ports"`
	NewProc   ProcConfig       `json:"new_proc"`
	NewTarget TargetConfig     `json:"new_target"`
	Anomaly   AnomalyConfig    `json:"anomaly"` // Faz 6.2: istatistiksel baseline
	Forti     FortiAlertConfig `json:"forti"`   // Faz 8.5: vpn/sdwan/oturum eşikleri

	Notifiers Notifiers `json:"notifiers"`
}

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
}

// DefaultConfig, ilk calistirma icin makul ayarlar.
func DefaultConfig() Config {
	return Config{
		Enabled:     true,
		CooldownMin: 10,
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
}

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
		notifier:          &Notifier{},
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
			// anomali degerlendirmesi: 5 dakikada bir (Faz 6.2)
			if m.tickN%300 == 1 {
				m.checkAnomaly(cfg)
			}
			// FortiGate uyarilari: dakikada bir (Faz 8.5)
			if m.tickN%60 == 1 {
				m.checkForti(cfg)
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
				m.st.MarkAlertSeen("proc", c.Process)
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
		m.st.MarkAlertSeen("proc", c.Process)
		m.fire("proc", c.Process, fmt.Sprintf("Yeni süreç ağa çıktı: %s (pid %d)", c.Process, c.PID))
	}
}

func (m *Manager) checkNewTarget(cfg Config, snap *capture.Snapshot) {
	if !cfg.NewTarget.Enabled {
		return
	}
	if n, err := m.st.CountAlertSeen("target"); err == nil && n == 0 {
		for _, e := range snap.TopEndpoints {
			m.st.MarkAlertSeen("target", e.IP)
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
		m.st.MarkAlertSeen("target", e.IP)
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
			m.fire("port", a.Name+":"+fmt.Sprint(port),
				fmt.Sprintf("Şüpheli porta bağlantı — agent %s: uzak %s (%s) — yerel %s", a.Name, c.RemoteAddr, c.Process, c.LocalAddr))
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
					m.st.MarkAlertSeen(seenKind, c.Process)
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
			m.st.MarkAlertSeen(seenKind, c.Process)
			m.fire("proc", a.Name+":"+c.Process,
				fmt.Sprintf("Yeni süreç ağa çıktı — agent %s: %s (pid %d)", a.Name, c.Process, c.PID))
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
				m.fire("bw", "agent-in:"+a.Name,
					fmt.Sprintf("Agent %s: indirme hızı %d ardışık kontrolde eşik üzerinde: %.1f Mbps", a.Name, need, inMbps))
			}
		} else {
			c.in = 0
		}
		if outMbps := tx * 8 / 1e6; cfg.Bandwidth.OutMbps > 0 && outMbps >= cfg.Bandwidth.OutMbps {
			c.out++
			if c.out == need {
				m.fire("bw", "agent-out:"+a.Name,
					fmt.Sprintf("Agent %s: gönderme hızı %d ardışık kontrolde eşik üzerinde: %.1f Mbps", a.Name, need, outMbps))
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

// fire, cooldown kontrolunden sonra olayi kaydeder ve bildirimleri dagitir.
func (m *Manager) fire(kind, key, message string) {
	cooldownKey := kind + "|" + key
	m.mu.Lock()
	cfg := m.cfg
	if t, ok := m.lastFire[cooldownKey]; ok && time.Since(t) < time.Duration(cfg.CooldownMin)*time.Minute {
		m.mu.Unlock()
		return
	}
	m.lastFire[cooldownKey] = time.Now()
	m.mu.Unlock()

	ev := store.AlertEvent{Ts: time.Now().Unix(), Kind: kind, Key: key, Message: message}
	id, err := m.st.InsertAlertEvent(ev)
	if err != nil {
		log.Printf("uyari kaydi hatasi: %v", err)
		return
	}
	ev.ID = id
	log.Printf("UYARI [%s] %s", kind, message)

	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n != nil {
		n.Deliver(cfg.Notifiers, ev)
	}
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
