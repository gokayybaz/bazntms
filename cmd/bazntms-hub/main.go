// bazntms-hub — merkezi sunucu: ingest, REST/WS API, dashboard, uyarılar,
// AI analizi ve rapor motoru. Faz 4 ile olcek altyapisi: SQLite veya
// PostgreSQL/TimescaleDB depo, opsiyonel NATS JetStream kuyrugu, graceful
// shutdown ve coklu-replika rolleri (capture/alerts/poller anahtarlari).
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	//nolint:gosec // G108: pprof yalnizca opt-in -pprof adresinde acilir, ana API mux'inda degil
	_ "net/http/pprof" // -pprof adresinde DefaultServeMux'a kaydolur
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/compliance"
	"github.com/gokayybaz/bazntms/internal/config"
	"github.com/gokayybaz/bazntms/internal/devpoll"
	"github.com/gokayybaz/bazntms/internal/flows"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/ioc"
	"github.com/gokayybaz/bazntms/internal/logging"
	"github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/queue"
	"github.com/gokayybaz/bazntms/internal/scheduler"
	"github.com/gokayybaz/bazntms/internal/server"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/syslogd"
	"github.com/gokayybaz/bazntms/internal/update"
	"github.com/gokayybaz/bazntms/internal/vault"
	"github.com/gokayybaz/bazntms/internal/version"
	"github.com/gokayybaz/bazntms/web"
)

