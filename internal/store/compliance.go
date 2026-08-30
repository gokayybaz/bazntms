package store

// 5651 uyumlu loglama (Faz 9.1): hash-zincirli log kayıtları, Merkle
// checkpoint'leri ve inceleme tutanakları.
//
// İki katmanlı imza modeli:
//   1. Kayıt düzeyi: her kayıt önceki kaydın hash'ini taşır (prev_hash) —
//      sıra bütünlüğü; aynı saatteki kayıt hash'lerinden Merkle kökü alınır
//   2. Günlük düzeyi: günün saatlik köklerinden günlük kök → RFC 3161
//      zaman damgası + ed25519 manifest imzası → WORM paketi (compliance paketi)
//
// Kayıtlar normal Prune kapsamına girmez; retention sealer tarafından
// compliance gün sayısıyla (varsayılan 730) yönetilir.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// macRe, log mesajlarından MAC adresi çıkarımı için (5651 m.5/b verisi).
var macRe = regexp.MustCompile(`([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}`)

// ExtractMAC, log mesajındaki ilk MAC adresini bulur (yoksa boş).
func ExtractMAC(message string) string {
	if m := macRe.FindString(message); m != "" {
		return strings.ToLower(m)
	}
	return ""
}

// ComplianceLog, zincirli tek log kaydı.
type ComplianceLog struct {
	Seq        int64  `json:"seq"`
	Ts         int64  `json:"ts"`          // unix — kayıt anı (NTP senkron sisteme güvenilir)
	SourceType string `json:"source_type"` // syslog | device | agent | manual
	SourceName string `json:"source_name"`
	SrcIP      string `json:"src_ip"`
	SrcMAC     string `json:"src_mac"`
	UserID     string `json:"user_id"`
	Category   string `json:"category"` // event | traffic | security | review | ...
	Message    string `json:"message"`
	PrevHash   string `json:"prev_hash"`
	Hash       string `json:"hash"`
}

// ComplianceHash, zincir hesabı: sha256(prev | ts | kaynaklar | mesaj).
func ComplianceHash(prev string, e ComplianceLog) string {
	h := sha256.New()
	h.Write([]byte(prev))
	fmt.Fprintf(h, "|%d|%s|%s|%s|%s|%s|%s|%s",
		e.Ts, e.SourceType, e.SourceName, e.SrcIP, e.SrcMAC, e.UserID, e.Category, e.Message)
	return hex.EncodeToString(h.Sum(nil))
}

// AppendComplianceLog, kaydı zincire ekler (eşzamanlı yazımda kilitli).
func (s *sqlStore) AppendComplianceLog(e ComplianceLog) (int64, error) {
	s.complianceMu.Lock()
	defer s.complianceMu.Unlock()

	var prev string
	err := s.db.QueryRow(s.q(`SELECT hash FROM compliance_logs ORDER BY seq DESC LIMIT 1`)).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	e.PrevHash = prev
	e.Hash = ComplianceHash(prev, e)

	var seq int64
	err = s.db.QueryRow(s.q(`INSERT INTO compliance_logs
		(ts, source_type, source_name, src_ip, src_mac, user_id, category, message, prev_hash, hash)
		VALUES (?,?,?,?,?,?,?,?,?,?) RETURNING seq`),
		e.Ts, e.SourceType, e.SourceName, e.SrcIP, e.SrcMAC, e.UserID, e.Category, e.Message,
		e.PrevHash, e.Hash).Scan(&seq)
	return seq, err
}

// ComplianceLogEntry, aralıktaki zincir doğrulaması için kayıt + hash.
type ComplianceLogEntry struct {
	Seq      int64
	Ts       int64
	PrevHash string
	Hash     string
}

// ComplianceHashesBetween, [from,to) aralığındaki kayıt hash'lerini sırayla
// dondurur (Merkle girdisi).
func (s *sqlStore) ComplianceHashesBetween(from, to int64) ([][]byte, int64, int64, int, error) {
	rows, err := s.db.Query(s.q(`SELECT seq, ts, prev_hash, hash FROM compliance_logs
		WHERE ts >= ? AND ts < ? ORDER BY seq ASC`), from, to)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer rows.Close()
	var out [][]byte
	var first, last int64
	n := 0
	for rows.Next() {
		var e ComplianceLogEntry
		if err := rows.Scan(&e.Seq, &e.Ts, &e.PrevHash, &e.Hash); err != nil {
			return nil, 0, 0, 0, err
		}
		b, _ := hex.DecodeString(e.Hash)
		out = append(out, b)
		if n == 0 {
			first = e.Seq
		}
		last = e.Seq
		n++
	}
	return out, first, last, n, rows.Err()
}

