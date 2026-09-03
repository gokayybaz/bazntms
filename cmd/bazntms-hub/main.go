// bazntms-hub — merkezi sunucu: ingest, REST/WS API, dashboard, uyarılar,
// AI analizi ve rapor motoru. Faz 4 ile olcek altyapisi: SQLite veya
// PostgreSQL/TimescaleDB depo, opsiyonel NATS JetStream kuyrugu, graceful
// shutdown ve coklu-replika rolleri (capture/alerts/poller anahtarlari).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // -pprof adresinde DefaultServeMux'a kaydolur
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/compliance"
	"github.com/gokayybaz/bazntms/internal/config"
	"github.com/gokayybaz/bazntms/internal/devpoll"
	"github.com/gokayybaz/bazntms/internal/flows"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/logging"
	"github.com/gokayybaz/bazntms/internal/queue"
	"github.com/gokayybaz/bazntms/internal/server"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/syslogd"
	"github.com/gokayybaz/bazntms/internal/vault"
	"github.com/gokayybaz/bazntms/internal/version"
	"github.com/gokayybaz/bazntms/web"
)

func main() {
	fl := flag.NewFlagSet("bazntms-hub", flag.ExitOnError)
	port := fl.String("port", "8080", "HTTP sunucu portu")
	dev := fl.Bool("dev", false, "frontend embed'i atla (vite dev server ile gelistirme)")
	dbPath := fl.String("db", "bazntms.db", "SQLite dosyasi veya postgres:// DSN (Faz 4.1)")
	retentionH := fl.Int("retention-hours", 24*7, "veritabani saklama suresi (saat, SQLite/Prune modu)")
	natsURL := fl.String("nats", "", "NATS JetStream adresi (bos = kuyruk kapali, dogrudan yazim; ex: nats://localhost:4222)")
	captureOn := fl.Bool("capture", true, "hub'in kendi paket yakalamasi (coklu replikada kapatilir)")
	alertsOn := fl.Bool("alerts", true, "uyari motoru (coklu replikada tek replikada acilir)")
	pollerOn := fl.Bool("poller", true, "SNMP cihaz poller'i (coklu replikada tek replikada acilir)")
	pruneOn := fl.Bool("prune", true, "veritabani bakimi (eski satirlarin temizligi); coklu replikada YALNIZCA bir hub'da acik olmali")
	pollInterval := fl.Int("poll-interval", 0, "tum cihazlar icin tek tip poll araligi (sn); 0 = per-device deger. min 5")
	pprofAddr := fl.String("pprof", "", "net/http/pprof dinleme adresi (ex: 127.0.0.1:6060); bos = kapali")
	geoipDir := fl.String("geoip-dir", "geoip", "MaxMind GeoLite2 .mmdb dosyalarinin dizini")
	ipAPILookup := fl.Bool("ip-api-lookup", true, "MMDB yoksa ip-api.com ile IP cozumleme")
	authPassword := fl.String("auth-password", "", "Arayuz sifresi (bos ise kimlik dogrulama kapali; AUTH_PASSWORD de gecerli)")
	configPath := fl.String("config", "", "YAML config dosyasi (bayraklar ustunlukte)")
	logLevel := fl.String("log-level", "", "log seviyesi: debug|info|warn|error (config'i override eder)")
	logFormat := fl.String("log-format", "", "log formati: json|text (config'i override eder)")
	enrollToken := fl.String("enroll-token", "", "agent enrollment token'i (bos ise rastgele uretilir ve loglanir)")
	telemetryInterval := fl.Int("telemetry-interval", 30, "agent telemetri araligi (saniye)")
	agentPCAP := fl.Bool("agent-pcap", false, "agent'larda derin toplama ve PCAP kaydina izin ver (politika)")
	vaultKeyFile := fl.String("vault-key-file", "vault.key", "Kimlik kasasi master key dosyasi (yoksa uretilir)")
	flowPort := fl.String("flow-port", "", "NetFlow v5 UDP dinleme portu (bos = kapali; ex: 2055)")
	flowExporter := fl.String("flow-exporter", "", "NetFlow exporter IP override — hub bir NAT/röle arkasindaysa (ör. Docker Desktop) paketin kaynak IP'si kaybolur; tek exporter'li kurulumda cihazin IP'sini yazin")
	syslogPort := fl.String("syslog-port", "", "Syslog UDP dinleme portu (bos = kapali; ex: 5514)")
	updatesDir := fl.String("updates-dir", "", "Agent guncelleme kanali dizini (bos = kapali; icerik: bazntmsctl update sign)")
	complianceOn := fl.Bool("compliance", false, "5651 log imzalama motoru (Faz 9): hash-zincir + Merkle checkpoint + gunluk muhur")
	tsaURL := fl.String("tsa-url", "", "RFC 3161 zaman damgasi servisi (TSA) adresi")
	complianceKey := fl.String("compliance-key", "compliance.key", "Manifest imza anahtari (ed25519 PEM; yoksa uretilir)")
	wormDir := fl.String("worm-dir", "", "Gunluk imzali log paketi dizini (WORM)")
	maskPII := fl.Bool("mask-pii", false, "Delil paketinde PII maskeleme (A.5.34)")
	complianceRetention := fl.Int("compliance-retention-days", 730, "Ham uyum logu saklama suresi (gun; 5651 minimum 730)")
	showVersion := fl.Bool("version", false, "surum bilgisini yaz ve cik")
	fl.Parse(os.Args[1:])

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
	defer st.Close()

	retention := time.Duration(*retentionH) * time.Hour
	// veritabani bakimi: eski satirlarin temizligi + (TS'te) native chunk-drop
	// retention. Collector'dan bagimsiz (coklu-hub'da -capture=false).
	if *pruneOn {
		if err := st.ConfigureRetention(retention); err != nil {
			slog.Warn("retention politikalari kurulamadi", "err", err)
		}
		maint := store.NewMaintainer(st, retention)
		maint.Start()
		defer maint.Stop()
		slog.Info("veritabani bakimi aktif", "retention_saat", *retentionH, "aralik", "15dk")
	} else {
		slog.Info("veritabani bakimi bu replikada kapali (-prune=false)")
	}

	// NATS JetStream kuyrugu (Faz 4.2): ingest → processor ayrismasi
	var q *queue.Queue
	if *natsURL != "" {
		q, err = queue.Connect(*natsURL)
		if err != nil {
			slog.Error("nats baglantisi kurulamadi", "url", *natsURL, "err", err)
			os.Exit(1)
		}
		defer q.Close()
		if err := q.RunProcessor(ctx, st, 4); err != nil {
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
	if *alertsOn {
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
	v, err := vault.Open(*vaultKeyFile)
	if err != nil {
		slog.Error("kimlik kasasi acilamadi", "err", err)
		os.Exit(1)
	}

	var sink server.TelemetrySink
	if q != nil {
		sink = q
	}
	var oidcOpts *server.OIDCOptions
	if cfg.OIDC.Issuer != "" {
		oidcOpts = &server.OIDCOptions{
			Issuer:       cfg.OIDC.Issuer,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			RedirectURL:  cfg.OIDC.RedirectURL,
			GroupRoles:   cfg.OIDC.GroupRoles,
			DefaultRole:  cfg.OIDC.DefaultRole,
		}
		slog.Info("SSO (OIDC) aktif", "issuer", cfg.OIDC.Issuer, "client_id", cfg.OIDC.ClientID)
	}
	srv := server.New(static, engine, st, *dbPath, alerts, geo, *authPassword, *enrollToken, *telemetryInterval, *agentPCAP, v, sink, oidcOpts)
	if autoTok := srv.EnrollToken(); *enrollToken == "" {
		slog.Info("otomatik enrollment token uretildi", "enroll_token", autoTok)
	}
	if *updatesDir != "" {
		srv.SetUpdatesDir(*updatesDir)
		slog.Info("agent guncelleme kanali aktif", "dir", *updatesDir)
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
	if *pollInterval > 0 {
		poller.SetInterval(time.Duration(*pollInterval) * time.Second)
	}
	if *pollerOn {
		poller.Start()
		defer poller.Stop()
	} else {
		slog.Info("snmp poller kapali (coklu replika ingest modu)")
	}

	// NetFlow v5 collector — kuyruk aciksa mesajlar JetStream'e gider
	if *flowPort != "" {
		flc := &flows.Collector{ExporterIP: *flowExporter, OnFlows: func(device string, rows []flows.Row) {
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
		}}
		if err := flc.Listen("0.0.0.0:" + *flowPort); err != nil {
			slog.Error("flow collector dinlenemedi", "port", *flowPort, "err", err)
		} else {
			slog.Info("netflow v5 collector dinliyor", "port", *flowPort, "exporter_override", *flowExporter)
			defer flc.Close()
		}
	}

	// Syslog alici — kuyruk aciksa mesajlar JetStream'e gider; uyum loglari
	// her iki yolda da compliance_logs zincirine eklenir (Faz 9.1)
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
		go func() {
			slog.Info("pprof dinleniyor", "addr", *pprofAddr)
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				slog.Warn("pprof sunucu hatasi", "err", err)
			}
		}()
	}

	addr := "0.0.0.0:" + *port
	hs := &http.Server{Addr: addr, Handler: srv.Handler()}
	go func() {
		slog.Info("http dinleniyor", "addr", addr)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http sunucu hatasi", "err", err)
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
