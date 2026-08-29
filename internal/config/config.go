// Package config, hub ve agent icin YAML + ortam degiskeni tabanli
// yapilandirma sunar. Oncelik sirasi:
//
//	bayrak (acikca set edilmis) > ortam degiskeni (BAZNTMS_*) > YAML dosyasi > bayrak varsayilani
//
// Ortam degiskeni esleme: BAZNTMS_ on eki + "__" ile ic ice anahtar:
// BAZNTMS_AUTH__PASSWORD -> auth.password, BAZNTMS_RECORD__MAX_MB -> record.max_mb
package config

import (
	"flag"
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	envPrefix = "BAZNTMS_"
	envSep    = "__"
)

// HubConfig, bazntms-hub icin YAML dosyasi semasidir. Alan adlari mevcut
// bayraklarin uzun adlariyla birebir eslesir (database.path <- -db hariç
// eslenmis takma adlar asagida belirtilir).
type HubConfig struct {
	Port     int  `koanf:"port"`
	Dev      bool `koanf:"dev"`
	Database struct {
		Path string `koanf:"path"`
	} `koanf:"database"`
	RetentionHours int `koanf:"retention_hours"`
	Auth           struct {
		Password string `koanf:"password"`
	} `koanf:"auth"`
	LLM struct {
		BaseURL   string `koanf:"base_url"`
		APIKey    string `koanf:"api_key"`
		Model     string `koanf:"model"`
		MaxTokens int    `koanf:"max_tokens"`
		NoThink   bool   `koanf:"no_think"`
	} `koanf:"llm"`
	Record struct {
		Dir   string `koanf:"dir"`
		MaxMB int    `koanf:"max_mb"`
	} `koanf:"record"`
	GeoIP struct {
		Dir         string `koanf:"dir"`
		IPAPILookup bool   `koanf:"ip_api_lookup"`
	} `koanf:"geoip"`
	Log struct {
		Level  string `koanf:"level"`
		Format string `koanf:"format"`
	} `koanf:"log"`
}

// AgentConfig, bazntms-agent icin YAML dosyasi semasidir (Faz 1'de
// telemetri ayarlari genisler).
type AgentConfig struct {
	Hub struct {
		URL   string `koanf:"url"`
		Token string `koanf:"token"`
	} `koanf:"hub"`
	Agent struct {
		Name string `koanf:"name"`
		Site string `koanf:"site"`
	} `koanf:"agent"`
	Collect struct {
		IntervalSeconds int    `koanf:"interval_seconds"`
		PCAP            bool   `koanf:"pcap"`           // surec atfi icin paket yakalama (root/admin gerekir)
		PCAPInterface   string `koanf:"pcap_interface"` // bos = otomatik secim
		PCAPRecord      bool   `koanf:"pcap_record"`    // ham paketleri diske yaz (hub politikasi da acik olmali)
		PCAPDir         string `koanf:"pcap_dir"`       // PCAP kayit dizini
	} `koanf:"collect"`
	Log struct {
		Level  string `koanf:"level"`
		Format string `koanf:"format"`
	} `koanf:"log"`
}

// hubFlagKeys, YAML/ortam anahtarlarindan bayrak adina esleme tablosudur.
var hubFlagKeys = map[string]string{
	"port":                "port",
	"dev":                 "dev",
	"database.path":       "db",
	"retention_hours":     "retention-hours",
	"auth.password":       "auth-password",
	"llm.base_url":        "llm-base-url",
	"llm.api_key":         "llm-api-key",
	"llm.model":           "llm-model",
	"llm.max_tokens":      "llm-max-tokens",
	"llm.no_think":        "llm-no-think",
	"record.dir":          "record-dir",
	"record.max_mb":       "record-max-mb",
	"geoip.dir":           "geoip-dir",
	"geoip.ip_api_lookup": "ip-api-lookup",
}

// LoadHub, -config ile verilen YAML dosyasini (varsa) yukler, BAZNTMS_* ortam
// degiskenlerini uygular ve acikca set edilmemis bayraklari config
// degerleriyle gunceller. Bayrak ustunlugu korunur.
func LoadHub(fs *flag.FlagSet, configPath string) (*HubConfig, error) {
	k, err := load(configPath)
	if err != nil {
		return nil, err
	}
	var cfg HubConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("hub config: %w", err)
	}

	// acikca set edilmemis bayraklari config degerleriyle esle
	changed := map[string]struct{}{}
	fs.Visit(func(f *flag.Flag) { changed[f.Name] = struct{}{} })
	for key, flagName := range hubFlagKeys {
		if _, ok := changed[flagName]; ok {
			continue
		}
		if v := k.String(key); v != "" {
			if err := fs.Set(flagName, v); err != nil {
				return nil, fmt.Errorf("config -> flag %s: %w", flagName, err)
			}
		}
	}
	return &cfg, nil
}

// LoadAgent, agent YAML dosyasini ve BAZNTMS_* ortam degiskenlerini yukler.
func LoadAgent(configPath string) (*AgentConfig, error) {
	k, err := load(configPath)
	if err != nil {
		return nil, err
	}
	var cfg AgentConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("agent config: %w", err)
	}
	return &cfg, nil
}

func load(configPath string) (*koanf.Koanf, error) {
	k := koanf.New(".")

	// YAML once (dosya), ardindan ortam degiskenleri → env ustunluk kazanir
	if configPath != "" {
		if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("config dosyasi %s: %w", configPath, err)
		}
	}

	// ortam degiskenleri: BAZNTMS_AUTH__PASSWORD -> auth.password
	if err := k.Load(env.Provider(envPrefix, ".", func(s string) string {
		s = strings.TrimPrefix(s, envPrefix)
		return strings.ReplaceAll(strings.ToLower(s), envSep, ".")
	}), nil); err != nil {
		return nil, err
	}
	return k, nil
}
