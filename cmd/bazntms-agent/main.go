// bazntms-agent — uclara kurulan telemetri daemon'i: enrollment, periyodik
// telemetri gonderimi, offline disk kuyrugu ve graceful shutdown.
// Windows'ta SCM (Service Control Manager) tarafindan baslatildiginda servis
// dispatcher'ina baglanir (bkz. service_windows.go).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
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
	collectMethod := fl.String("collect-method", "", "surec atfi arka ucu: auto|ebpf|pcap|etw|off (bos = config veya auto)")
	recordFlag := fl.Bool("record", false, "ham paketleri diske kaydet (hub politikasi da acik olmali)")
	recordDir := fl.String("record-dir", "captures", "PCAP kayit dizini")
	logLevel := fl.String("log-level", "", "log seviyesi (config'i override eder)")
	logFormat := fl.String("log-format", "", "log formati: json|text")
	updateEnabled := fl.Bool("update-enabled", true, "otomatik guncelleme (hub guncelleme kanalindan; SHA-256 + varsa ed25519)")
	updateDisabled := fl.Bool("update-disabled", false, "otomatik guncellemeyi kapat (update.disabled config esdegeri)")
	updateChannel := fl.String("update-channel", "stable", "guncelleme kanali: stable|beta")
	updateKey := fl.String("update-key", "", "ed25519 public key (hex); bos ise yalnizca sha256 dogrulanir")
	updateInterval := fl.Int("update-interval", 6, "guncelleme kontrol araligi (saat)")
	showVersion := fl.Bool("version", false, "surum bilgisini yaz ve cik")
	_ = fl.Parse(os.Args[1:])

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
		// atif arka ucu yontemi: flag > config > "" (seçici "auto" sayar).
		attrMethod := firstNonEmpty(*collectMethod, cfg.Collect.Method)
		switch attrMethod {
		case "off":
			pcapWant = false // "off" derin toplamayı da kapatır
		case "":
			// yontem belirtilmemis — eski davranis: collect.pcap / -pcap gecerli.
		default:
			// Acik bir yontem (auto|ebpf|pcap|etw) = "derin toplama istiyorum".
			// eBPF/ETW Npcap/libpcap gerektirmez; collect.pcap: true zorunlulugu
			// kaldirildi (Faz 20). Yalniz hub politikasi (PCAPEnabled) hala kisitlar.
			pcapWant = true
		}
		attrIface := *pcapIface
		if attrIface == "" {
			attrIface = cfg.Collect.PCAPInterface
		}
		if attrIface == "" || attrIface == "auto" {
			attrIface = autoIface()
		}
		// Windows'ta friendly arayuz adini (\Device\NPF_{GUID}) pcap cihaz
		// adina cevirmek newPcapAttrSource icine tasindi (yalniz pcap arka ucu
		// secilince Npcap'e dokunulsun; ETW/eBPF varsayilaninda hic aranmasin).
		var attrEng agent.AttrSource
		attrTried := false // bu politika-acik doneminde atif arka ucu denendi mi
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
				eng, e := agent.NewAttrSource(agent.AttrConfig{Method: attrMethod, Iface: attrIface})
				if e != nil {
					if hint := pcapErrHint(e); hint != "" {
						slog.Warn("surec atfi baslatilamadi — telemetri surecek", "yontem", firstNonEmpty(attrMethod, "auto"), "iface", attrIface, "err", e, "cozum", hint)
					} else {
						slog.Warn("surec atfi baslatilamadi — telemetri surecek", "yontem", firstNonEmpty(attrMethod, "auto"), "iface", attrIface, "err", e)
					}
					return
				}
				slog.Info("surec atfi aktif", "yontem", eng.Method(), "iface", attrIface)
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
		// pcapWant calisma boyunca sabit (bayrak + config'ten bir kez cozulur).
		// Kapaliysa syncAttr() hicbir case'e girmez ve sessiz kalir — surec
		// trafigi / DNS / L7 gorunurlugunun neden bos oldugu loglardan
		// anlasilmadigi icin burada bir kez acikca belirt.
		if !pcapWant {
			if attrMethod == "off" {
				slog.Info("surec atfi kapali — collect.method=off")
			} else {
				slog.Info("derin toplama kapali — surec trafigi / DNS / L7 gorunurlugu yok",
					"cozum", "agent.yml'de collect.pcap: true yapin (veya -pcap ile baslatin); hub'da da -agent-pcap acik olmali")
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
				if dev, rerr := agent.ResolvePcapDevice(iface); rerr == nil {
					iface = dev
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
		// baslatir: systemd Restart=always / launchd KeepAlive / docker
		// --restart / Windows SCM failure-action — bkz. exitAfterUpdate).
		//
		// Varsayilan ACIK (Faz 16 sonrasi). Kapatmak icin: -update-disabled
		// veya config `update: {disabled: true}`. -update-enabled=false de
		// acikca verilirse saygi gosterilir.
		updateOn := true
		fl.Visit(func(f *flag.Flag) {
			if f.Name == "update-enabled" {
				updateOn = *updateEnabled
			}
		})
		if *updateDisabled || cfg.Update.Disabled {
			updateOn = false
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
		if updateOn {
			updateTicker = time.NewTicker(time.Duration(*updateInterval) * time.Hour)
			defer updateTicker.Stop()
			slog.Info("otomatik guncelleme aktif", "channel", *updateChannel,
				"interval_hours", *updateInterval, "imza", *updateKey != "")
			go func() {
				for range updateTicker.C {
					upd := update.NewClient(client.BaseURL(), *updateChannel, *updateKey, st.Token, client.UpdateHTTPClient())
					applied, err := upd.Apply(version.Version)
					if err != nil {
						slog.Warn("guncelleme kontrolu basarisiz", "err", err)
						continue
					}
					if applied {
						slog.Info("guncelleme kuruldu, yeniden baslatiliyor", "channel", *updateChannel)
						update.CleanupOld(os.Args[0])
						exitAfterUpdate()
					}
				}
			}()
		}

		timer := time.NewTimer(time.Duration(client.Interval()) * time.Second)
		defer timer.Stop()
		slog.Info("telemetri dongusu basladi", "interval", client.Interval())

		// Ard arda 401 sayaci: hub veritabani sifirlaninca (agent kaydi dustu)
		// telemetri kalici 401 doner. Tek bir 401 gecici olabilir (hub restart
		// sirasinda DB baglantisi) — bu yuzden esige ulasinca yeniden enroll
		// edilir. reenrollAfter * interval kadar bekleme, hub tarafinin ayni
		// machine_id'li bayat kaydi yeniden kullanmasi icin de yeterli sure.
		//
		// Sayac state dosyasinda tutulur (Client.NoteAuthFailure): bellekte
		// tutulunca her restart onu sifirliyor, token'i olmus ama sik yeniden
		// baslayan agent esige hic ulasamiyordu. Onceki oturumdan devral.
		authFails := client.AuthFailStreak()
		sendFails := 0
		const reenrollAfter = 3
		if authFails > 0 {
			slog.Warn("onceki oturumdan devralinan ardisik 401 sayaci", "ard_arda", authFails, "esik", reenrollAfter)
		}

		for {
			select {
			case <-stop:
				slog.Info("kapatiliyor")
				return nil
			case <-timer.C:
				batch := client.Collect()
				batch.AttrMethod = "off"
				if attrEng != nil {
					batch.AttrMethod = attrEng.Method()
					batch.ProcessTraffic = attrEng.Deltas()
					batch.L7 = attrEng.L7Deltas()
					batch.DNS = attrEng.DNSDeltas()
				}
				switch err := client.Send(st, batch); {
				case err == nil:
					if authFails > 0 {
						client.ClearAuthFailure() // diskteki sayaci da temizle
					}
					authFails, sendFails = 0, 0
					slog.Debug("telemetri gonderildi", "ifaces", len(batch.Interfaces), "conns", len(batch.Connections))
				case errors.Is(err, agent.ErrUnauthorized):
					sendFails++
					if authFails < reenrollAfter {
						authFails = client.NoteAuthFailure() // diskte kalici — restart sifirlamaz
					}
					if authFails < reenrollAfter {
						slog.Warn("telemetri reddedildi (401) — offline kuyruga alindi", "ard_arda", authFails, "esik", reenrollAfter)
						break
					}
					slog.Warn("hub agent kimligini surekli reddediyor (401) — yeniden enroll ediliyor", "denemeler", authFails)
					if newSt, rerr := client.Reenroll(); rerr != nil {
						slog.Error("yeniden enroll basarisiz — elle mudahale gerekebilir", "err", rerr)
					} else {
						st = newSt
						authFails, sendFails = 0, 0
						slog.Info("yeniden enroll tamamlandi", "agent_id", st.AgentID)
					}
				default:
					sendFails++
					slog.Warn("telemetri gonderilemedi (offline kuyruga alindi)", "err", err, "ard_arda", sendFails)
				}
				// Send, hub politikasini (interval + pcap_enabled) tazeledi;
				// atif motorunu yeni duruma gore ac/kapat.
				syncAttr()
				timer.Reset(retryDelay(client.Interval(), sendFails))
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

// retryDelay, bir sonraki telemetri denemesine kadar beklenecek süre (S21.15).
// Başarılıysa (fails == 0) normal aralık. Ard arda hata varsa üstel geri
// çekilme (aralık × 2^fails, en fazla 8× ve 5 dk) + ±%20 jitter — 5000
// agent'ın kesinti sonrası hub'ı aynı anda dövmemesi için.
func retryDelay(intervalSec, fails int) time.Duration {
	base := time.Duration(intervalSec) * time.Second
	if fails <= 0 {
		return base
	}
	mult := 1 << minInt(fails, 3) // 2, 4, 8
	d := base * time.Duration(mult)
	if d > 5*time.Minute {
		d = 5 * time.Minute
	}
	jitter := time.Duration(rand.Int63n(int64(d)/5+1)) - d/10
	return d + jitter
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "wpcap.dll"):
		return "pcap arka ucu icin Npcap gerekir (https://npcap.com). Sürec trafigi + DNS ETW ile Npcap'siz calisir (-collect-method=etw / auto); Npcap yalniz L7 (SNI/Host) ve ham -record icin lazim"
	case strings.Contains(msg, "error opening adapter"),
		strings.Contains(msg, "system cannot find the device"),
		strings.Contains(msg, "birim etiketi"):
		// Npcap var ama verilen arayuz adi pcap cihazi degil (friendly ad).
		return `yakalama arayuzu pcap tarafindan taninmadi — config'de collect.pcap_interface'i "auto" birakin veya '\Device\NPF_{GUID}' verin (PowerShell: Get-NetAdapter | Select Name,InterfaceGuid)`
	}
	return ""
}

// autoIface, atf/kayit icin ilk uygun arayuzu secer: up, loopback degil ve
// yonlendirilebilir bir IPv4'u olan. IPv4 bulunamazsa ikinci gecişte adresi
// olan ilk arayuze duser (yalniz-IPv6 ortamlar). Windows'ta friendly ad
// doner; cagiran taraf ResolvePcapDevice ile \Device\NPF_ adina cevirir.
func autoIface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var fallback string
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		if fallback == "" {
			fallback = i.Name
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipn.IP.To4()
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
				continue
			}
			return i.Name
		}
	}
	return fallback
}