func main() {
	fl := flag.NewFlagSet("bazntms-hub", flag.ExitOnError)
	port := fl.String("port", "8080", "HTTP sunucu portu")
	dev := fl.Bool("dev", false, "frontend embed'i atla (vite dev server ile gelistirme)")
	dbPath := fl.String("db", "bazntms.db", "SQLite dosyasi veya postgres:// DSN")
	retentionH := fl.Int("retention-hours", 24*7, "veritabani saklama suresi (saat, SQLite/Prune modu)")
	agentArchiveDays := fl.Int("agent-archive-days", 30, "bu kadar gun cevrimdisi kalan agent'lar tam cascade ile silinir (0 = kapali)")
	natsURL := fl.String("nats", "", "NATS JetStream adresi (bos = kuyruk kapali, dogrudan yazim; ex: nats://localhost:4222)")
	captureOn := fl.Bool("capture", true, "hub'in kendi paket yakalamasi (coklu replikada kapatilir)")
	alertsOn := fl.Bool("alerts", true, "uyari motoru (coklu replikada tek replikada acilir)")
	pollerOn := fl.Bool("poller", true, "SNMP cihaz poller'i (coklu replikada tek replikada acilir)")
	pruneOn := fl.Bool("prune", true, "veritabani bakimi (eski satirlarin temizligi); coklu replikada YALNIZCA bir hub'da acik olmali")
	pollInterval := fl.Int("poll-interval", 0, "tum cihazlar icin tek tip poll araligi (sn); 0 = per-device deger. min 5")
	devpollConcurrency := fl.Int("devpoll-concurrency", 96, "tek poll dongusunde eszamanli yoklanan cihaz sayisi tavani (buyuk filolar icin; S21.10)")
	pprofAddr := fl.String("pprof", "", "net/http/pprof dinleme adresi (ex: 127.0.0.1:6060); bos = kapali")
	pprofRates := fl.Int("pprof-rates", 0, "0'dan buyukse block + mutex profillemesini acar (SetBlockProfileRate=N ns, SetMutexProfileFraction=N); yalnizca -pprof ile anlamli, kucuk ek yuk (S21.6)")
	geoipDir := fl.String("geoip-dir", "geoip", "MaxMind GeoLite2 .mmdb dosyalarinin dizini")
	ipAPILookup := fl.Bool("ip-api-lookup", true, "MMDB yoksa ip-api.com ile IP cozumleme")
	authPassword := fl.String("auth-password", "", "Arayuz sifresi (bos ise kimlik dogrulama kapali; AUTH_PASSWORD de gecerli)")
	configPath := fl.String("config", "", "YAML config dosyasi (bayraklar ustunlukte)")
	logLevel := fl.String("log-level", "", "log seviyesi: debug|info|warn|error (config'i override eder)")
	logFormat := fl.String("log-format", "", "log formati: json|text (config'i override eder)")
	enrollToken := fl.String("enroll-token", "", "agent enrollment token'i (bos ise rastgele uretilir ve loglanir)")
	multiSite := fl.Bool("multi-site", false, "coklu-saha (MSP) modu: site sert yetki siniri — agent kaydi site-bagli enroll token ister, site-admin rolu etkinlesir (bkz. docs/DEPLOYMENT-MODEL.md)")
	mockDevices := fl.Bool("mock-devices", false, "olcek testi: vendor=mock cihaz eklemeye izin ver (sentetik SNMP filosu — bazntms-loadgen -mode device)")
	sessionStore := fl.String("session-store", "memory", "panel oturum deposu: memory (tek replika) | db (paylasimli `sessions` tablosu — coklu controller replikasi icin, A4)")
	queueMaxAgeH := fl.Int("queue-max-age-hours", 24, "JetStream stream mesaj yasi siniri (saat) — tuketilmeyen mesajlar bu sureden sonra dusurulur")
	queueWorkers := fl.Int("queue-workers", 4, "JetStream store-writer paralel worker sayisi (yuksek flow hacmi / patlama icin artirin)")
	telemetryInterval := fl.Int("telemetry-interval", 30, "agent telemetri araligi (saniye)")
	agentPCAP := fl.Bool("agent-pcap", false, "agent'larda derin toplama ve PCAP kaydina izin ver (politika)")
	tlsOn := fl.Bool("tls", false, "HTTPS + agent karsilikli TLS (mTLS): hub kendi CA'sini uretir, agent CSR'larini enrollment'ta imzalar")
	tlsDir := fl.String("tls-dir", "pki", "CA + sunucu sertifikasi dizini (ca.crt/ca.key/server.crt/server.key)")
	tlsCert := fl.String("tls-cert", "", "operator sunucu sertifikasi (PEM); bos = CA'dan otomatik uret")
	tlsKey := fl.String("tls-key", "", "operator sunucu ozel anahtari (PEM); -tls-cert ile birlikte")
	tlsHosts := fl.String("tls-hosts", "", "sunucu sertifikasi SAN'lari (virgulle): hub'in DNS adi/IP'leri — agent'in baglandigi ad buraya girmeli")
	publicURL := fl.String("public-url", "", "panelin dis adresi (ör. https://ntms.example.com) — WS origin izin listesi ve OIDC redirect varsayilani icin")
	vaultKeyFile := fl.String("vault-key-file", "vault.key", "Kimlik kasasi master key dosyasi (yoksa uretilir; -vault-key-source=file iken)")
	vaultKeySource := fl.String("vault-key-source", "file", "Master anahtar kaynagi: file (-vault-key-file) | env (BAZNTMS_VAULT_MASTER_KEY — disk'e yazilmaz, bulut secret manager / KMS enjeksiyonu)")
	flowPort := fl.String("flow-port", "", "NetFlow v5/v9 + IPFIX + sFlow v5 UDP dinleme portu (bos = kapali; ex: 2055)")
	sflowPort := fl.String("sflow-port", "", "sFlow v5 icin ayri UDP portu (bos = kapali; ex: 6343). -flow-port zaten sFlow'u da kabul eder; bu yalnizca farkli portta dinlemek icin")
	flowExporter := fl.String("flow-exporter", "", "NetFlow/sFlow exporter IP override — hub bir NAT/röle arkasindaysa (ör. Docker Desktop) paketin kaynak IP'si kaybolur; tek exporter'li kurulumda cihazin IP'sini yazin")
	syslogPort := fl.String("syslog-port", "", "Syslog UDP dinleme portu (bos = kapali; ex: 5514)")
	iocFile := fl.String("ioc-file", "", "Tehdit istihbarati domain kara listesi (IOC) — eslesen L7/DNS trafigi 'ioc' uyarisi uretir. hosts/AdBlock/duz metin formatlari; mtime degisince otomatik yeniden yuklenir")
	updatesDir := fl.String("updates-dir", "", "Agent guncelleme kanali dizini (bos: -update-github-repo doluysa 'updates', degilse kapali)")
	updateRepo := fl.String("update-github-repo", "gokayybaz/bazntms", "Agent binary'lerini cekecek GitHub deposu (owner/name); bos = GitHub senkronu kapali, yalnizca -updates-dir icerigi (bazntmsctl update sign) sunulur")
	updateSyncInterval := fl.Duration("update-github-interval", 30*time.Minute, "GitHub release yoklama araligi")
	complianceOn := fl.Bool("compliance", false, "5651 log imzalama motoru: hash-zincir + Merkle checkpoint + gunluk muhur")
	tsaURL := fl.String("tsa-url", "", "RFC 3161 zaman damgasi servisi (TSA) adresi")
	complianceKey := fl.String("compliance-key", "compliance.key", "Manifest imza anahtari (ed25519 PEM; yoksa uretilir)")
	wormDir := fl.String("worm-dir", "", "Gunluk imzali log paketi dizini (WORM)")
	maskPII := fl.Bool("mask-pii", false, "Delil paketinde PII maskeleme (A.5.34)")
	complianceRetention := fl.Int("compliance-retention-days", 730, "Ham uyum logu saklama suresi (gun; 5651 minimum 730)")
	showVersion := fl.Bool("version", false, "surum bilgisini yaz ve cik")
	_ = fl.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("bazntms-hub %s (protokol v%d, %s)\n", version.Version, version.ProtocolVersion, version.Info()["go_version"])
		return
	}

	cfg, err := config.LoadHub(fl, *configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}
	if *logFormat != "" {
		cfg.Log.Format = *logFormat
	}
	logging.Setup(logging.Options{Level: cfg.Log.Level, Format: cfg.Log.Format})
	if *complianceKey == "" {
		*complianceKey = "compliance.key"
	}
	_ = complianceKey // config.LoadHub flag'e yazar (aşağıda kullanılır)

	// graceful shutdown zemini (Faz 4.4): SIGINT/SIGTERM → http Shutdown →
	// defer'lar (collector, alerts, poller, kuyruk) sirayla kapanir
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("bazNTMS hub basliyor",
		"version", version.Version,
		"protocol_version", version.ProtocolVersion,
		"port", *port,
	)
	slog.Info("baz Network Traffic Monitoring System",
		"db", *dbPath,
		"nats", *natsURL != "",
		"capture", *captureOn,
		"alerts", *alertsOn,
		"poller", *pollerOn,
		"poll_interval", *pollInterval,
		"retention_hours", *retentionH,
		"auth", *authPassword != "",
	)

	if *authPassword == "" {
		if env := os.Getenv("AUTH_PASSWORD"); env != "" {
			*authPassword = env
		}
	}
	if *authPassword == "" {
		slog.Warn("kimlik dogrulama kapali — LAN uzerinden herkes erisebilir", "cozum", "-auth-password ile sifre belirleyin")
	}

	var static fs.FS
	if !*dev {
		sub, err := web.Dist()
		if err != nil {
			slog.Error("frontend embed okunamadi", "err", err)
			os.Exit(1)
		}
		static = sub
	} else {
		slog.Info("dev modu: statik dosyalar serve edilmiyor, vite dev server kullanin")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("veritabani acilamadi", "err", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	// DB bağlantı havuzu istatistiklerini bazntms_db_pool_* metriklerine bağla
	// (S21.5 — ölçek koşularında havuz doygunluğu göstergesi).
	if ps, ok := st.(interface{ PoolStats() sql.DBStats }); ok {
		metrics.RegisterDBPool(ps.PoolStats)
	}

	retention := time.Duration(*retentionH) * time.Hour
	// veritabani bakimi: eski satirlarin temizligi + (TS'te) native chunk-drop
	// retention. Collector'dan bagimsiz (coklu-hub'da -capture=false).
	if *pruneOn {
		if err := st.ConfigureRetention(retention); err != nil {
			slog.Warn("retention politikalari kurulamadi", "err", err)
		}
		archiveAfter := time.Duration(*agentArchiveDays) * 24 * time.Hour
		maint := store.NewMaintainer(st, retention, archiveAfter)
		maint.Start()
		defer maint.Stop()
		slog.Info("veritabani bakimi aktif", "retention_saat", *retentionH,
			"agent_arsiv_gun", *agentArchiveDays, "aralik", "15dk")
	} else {
		slog.Info("veritabani bakimi bu replikada kapali (-prune=false)")
	}

	// NATS JetStream kuyrugu (Faz 4.2): ingest → processor ayrismasi
	var q *queue.Queue
	if *natsURL != "" {
		q, err = queue.Connect(*natsURL, time.Duration(*queueMaxAgeH)*time.Hour)
		if err != nil {
			slog.Error("nats baglantisi kurulamadi", "url", *natsURL, "err", err)
			os.Exit(1)
		}
		defer q.Close()
		if err := q.RunProcessor(ctx, st, *queueWorkers); err != nil {
			slog.Error("kuyruk processor baslatilamadi", "err", err)
			os.Exit(1)
		}
		slog.Info("nats jetstream aktif", "url", *natsURL, "stream", "BAZNTMS")
	}

	engine := capture.NewEngine()
	if *captureOn {
		collector := store.NewCollector(engine, st, *dbPath)
		collector.Start()
		defer collector.Stop()
	} else {
		slog.Info("hub paket yakalamasi kapali (coklu replika ingest modu)")
	}

	alertCfg := alert.DefaultConfig()
	if raw, err := st.LoadAlertConfig(); err == nil && raw != "" {
		if json.Unmarshal([]byte(raw), &alertCfg) != nil {
			slog.Warn("uyari ayarlari okunamadi, varsayilanlar kullaniliyor")
			alertCfg = alert.DefaultConfig()
		}
	}
	alertCfg = alert.NormalizeConfig(alertCfg)
	alertCfg = alert.NormalizeFortiConfig(alertCfg)
	alerts := alert.NewManager(alertCfg, st, engine, *telemetryInterval)
	if *iocFile != "" {
		if list, err := ioc.Load(*iocFile); err != nil {
			slog.Error("IOC listesi yuklenemedi", "file", *iocFile, "err", err)
		} else {
			slog.Info("IOC listesi yuklendi", "file", *iocFile, "domain", list.Count())
			alerts.SetIOC(list)
			go list.Watch(2*time.Minute, ctx.Done())
		}
	}
	if *alertsOn {
		// C1 (Faz 15): coklu controller replikasinda yalnizca lider degerlendirir.
		// SQLite / tek replika modunda Leader hemen ve kalici lider olur (no-op).
		leaderAlerts := st.Leader(store.LeaderKeyAlerts, "alerts")
		go leaderAlerts.Run(ctx)
		alerts.SetLeaderCheck(leaderAlerts.IsLeader)
		alerts.Start()
		defer alerts.Stop()
	} else {
		slog.Info("uyari motoru kapali (coklu replika ingest modu)")
	}

	geo := geoip.New(
		filepath.Join(*geoipDir, "GeoLite2-Country.mmdb"),
		filepath.Join(*geoipDir, "GeoLite2-ASN.mmdb"),
		*ipAPILookup,
	)

	if *agentPCAP {
		slog.Info("agent PCAP politikasi acik")
	}
	vaultProvider, err := vault.ProviderFor(*vaultKeySource, *vaultKeyFile)
	if err != nil {
		slog.Error("vault anahtar kaynagi", "err", err)
		os.Exit(1)
	}
	v, err := vault.OpenWith(vaultProvider)
	if err != nil {
		slog.Error("kimlik kasasi acilamadi", "err", err)
		os.Exit(1)
	}
	slog.Info("kimlik kasasi acildi", "anahtar_kaynagi", vaultProvider.Name())
	alerts.SetCrypter(v) // S22.14: Jira/ServiceNow API token'larını vault ile şifrele

	var sink server.TelemetrySink
	if q != nil {
		sink = q
	}
	var oidcOpts *server.OIDCOptions
	if cfg.OIDC.Issuer != "" {
		redirect := cfg.OIDC.RedirectURL
		if redirect == "" && *publicURL != "" {
			redirect = strings.TrimRight(*publicURL, "/") + "/api/auth/oidc/callback"
		}
		oidcOpts = &server.OIDCOptions{
			Issuer:       cfg.OIDC.Issuer,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			RedirectURL:  redirect,
			GroupRoles:   cfg.OIDC.GroupRoles,
			DefaultRole:  cfg.OIDC.DefaultRole,
		}
		slog.Info("SSO (OIDC) aktif", "issuer", cfg.OIDC.Issuer, "client_id", cfg.OIDC.ClientID)
	}
	srv := server.New(static, engine, st, *dbPath, alerts, geo, *authPassword, *enrollToken, *telemetryInterval, *agentPCAP, v, sink, oidcOpts)
	if q != nil {
		q.SetDeadLetterHook(srv.IngestDead) // C4: DLQ metriği (bazntms_ingest_dead_total)
	}
	// B5: WS origin izin listesi — -public-url + -tls-hosts. Bos ise tum
	// origin'ler kabul edilir (bugunku davranis) + uyari loglanir.
	var wsHosts []string
	if *publicURL != "" {
		wsHosts = append(wsHosts, *publicURL)
	}
	for _, h := range strings.Split(*tlsHosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			wsHosts = append(wsHosts, h)
		}
	}
	if len(wsHosts) > 0 {
		srv.SetWSOrigins(wsHosts)
		slog.Info("WS origin izin listesi aktif", "host_sayisi", len(wsHosts)+3)
	}
	if *publicURL != "" {
		srv.SetPublicURL(*publicURL) // agent kurulum sihirbazı bunu enroll hub adresi olarak kullanır
	}
	srv.SetMultiSite(*multiSite)
	if *multiSite {
		slog.Info("coklu-saha (MSP) modu aktif — site sert yetki siniri, site-bagli enroll token zorunlu")
	}
	srv.SetMockDevices(*mockDevices)
	if *mockDevices {
		slog.Warn("mock-devices AÇIK — vendor=mock cihaz eklenebilir (yalnizca olcek testi icin)")
	}
	if *sessionStore == "db" {
		srv.UseDBSessions(ctx)
		slog.Info("panel oturumlari paylasimli DB deposunda (coklu controller replikasi mumkun)")
	} else if *sessionStore != "memory" {
		slog.Warn("bilinmeyen -session-store degeri, memory kullaniliyor", "verilen", *sessionStore)
	}
	if autoTok := srv.EnrollToken(); *enrollToken == "" {
		slog.Info("otomatik bootstrap enrollment token uretildi", "enroll_token", autoTok)
	}
	// B7: statik token bootstrap icindir — sizarsa yeniden baslatmadan iptal
	// edilemez. Kalici token'lar panel > Yonetim > Agent Ekle'den yonetilir.
	slog.Info("enrollment: statik token yalnizca ilk kurulum icin — kalici/iptal-edilebilir token'lar icin panel > Yonetim > Agent Ekle")

	// mTLS: CA + sunucu sertifikasi (bkz. asagida ListenAndServeTLS)
	var tlsConf *tls.Config
	if *tlsOn {
		ca, err := pki.LoadOrCreateCA(*tlsDir)
		if err != nil {
			slog.Error("mTLS CA acilamadi", "dir", *tlsDir, "err", err)
			os.Exit(1)
		}
		srv.SetAgentCA(ca)
		var serverCert tls.Certificate
		if *tlsCert != "" {
			serverCert, err = tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		} else {
			hosts := []string{"localhost", "127.0.0.1", "::1"}
			for _, h := range strings.Split(*tlsHosts, ",") {
				if h = strings.TrimSpace(h); h != "" {
					hosts = append(hosts, h)
				}
			}
			if hn, _ := os.Hostname(); hn != "" {
				hosts = append(hosts, hn)
			}
			serverCert, err = ca.ServerTLSCertificate(hosts)
		}
		if err != nil {
			slog.Error("sunucu sertifikasi hazirlanamadi", "err", err)
			os.Exit(1)
		}
		tlsConf = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{serverCert},
			ClientAuth:   tls.VerifyClientCertIfGiven, // tarayici sertifikasiz baglanabilir; agent sertifikasi zorunlu dogrulanir
			ClientCAs:    ca.Pool(),
		}
		slog.Info("mTLS aktif", "ca_dir", *tlsDir, "operator_cert", *tlsCert != "")
	}
	// Agent guncelleme kanali. -update-github-repo doluysa (varsayilan)
	// hub, deponun en son release'ini periyodik cekip -updates-dir'e
	// manifest + binary olarak yazar; agent'lar bugunku gibi yalnizca
	// hub'dan indirir. Repo bos ise yalnizca elle hazirlanmis
	// (bazntmsctl update sign) dizin sunulur.
	updDir := *updatesDir
	if updDir == "" && *updateRepo != "" {
		updDir = "updates"
	}
	if updDir != "" {
		srv.SetUpdatesDir(updDir)
		if *updateRepo != "" {
			syncer := update.NewGitHubSyncer(*updateRepo, updDir, os.Getenv("GITHUB_TOKEN"))
			go syncer.Run(ctx, *updateSyncInterval)
			slog.Info("agent guncelleme: GitHub release senkronu aktif",
				"repo", *updateRepo, "dir", updDir, "interval", *updateSyncInterval)
		} else {
			slog.Info("agent guncelleme kanali aktif (statik dizin)", "dir", updDir)
		}
	}

	// 5651 uyumlu loglama (Faz 9)
	if *complianceOn {
		sealer, err := compliance.NewSealer(st, compliance.Config{
			Enabled:       true,
			TSAURL:        *tsaURL,
			SignKeyFile:   *complianceKey,
			WormDir:       *wormDir,
			RetentionDays: *complianceRetention,
		})
		if err != nil {
			slog.Error("compliance sealer baslatilamadi", "err", err)
			os.Exit(1)
		}
		sealer.Start()
		defer sealer.Stop()
		slog.Info("5651 log imzalama aktif",
			"tsa", *tsaURL != "", "imza", true, "worm", *wormDir != "",
			"retention_days", *complianceRetention)
	}
	srv.SetCompliance(server.ComplianceInfo{
		Enabled: *complianceOn, TSAURL: *tsaURL, SignKey: *complianceOn,
		WormDir: *wormDir, MaskPII: *maskPII, RetentionDays: *complianceRetention,
	})

	// cihaz SNMP poller
	poller := devpoll.New(st, v)
	poller.SetConcurrency(*devpollConcurrency)
	if *pollInterval > 0 {
		poller.SetInterval(time.Duration(*pollInterval) * time.Second)
	}
	if *pollerOn {
		leaderPoller := st.Leader(store.LeaderKeyPoller, "poller")
		go leaderPoller.Run(ctx)
		poller.SetLeaderCheck(leaderPoller.IsLeader)
		poller.Start()
		defer poller.Stop()
	} else {
		slog.Info("snmp poller kapali (coklu replika ingest modu)")
	}

	// zamanlanmış işler (S22.18) — lider-kapılı, controller replikasında.
	// İş türü handler'ları aşağıda kaydedilir (S22.19: rapor teslimi).
	if *alertsOn {
		sched := scheduler.New(st)
		leaderSched := st.Leader(store.LeaderKeyScheduler, "scheduler")
		go leaderSched.Run(ctx)
		sched.SetLeaderCheck(leaderSched.IsLeader)
		sched.Start()
		defer sched.Stop()
	}

	// NetFlow v5/v9 + IPFIX + sFlow v5 collector — kuyruk aciksa JetStream'e gider.
	// Ayni Collector her uc protokolu de datagram versiyonundan ayirir; -sflow-port
	// yalnizca sFlow'u farkli bir portta da dinlemek isteyenler icin ikinci bir bind.
	onFlows := func(device string, rows []flows.Row) {
		srows := make([]store.FlowRow, 0, len(rows))
		for _, f := range rows {
			srows = append(srows, store.FlowRow(f))
		}
		if q != nil {
			if err := q.PublishFlows(srows); err != nil {
				slog.Error("akis kuyruguna yayinlama hatasi", "err", err)
			}
			return
		}
		if err := st.SaveFlows(srows); err != nil {
			slog.Error("akis kaydi hatasi", "err", err)
		}
	}
	startFlowCollector := func(port, label string) {
		flc := &flows.Collector{ExporterIP: *flowExporter, OnFlows: onFlows}
		if err := flc.Listen("0.0.0.0:" + port); err != nil {
			slog.Error("flow collector dinlenemedi", "port", port, "err", err)
			return
		}
		slog.Info(label+" collector dinliyor", "port", port, "exporter_override", *flowExporter)
		defer flc.Close()
		<-ctx.Done()
	}
	if *flowPort != "" {
		go startFlowCollector(*flowPort, "netflow/ipfix/sflow")
	}
	if *sflowPort != "" && *sflowPort != *flowPort {
		go startFlowCollector(*sflowPort, "sflow")
	}

	// Syslog + NetFlow alicilari coklu replikada calisabilir (C1/S15.7): bir
	// exporter tek adrese gonderir, k8s Service datagram'i tek pod'a yonlendirir;
	// alicilar yalnizca kuyruga yazar, store-writer mesaji bir kez isler.
	// Uyum logu SENKRON burada eklenir (durabilite) — kuyruk yolu tekrarlamaz.
	// Kuyruk kapaliysa (-nats bos) alicilar dogrudan store'a yazar → tek node.
	if *syslogPort != "" {
		sl := &syslogd.Listener{OnEvent: func(srcIP string, ev syslogd.Event) {
			se := store.SyslogEvent{
				Ts: time.Now().Unix(), Host: ev.Host, SourceIP: srcIP, Severity: ev.Severity,
				Tag: ev.Tag, Message: ev.Message,
			}
			if q != nil {
				if err := q.PublishSyslog(se); err != nil {
					slog.Error("syslog kuyruguna yayinlama hatasi", "err", err)
				}
			} else {
				if err := st.SaveSyslogEvent(se); err != nil {
					slog.Error("syslog kaydi hatasi", "err", err)
				}
			}
			if _, err := st.AppendComplianceLog(store.ComplianceLog{
				Ts: se.Ts, SourceType: "syslog", SourceName: ev.Host,
				SrcIP: srcIP, SrcMAC: store.ExtractMAC(ev.Message), Category: "syslog",
				Message: fmt.Sprintf("[%d] %s: %s", ev.Severity, ev.Tag, ev.Message),
			}); err != nil {
				slog.Error("compliance log hatasi", "err", err)
			}
		}}
		if err := sl.Listen("0.0.0.0:" + *syslogPort); err != nil {
			slog.Error("syslog dinlenemedi", "port", *syslogPort, "err", err)
		} else {
			slog.Info("syslog alici dinliyor", "port", *syslogPort)
			defer sl.Close()
		}
	}

	if *pprofAddr != "" {
		if *pprofRates > 0 {
			runtime.SetBlockProfileRate(*pprofRates)
			runtime.SetMutexProfileFraction(*pprofRates)
			slog.Info("block + mutex profillemesi acik", "rate", *pprofRates)
		}
		go func() {
			slog.Info("pprof dinleniyor", "addr", *pprofAddr)
			ps := &http.Server{Addr: *pprofAddr, ReadHeaderTimeout: 10 * time.Second}
			if err := ps.ListenAndServe(); err != nil {
				slog.Warn("pprof sunucu hatasi", "err", err)
			}
		}()
	}

	addr := "0.0.0.0:" + *port
	// ReadHeaderTimeout: Slowloris'e karsi (G112). ReadTimeout/WriteTimeout
	// bilerek ayarlanmadi — WS akislari ve uzun rapor indirmeleri var.
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		TLSConfig:         tlsConf,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		scheme := "http"
		serve := hs.ListenAndServe
		if tlsConf != nil {
			scheme = "https (mTLS)"
			serve = func() error { return hs.ListenAndServeTLS("", "") }
		}
		slog.Info(scheme+" dinleniyor", "addr", addr)
		if err := serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("sunucu hatasi", "err", err)
			stop() // graceful shutdown akisini tetikle ve cik
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("kapanis sinyali alindi, graceful shutdown")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown hatasi", "err", err)
	}
}
