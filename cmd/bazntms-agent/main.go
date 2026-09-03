// bazntms-agent — uclara kurulan telemetri daemon'i: enrollment, periyodik
// telemetri gonderimi, offline disk kuyrugu ve graceful shutdown.
// Windows'ta SCM (Service Control Manager) tarafindan baslatildiginda servis
// dispatcher'ina baglanir (bkz. service_windows.go).
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gokayybaz/bazntms/internal/agent"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/config"
	"github.com/gokayybaz/bazntms/internal/logging"
	"github.com/gokayybaz/bazntms/internal/update"
	"github.com/gokayybaz/bazntms/internal/version"
)

func main() {
	fl := flag.NewFlagSet("bazntms-agent", flag.ExitOnError)
	configPath := fl.String("config", "bazntms-agent.yml", "agent YAML config dosyasi")
	hubURL := fl.String("hub-url", "", "hub adresi (birden fazla: virgulle ayirin → failover sirasi)")
	enrollToken := fl.String("enroll-token", "", "enrollment token'i (ilk kayit icin; config'i override eder)")
	name := fl.String("name", "", "agent adi (config'i override eder)")
	site := fl.String("site", "", "site etiketi (config'i override eder)")
	stateFile := fl.String("state", "bazntms-agent.state.json", "kalici agent kimlik dosyasi")
	interval := fl.Int("interval", 0, "telemetri araligi sn (config'i override eder)")
	hubCAFile := fl.String("hub-ca", "", "hub CA sertifikasi (PEM) — mTLS'te sunucuyu dogrulamak icin; bos ise ilk baglantida guvenilir kabul edilip pinlenir (TOFU)")
	pcapFlag := fl.Bool("pcap", false, "surec bazli trafik atfi icin paket yakalama (config'i override eder; root/admin gerekir)")
	pcapIface := fl.String("pcap-iface", "", "atif yakalamasi icin arayuz (bos = otomatik)")
	recordFlag := fl.Bool("record", false, "ham paketleri diske kaydet (hub politikasi da acik olmali)")
	recordDir := fl.String("record-dir", "captures", "PCAP kayit dizini")
	logLevel := fl.String("log-level", "", "log seviyesi (config'i override eder)")
	logFormat := fl.String("log-format", "", "log formati: json|text")
	updateEnabled := fl.Bool("update-enabled", false, "otomatik guncelleme (imza dogrulamali, Faz 7.3)")
	updateChannel := fl.String("update-channel", "stable", "guncelleme kanali: stable|beta")
	updateKey := fl.String("update-key", "", "ed25519 public key (hex); bos ise yalnizca sha256 dogrulanir")
	updateInterval := fl.Int("update-interval", 6, "guncelleme kontrol araligi (saat)")
	showVersion := fl.Bool("version", false, "surum bilgisini yaz ve cik")
	fl.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("bazntms-agent %s (protokol v%d, %s)\n", version.Version, version.ProtocolVersion, version.Info()["go_version"])
		return
	}

	var cfg *config.AgentConfig
	var err error
	if _, statErr := os.Stat(*configPath); statErr == nil {
		cfg, err = config.LoadAgent(*configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config:", err)
			os.Exit(1)
		}
	} else {
		cfg = &config.AgentConfig{}
	}

	// config -> flag override'lari; araya platform katmani girer
	// (Windows'ta MSI property'lerinden yazilan kayit defteri degerleri,
	// oncelik: flag > registry > config)
	if *hubURL == "" {
		*hubURL = firstNonEmpty(platformValue("hub_url"), cfg.Hub.URL)
	}
	if *enrollToken == "" {
		*enrollToken = firstNonEmpty(platformValue("enroll_token"), cfg.Hub.Token)
	}
	if *name == "" {
		*name = cfg.Agent.Name
	}
	if *site == "" {
		*site = firstNonEmpty(platformValue("site"), cfg.Agent.Site)
	}
	intervalSec := *interval
	if intervalSec <= 0 {
		intervalSec = cfg.Collect.IntervalSeconds
	}
	level := cfg.Log.Level
	if *logLevel != "" {
		level = *logLevel
	}
	format := cfg.Log.Format
	if *logFormat != "" {
		format = *logFormat
	}
	logOpts := logging.Options{Level: level, Format: format}
	if serviceMode() {
		// servis modunda stdout kaybolur; loglari config ile ayni dizine yaz
		// (C:\ProgramData\bazntms\agent.log)
		if f, ferr := os.OpenFile(filepath.Join(filepath.Dir(*configPath), "agent.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); ferr == nil {
			logOpts.Out = f
			defer f.Close()
		}
	}
	logging.Setup(logOpts)

	hostname, _ := os.Hostname()
	if *name == "" {
		*name = hostname
	}

	// hub havuzu: -hub-url CSV (a,b) veya config hub.url / hub.urls (Faz 5.4 failover)
	var hubPool []string
	for _, u := range cfg.Hub.URLs {
		if u != "" {
			hubPool = append(hubPool, u)
		}
	}
	if len(hubPool) == 0 && *hubURL != "" {
		for _, u := range strings.Split(*hubURL, ",") {
			if u = strings.TrimSpace(u); u != "" {
				hubPool = append(hubPool, u)
			}
		}
	}
	*hubURL = ""
	if len(hubPool) > 0 {
		*hubURL = hubPool[0]
	}

	slog.Info("bazNTMS agent basladi",
		"version", version.Version,
		"protocol_version", version.ProtocolVersion,
		"agent", *name,
		"site", *site,
		"hubs", len(hubPool),
	)
	if *hubURL == "" {
		slog.Error("hub url gerekli", "ornek", "-hub-url https://hub.example.com veya bazntms-agent.yml icerisinde hub.url")
		os.Exit(1)
	}
	if *enrollToken == "" {
		// kayitli agent kimligi varsa enrollment gerekmez
		if agent.New(agent.Options{StateFile: *stateFile}).LoadState().Token == "" {
			slog.Warn("enrollment token'i verilmemis — ilk baglanti basarisiz olacak",
				"cozum", "hub'i -enroll-token ile baslatip token'i bu agent'a verin")
		}
	}

	client := agent.New(agent.Options{
		HubURLs:     hubPool,
		EnrollToken: *enrollToken,
		Name:        *name,
		Site:        *site,
		StateFile:   *stateFile,
		IntervalSec: intervalSec,
		HubCAFile:   firstNonEmpty(*hubCAFile, cfg.Hub.CAFile),
	})

	// run, agent'in omuz dongusunu yurutur; stop kapaninca temiz cikar.
	// Windows servis modunda ayni fonksiyon SCM handler'i tarafindan
	// calistirilir (Stop/Shutdown -> stop kapanir).
	run := func(stop chan struct{}) error {
		st, err := client.Enroll()
		if err != nil {
			return fmt.Errorf("enrollment basarisiz: %w", err)
		}
		slog.Info("agent kayitli", "agent_id", st.AgentID)

		// derin toplama: agent istegi + hub politikasi ikisi de acik olmali.
		// Politika hub'da calisma aninda degisir (telemetri cevabindaki
		// pcap_enabled) ve kayitli agent enrollment'i tekrarlamadigi icin
		// baslangicta client.PCAPEnabled() bayat olabilir — bu yuzden atif
		// motoru sabit degil, asagidaki syncAttr ile dongude ac/kapat yonetilir.
		pcapWant := *pcapFlag || cfg.Collect.PCAP
		attrIface := *pcapIface
		if attrIface == "" {
			attrIface = cfg.Collect.PCAPInterface
		}
		if attrIface == "" || attrIface == "auto" {
			attrIface = autoIface()
		}
		var attrEng *agent.AttrEngine
		attrTried := false // bu politika-acik doneminde NewAttrEngine denendi mi
		attrOffLogged := false
		defer func() {
			if attrEng != nil {
				attrEng.Stop()
			}
		}()
		syncAttr := func() {
			allow := pcapWant && client.PCAPEnabled()
			switch {
			case allow && attrEng == nil && !attrTried:
				attrTried = true
				eng, e := agent.NewAttrEngine(attrIface)
				if e != nil {
					if hint := pcapErrHint(e); hint != "" {
						slog.Warn("surec atfi baslatilamadi — telemetri surecek", "iface", attrIface, "err", e, "cozum", hint)
					} else {
						slog.Warn("surec atfi baslatilamadi — telemetri surecek", "iface", attrIface, "err", e)
					}
					return
				}
				slog.Info("surec atfi aktif", "iface", attrIface)
				attrEng = eng
				attrOffLogged = false
			case !allow && attrEng != nil:
				slog.Info("surec atfi durduruldu — hub PCAP politikasi kapandi")
				attrEng.Stop()
				attrEng = nil
				attrTried = false
				attrOffLogged = true // "durduruldu" yeterli; ayrica "devre disi" yazma
			case !allow && attrEng == nil && pcapWant && !attrOffLogged:
				slog.Info("PCAP politikasi hub tarafinda kapali — surec atfi devre disi", "cozum", "hub'i -agent-pcap ile baslatin")
				attrOffLogged = true
				attrTried = false
			}
		}
		syncAttr()

		// ham PCAP kaydi: politika + agent istegi
		if *recordFlag || cfg.Collect.PCAPRecord {
			if client.PCAPEnabled() {
				iface := *pcapIface
				if iface == "" || iface == "auto" {
					iface = autoIface()
				}
				dir := *recordDir
				if dir == "" {
					dir = cfg.Collect.PCAPDir
				}
				recEngine := capture.NewEngine()
				recEngine.SetRecordOptions(dir, uint64(100)<<20)
				if err := recEngine.Start(iface); err != nil {
					if hint := pcapErrHint(err); hint != "" {
						slog.Warn("PCAP yakalama acilamadi", "err", err, "cozum", hint)
					} else {
						slog.Warn("PCAP yakalama acilamadi", "err", err)
					}
				} else if err := recEngine.StartRecording(); err != nil {
					slog.Warn("PCAP kayit baslatilamadi", "err", err)
					recEngine.Stop()
				} else {
					slog.Info("ham PCAP kaydi basladi", "iface", iface, "dir", dir)
					defer recEngine.Stop()
				}
			} else {
				slog.Info("ham PCAP kaydi icin hub politikasi kapali")
			}
		}

		// otomatik guncelleme (Faz 7.3): periyodik manifest kontrolu; yukselme
		// varsa indir + dogrula + binary'yi degistir + cik (supervisor yeniden
		// baslatir: systemd/launchd/k8s restartPolicy:Always)
		if !*updateEnabled {
			*updateEnabled = cfg.Update.Enabled
		}
		if *updateChannel == "stable" && cfg.Update.Channel != "" {
			*updateChannel = cfg.Update.Channel
		}
		if *updateKey == "" {
			*updateKey = cfg.Update.PublicKey
		}
		if *updateInterval == 6 && cfg.Update.IntervalHours > 0 {
			*updateInterval = cfg.Update.IntervalHours
		}
		var updateTicker *time.Ticker
		if *updateEnabled {
			updateTicker = time.NewTicker(time.Duration(*updateInterval) * time.Hour)
			defer updateTicker.Stop()
			slog.Info("otomatik guncelleme aktif", "channel", *updateChannel,
				"interval_hours", *updateInterval, "imza", *updateKey != "")
			go func() {
				for range updateTicker.C {
					upd := update.NewClient(client.BaseURL(), *updateChannel, *updateKey)
					applied, err := upd.Apply(version.Version)
					if err != nil {
						slog.Warn("guncelleme kontrolu basarisiz", "err", err)
						continue
					}
					if applied {
						slog.Info("guncelleme kuruldu, yeniden baslatiliyor", "channel", *updateChannel)
						update.CleanupOld(os.Args[0])
						os.Exit(0)
					}
				}
			}()
		}

		timer := time.NewTimer(time.Duration(client.Interval()) * time.Second)
		defer timer.Stop()
		slog.Info("telemetri dongusu basladi", "interval", client.Interval())

		for {
			select {
			case <-stop:
				slog.Info("kapatiliyor")
				return nil
			case <-timer.C:
				batch := client.Collect()
				if attrEng != nil {
					batch.ProcessTraffic = attrEng.Deltas()
				}
				if err := client.Send(st, batch); err != nil {
					slog.Warn("telemetri gonderilemedi (offline kuyruga alindi)", "err", err)
				} else {
					slog.Debug("telemetri gonderildi", "ifaces", len(batch.Interfaces), "conns", len(batch.Connections))
				}
				// Send, hub politikasini (interval + pcap_enabled) tazeledi;
				// atif motorunu yeni duruma gore ac/kapat.
				syncAttr()
				timer.Reset(time.Duration(client.Interval()) * time.Second)
			}
		}
	}

	// Windows: SCM tarafindan baslatildiysak servis olarak kos (signal
	// kanali olmaz; Stop/Shutdown istekleri stop kanalini kapatir).
	if serviceMode() {
		if err := runService(run); err != nil {
			slog.Error("servis sonlandi", "err", err)
			os.Exit(1)
		}
		return
	}

	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		close(stop)
	}()

	if err := run(stop); err != nil {
		slog.Error("agent sonlandi", "err", err)
		os.Exit(1)
	}
}

// firstNonEmpty, bos olmayan ilk degeri dondurur.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// pcapErrHint, pcap acilis hatasinin Windows'ta Npcap eksikligi oldugunu
// tespit edip anlamli bir cozum onerisi dondurur — oncesinde gopacket'in ham
// "couldn't load wpcap.dll" hatasi tek basina loglaniyordu, bu hatanin
// Npcap kurulumuyla ilgisi oldugunu bilmeyen bir kullaniciya bir sey ifade
// etmiyordu (bkz. plan: "Windows'ta Npcap eksikligini sessizce yutmak
// yerine bildir"). Bos donerse (Windows disi, ya da farkli bir pcap hatasi)
// cagiran ham hatayi oldugu gibi loglamaya devam eder.
func pcapErrHint(err error) string {
	if runtime.GOOS != "windows" || err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "wpcap.dll") {
		return "Npcap kurulu degil gibi gorunuyor — https://npcap.com adresinden indirip kurun (yonetici olarak calistirin), sonra agent servisini yeniden baslatin"
	}
	return ""
}

// autoIface, atf/kayit icin ilk uygun arayuzu secer (up, loopback degil).
func autoIface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		return i.Name
	}
	return ""
}
