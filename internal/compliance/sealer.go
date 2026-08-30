package compliance

// Sealer — Faz 9.2: saatlik Merkle checkpoint'leri, günlük mühür
// (TSA + imza + WORM paketi) ve zaman sapması alarmı (A.8.17).

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Config, sealer yapılandırması.
type Config struct {
	Enabled        bool   // zincir + checkpoint'ler
	TSAURL         string // boş → zaman damgası atlanır (status: none)
	SignKeyFile    string // boş → manifest imzası atlanır
	WormDir        string // boş → WORM paketi yazılmaz
	RetentionDays  int    // ham log saklama (varsayılan 730 = 5651 2 yıl)
	CheckpointHour bool   // saatlik checkpoint üret (testlerde kapatılabilir)
}

// Sealer, checkpoint/mühür döngüsünü yürütür.
type Sealer struct {
	st      store.Store
	cfg     Config
	tsa     *TSAClient
	signKey ed25519.PrivateKey
	pubHex  string

	stop chan struct{}
	done chan struct{}
}

// NewSealer, sealer'ı hazırlar; imza anahtarı varsa yükler/üretir.
func NewSealer(st store.Store, cfg Config) (*Sealer, error) {
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 730
	}
	s := &Sealer{st: st, cfg: cfg, stop: make(chan struct{}), done: make(chan struct{})}
	if cfg.TSAURL != "" {
		s.tsa = NewTSAClient(cfg.TSAURL, 0)
	}
	if cfg.SignKeyFile != "" {
		priv, pub, err := LoadOrCreateSignKey(cfg.SignKeyFile)
		if err != nil {
			return nil, fmt.Errorf("imza anahtarı: %w", err)
		}
		s.signKey = priv
		s.pubHex = pub
	}
	return s, nil
}

// Start, mühürleme döngüsünü başlatır (30 sn denetim aralığı).
func (s *Sealer) Start() {
	go s.run()
}

// Stop, döngüyü kapatır.
func (s *Sealer) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Sealer) run() {
	defer close(s.done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			now := time.Now()
			// saatlik checkpoint: bir önceki saat dilimi
			hourStart := now.Truncate(time.Hour).Add(-time.Hour)
			if err := s.HourlyCheckpoint(context.Background(), hourStart); err != nil {
				slog.Warn("saatlik checkpoint hatası", "err", err)
			}
			// günlük mühür: dün (00:05'ten sonra)
			if now.Hour() == 0 && now.Minute() >= 5 {
				day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
				if err := s.DailySeal(context.Background(), day); err != nil {
					slog.Warn("günlük mühür hatası", "err", err)
				}
			}
			// retention: günlük bir
			if now.Hour() == 3 && now.Minute() < 1 {
				if err := s.st.PruneComplianceLogs(s.cfg.RetentionDays); err != nil {
					slog.Warn("compliance retention", "err", err)
				}
			}
		}
	}
}

// HourlyCheckpoint, [hour, hour+1h) diliminin Merkle kökünü zincirler.
// Boş dilim atlanır; mevcut checkpoint idempotent şekilde yeniden üretilmez.
func (s *Sealer) HourlyCheckpoint(ctx context.Context, hour time.Time) error {
	start := hour.Truncate(time.Hour)
	end := start.Add(time.Hour)
	if exists, err := s.st.CheckpointExists("hourly", start.Unix()); err != nil || exists {
		return err
	}
	hashes, first, last, count, err := s.st.ComplianceHashesBetween(start.Unix(), end.Unix())
	if err != nil {
		return err
	}
	if count == 0 {
		return nil // boş saat: checkpoint üretme
	}
	root := MerkleRoot(hashes)
	prevCp, err := s.st.LatestLogCheckpoint("hourly")
	if err != nil {
		return err
	}
	prev := ""
	if prevCp != nil {
		prev = prevCp.Root
	}
	// zaman sapması tespiti (A.8.17): kayıt saati dilim gelecekteyse saat geri alınmış demektir
	if end.Unix() > time.Now().Unix()+3600 {
		s.st.InsertAlertEvent(store.AlertEvent{
			Ts: time.Now().Unix(), Kind: "time_drift", Key: "compliance",
			Message: fmt.Sprintf("Zaman sapması şüphesi: saatlik dilim sistem saatinden ileride (%s) — NTP senkronizasyonunu doğrulayın", end.Format(time.RFC3339)),
		})
	}
	_, err = s.st.SaveLogCheckpoint(store.LogCheckpoint{
		Kind: "hourly", BucketStart: start.Unix(), BucketEnd: end.Unix(),
		RecordCount: count, PrevRoot: prev, Root: fmt.Sprintf("%x", root),
		SignedAt: first + last, // bilgi amaçlı: ilk+son seq
	})
	return err
}

