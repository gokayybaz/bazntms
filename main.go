package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/server"
	"github.com/gokayybaz/bazntms/internal/store"
)

//go:embed all:frontend/dist
var distFS embed.FS

var version = "dev" // release ldflags ile set edilir

func main() {
	port := flag.String("port", "8080", "HTTP sunucu portu")
	dev := flag.Bool("dev", false, "frontend embed'i atla (vite dev server ile gelistirme)")
	dbPath := flag.String("db", "bazntms.db", "SQLite veritabani dosyasi")
	retentionH := flag.Int("retention-hours", 24*7, "veritabani saklama suresi (saat)")
	llmURL := flag.String("llm-base-url", "", "AI servisi adresi (OpenAI-uyumlu; ex: http://localhost:11434/v1)")
	llmKey := flag.String("llm-api-key", "", "AI API anahtari (yerel modeller icin gerekmez)")
	llmModel := flag.String("llm-model", "", "varsayilan model (ex: llama3.2, qwen2.5:7b)")
	llmMaxTokens := flag.Int("llm-max-tokens", 0, "istek basina token limiti (0=dahili varsayilan; reasoning modellerde dusunme yetersiz kalirsa artirin)")
	llmNoThink := flag.Bool("llm-no-think", false, "Qwen3 tarzi modellerde dusunme modunu kapat (/no_think)")
	recordDir := flag.String("record-dir", "captures", "PCAP kayit dosyalarinin dizini")
	recordMaxMB := flag.Int("record-max-mb", 100, "PCAP dosyasi basina maksimum boyut (MB, rotasyon siniri)")
	geoipDir := flag.String("geoip-dir", "geoip", "MaxMind GeoLite2 .mmdb dosyalarinin dizini")
	ipAPILookup := flag.Bool("ip-api-lookup", true, "MMDB yoksa ip-api.com ile IP cozumleme (internet kullanir; kapatmak icin -ip-api-lookup=false)")
	authPassword := flag.String("auth-password", "", "Arayuz sifresi (bos ise kimlik dogrulama kapali; AUTH_PASSWORD ortam degiskeni de kullanilabilir)")
	flag.Parse()

	var static fs.FS
	if !*dev {
		sub, err := fs.Sub(distFS, "frontend/dist")
		if err != nil {
			log.Fatal(err)
		}
		static = sub
	} else {
		fmt.Println(">> dev modu: statik dosyalar serve edilmiyor, vite dev server kullanin")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("veritabani acilamadi: %v", err)
	}
	defer st.Close()

	engine := capture.NewEngine()
	engine.SetRecordOptions(*recordDir, uint64(*recordMaxMB)<<20)
	collector := store.NewCollector(engine, st, *dbPath, time.Duration(*retentionH)*time.Hour)
	collector.Start()
	defer collector.Stop()

	// uyarilar: DB'de kayitli ayar yoksa varsayilanlari kullan
	alertCfg := alert.DefaultConfig()
	if raw, err := st.LoadAlertConfig(); err == nil && raw != "" {
		if json.Unmarshal([]byte(raw), &alertCfg) != nil {
			log.Printf("uyari ayarlari okunamadi, varsayilanlar kullaniliyor")
			alertCfg = alert.DefaultConfig()
		}
	}
	alerts := alert.NewManager(alertCfg, st, engine)
	alerts.Start()
	defer alerts.Stop()

	aiCfg := ai.ConfigFromEnv()
	if *llmURL != "" {
		aiCfg.BaseURL = strings.TrimRight(*llmURL, "/")
	}
	if *llmKey != "" {
		aiCfg.APIKey = *llmKey
	}
	if *llmModel != "" {
		aiCfg.Model = *llmModel
	}
	if *llmMaxTokens > 0 {
		aiCfg.MaxTokens = *llmMaxTokens
	}
	if *llmNoThink {
		aiCfg.NoThink = true
	}
	aiClient := ai.NewClient(aiCfg)

	// GeoIP: once yerel MMDB aranir, yoksa ip-api.com toplu servisi devreye girer
	geo := geoip.New(
		filepath.Join(*geoipDir, "GeoLite2-Country.mmdb"),
		filepath.Join(*geoipDir, "GeoLite2-ASN.mmdb"),
		*ipAPILookup,
	)
	if aiCfg.Enabled() {
		fmt.Printf(">> AI aktif: %s (%s)\n", aiCfg.Model, aiCfg.BaseURL)
	} else {
		fmt.Println(">> AI pasif: -llm-base-url http://localhost:11434/v1 (Ollama) ile ya da LLM_API_KEY ile baslatin")
	}

	if authPassword == nil || *authPassword == "" {
		if env := os.Getenv("AUTH_PASSWORD"); env != "" {
			*authPassword = env
		}
	}
	if *authPassword == "" {
		fmt.Println(">> UYARI: kimlik dogrulama kapali — LAN uzerinden herkes erisebilir. -auth-password ile sifre belirleyin")
	}

	srv := server.New(static, engine, st, *dbPath, aiClient, alerts, geo, *authPassword)
	addr := "0.0.0.0:" + *port
	fmt.Println()
	fmt.Printf("  bazNTMS %s — baz Network Traffic Monitoring System\n", version)
	fmt.Printf("  http://localhost:%s\n", *port)
	fmt.Printf("  veritabani: %s (saklama: %d saat)\n", *dbPath, *retentionH)
	fmt.Println()
	fmt.Println("  Not: paket yakalama icin yonetici (sudo/admin) yetkisi ve")
	fmt.Println("  platforma uygun pcap destegi gerekir:")
	fmt.Println("   - macOS/Linux : sudo ile calistirin")
	fmt.Println("   - Windows     : Npcap kurulu olmali (https://npcap.com)")
	fmt.Println()

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