// ComplianceLogsBetween, delil paketi için tüm kayıtları (hash dahil) dondurur.
func (s *sqlStore) ComplianceLogsBetween(from, to int64) ([]ComplianceLog, error) {
	rows, err := s.db.Query(s.q(`SELECT seq, ts, source_type, source_name, src_ip, src_mac, user_id,
		category, message, prev_hash, hash FROM compliance_logs
		WHERE ts >= ? AND ts < ? ORDER BY seq ASC`), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComplianceLog{}
	for rows.Next() {
		var e ComplianceLog
		if err := rows.Scan(&e.Seq, &e.Ts, &e.SourceType, &e.SourceName, &e.SrcIP, &e.SrcMAC,
			&e.UserID, &e.Category, &e.Message, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ComplianceStats, panel durumu için sayılar.
func (s *sqlStore) ComplianceStats() (total int64, lastTs int64, err error) {
	err = s.db.QueryRow(s.q(`SELECT COUNT(*), COALESCE(MAX(ts), 0) FROM compliance_logs`)).Scan(&total, &lastTs)
	return total, lastTs, err
}

// PruneComplianceLogs, uyum retention penceresinin dışını siler (normal
// Prune'dan ayrı — 5651 saklama süresi bağımsızdır).
func (s *sqlStore) PruneComplianceLogs(retentionDays int) error {
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	_, err := s.db.Exec(s.q(`DELETE FROM compliance_logs WHERE ts < ?`), cutoff)
	return err
}

// --- checkpoint'ler ---

// LogCheckpoint, saatlik/günlük bütünlük kökü (+ günlükte TSA/imza).
type LogCheckpoint struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"` // hourly | daily
	BucketStart int64  `json:"bucket_start"`
	BucketEnd   int64  `json:"bucket_end"`
	RecordCount int    `json:"record_count"`
	PrevRoot    string `json:"prev_root"`
	Root        string `json:"root"`
	TSAStatus   string `json:"tsa_status"` // "" | none | ok | error:<mesaj>
	TSATime     int64  `json:"tsa_time"`
	TSAToken    []byte `json:"-"` // ham RFC 3161 yanıtı (DER)
	Signature   string `json:"signature"`
	SignedAt    int64  `json:"signed_at"`
}

// SaveLogCheckpoint, checkpoint kaydını yazar (id döner).
func (s *sqlStore) SaveLogCheckpoint(cp LogCheckpoint) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO log_checkpoints
		(kind, bucket_start, bucket_end, record_count, prev_root, root, tsa_status, tsa_time, tsa_token, signature, signed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		cp.Kind, cp.BucketStart, cp.BucketEnd, cp.RecordCount, cp.PrevRoot, cp.Root,
		cp.TSAStatus, cp.TSATime, cp.TSAToken, cp.Signature, cp.SignedAt).Scan(&id)
	return id, err
}

// CheckpointExists, idempotent mühürleme için varlık kontrolü.
func (s *sqlStore) CheckpointExists(kind string, bucketStart int64) (bool, error) {
	var one int
	err := s.db.QueryRow(s.q(`SELECT 1 FROM log_checkpoints WHERE kind = ? AND bucket_start = ?`),
		kind, bucketStart).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// LatestLogCheckpoint, türdeki en son checkpoint'i dondurur (yoksa nil).
func (s *sqlStore) LatestLogCheckpoint(kind string) (*LogCheckpoint, error) {
	row := s.db.QueryRow(s.q(`SELECT id, kind, bucket_start, bucket_end, record_count, prev_root, root,
		tsa_status, tsa_time, tsa_token, signature, signed_at
		FROM log_checkpoints WHERE kind = ? ORDER BY bucket_start DESC LIMIT 1`), kind)
	var cp LogCheckpoint
	var token []byte
	err := row.Scan(&cp.ID, &cp.Kind, &cp.BucketStart, &cp.BucketEnd, &cp.RecordCount,
		&cp.PrevRoot, &cp.Root, &cp.TSAStatus, &cp.TSATime, &token, &cp.Signature, &cp.SignedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cp.TSAToken = token
	return &cp, nil
}

// LogCheckpointsBetween, delil paketi için aralıktaki checkpoint'ler.
func (s *sqlStore) LogCheckpointsBetween(from, to int64) ([]LogCheckpoint, error) {
	rows, err := s.db.Query(s.q(`SELECT id, kind, bucket_start, bucket_end, record_count, prev_root, root,
		tsa_status, tsa_time, tsa_token, signature, signed_at
		FROM log_checkpoints WHERE bucket_start >= ? AND bucket_start < ? ORDER BY bucket_start, kind`),
		from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogCheckpoint{}
	for rows.Next() {
		var cp LogCheckpoint
		var token []byte
		if err := rows.Scan(&cp.ID, &cp.Kind, &cp.BucketStart, &cp.BucketEnd, &cp.RecordCount,
			&cp.PrevRoot, &cp.Root, &cp.TSAStatus, &cp.TSATime, &token, &cp.Signature, &cp.SignedAt); err != nil {
			return nil, err
		}
		cp.TSAToken = token
		out = append(out, cp)
	}
	return out, rows.Err()
}

// --- inceleme tutanakları (ISO A.8.15 / A.8.2) ---

// ComplianceReview, log inceleme veya erişim gözden geçirme tutanağı.
// Tutanak yazımı aynı zamanda Faz 5 audit zincirine de işlenir (server).
type ComplianceReview struct {
	ID       int64  `json:"id"`
	Ts       int64  `json:"ts"`
	Username string `json:"username"`
	Kind     string `json:"kind"`   // log | access
	Period   string `json:"period"` // ör. "2026-08-01..2026-08-31"
	Notes    string `json:"notes"`
	Finding  string `json:"finding"` // bulgu yoksa boş
}

func (s *sqlStore) SaveComplianceReview(r ComplianceReview) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO compliance_reviews (ts, username, kind, period, notes, finding)
		VALUES (?,?,?,?,?,?) RETURNING id`),
		r.Ts, r.Username, r.Kind, r.Period, r.Notes, r.Finding).Scan(&id)
	return id, err
}

func (s *sqlStore) RecentComplianceReviews(limit int) ([]ComplianceReview, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.Query(s.q(`SELECT id, ts, username, kind, period, notes, finding
		FROM compliance_reviews ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComplianceReview{}
	for rows.Next() {
		var r ComplianceReview
		if err := rows.Scan(&r.ID, &r.Ts, &r.Username, &r.Kind, &r.Period, &r.Notes, &r.Finding); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