// manifest, günlük mühür imzalanan içeriktir.
type manifest struct {
	Day         string `json:"day"` // YYYY-MM-DD
	Root        string `json:"root"`
	RecordCount int    `json:"record_count"`
	Hours       int    `json:"hours"`
	PrevDaily   string `json:"prev_daily"`
	CreatedAt   int64  `json:"created_at"`
}

// DailySeal, günün saatlik köklerinden günlük kök üretir, TSA + imza uygular
// ve WORM paketini yazar. İdempotenttir.
func (s *Sealer) DailySeal(ctx context.Context, day time.Time) error {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	if exists, err := s.st.CheckpointExists("daily", dayStart.Unix()); err != nil || exists {
		return err
	}

	// günün saatlik checkpoint'leri
	cps, err := s.st.LogCheckpointsBetween(dayStart.Unix(), dayStart.Add(24*time.Hour).Unix())
	if err != nil {
		return err
	}
	var hourlyRoots [][]byte
	recordCount := 0
	for _, cp := range cps {
		if cp.Kind != "hourly" {
			continue
		}
		b, _ := fromHex(cp.Root)
		hourlyRoots = append(hourlyRoots, b)
		recordCount += cp.RecordCount
	}
	if len(hourlyRoots) == 0 {
		return nil // logsuz gün: mühürleme yok
	}
	root := MerkleRoot(hourlyRoots)

	prevDaily, err := s.st.LatestLogCheckpoint("daily")
	if err != nil {
		return err
	}
	prev := ""
	if prevDaily != nil {
		prev = prevDaily.Root
	}

	now := time.Now().Unix()
	m := manifest{
		Day: dayStart.Format("2006-01-02"), Root: fmt.Sprintf("%x", root),
		RecordCount: recordCount, Hours: len(hourlyRoots), PrevDaily: prev,
		CreatedAt: now,
	}
	mBytes, _ := json.Marshal(m)

	// TSA zaman damgası
	tsaStatus := "none"
	var token []byte
	if s.tsa != nil {
		h := sha256Sum(root)
		tok, _, err := s.tsa.Timestamp(ctx, h)
		if err != nil {
			tsaStatus = "error:" + truncateStr(err.Error(), 120)
			slog.Warn("tsa zaman damgası alınamadı", "err", err)
		} else {
			tsaStatus = "ok"
			token = tok
		}
	}

	// manifest imzası
	sig := ""
	if s.signKey != nil {
		sig = SignManifest(s.signKey, mBytes)
	}

	cp := store.LogCheckpoint{
		Kind: "daily", BucketStart: dayStart.Unix(), BucketEnd: dayStart.Add(24 * time.Hour).Unix(),
		RecordCount: recordCount, PrevRoot: prev, Root: fmt.Sprintf("%x", root),
		TSAStatus: tsaStatus, TSAToken: token, Signature: sig, SignedAt: now,
	}
	if _, err := s.st.SaveLogCheckpoint(cp); err != nil {
		return err
	}

	// WORM paketi: <dir>/bazntms-logs-YYYY-MM-DD.jsonl.gz + manifest.json
	if s.cfg.WormDir != "" {
		if err := s.exportWorm(dayStart, cp); err != nil {
			slog.Warn("worm paketi yazılamadı", "err", err)
		}
	}
	slog.Info("günlük mühür tamam", "day", m.Day, "root", m.Root[:16], "tsa", tsaStatus, "imza", sig != "")
	return nil
}

// exportWorm, günün loglarını + manifest'i WORM dizinine yazar.
func (s *Sealer) exportWorm(dayStart time.Time, cp store.LogCheckpoint) error {
	day := dayStart.Format("2006-01-02")
	dir := filepath.Join(s.cfg.WormDir, dayStart.Format("2006"), dayStart.Format("01"))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	logs, err := s.st.ComplianceLogsBetween(dayStart.Unix(), dayStart.Add(24*time.Hour).Unix())
	if err != nil {
		return err
	}
	// JSONL kayıtlar (satır satır, zaten eklenen dosyanın üzerine yazılmaz:
	// aynı gün dosyası varsa O_EXCL ile atlanır — WORM davranışı)
	jsonlPath := filepath.Join(dir, fmt.Sprintf("bazntms-logs-%s.jsonl", day))
	f, err := os.OpenFile(jsonlPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		if os.IsExist(err) {
			return nil // zaten yazılmış
		}
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range logs {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	// manifest (checkpoint + token)
	mBytes, _ := json.MarshalIndent(cp, "", "  ")
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("bazntms-manifest-%s.json", day)), mBytes, 0o640)
}
