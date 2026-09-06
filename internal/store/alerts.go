package store

// Uyari motorunun kalici durumu: olay gecmisi (alert_events), tekrar
// bastirma (alert_seen), tek-satir JSON yapilandirmasi (alert_config).

import (
	"database/sql"
	"time"
)

// --- uyarilar ---

type AlertEvent struct {
	ID      int64  `json:"id"`
	Ts      int64  `json:"ts"`
	Kind    string `json:"kind"` // bw | port | proc | target
	Key     string `json:"key"`
	Message string `json:"message"`
}

func (s *sqlStore) InsertAlertEvent(e AlertEvent) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO alert_events (ts, kind, key, message) VALUES (?,?,?,?) RETURNING id`),
		e.Ts, e.Kind, e.Key, e.Message).Scan(&id)
	return id, err
}

func (s *sqlStore) RecentAlertEvents(limit int) ([]AlertEvent, error) {
	rows, err := s.db.Query(s.q(`SELECT id, ts, kind, key, message FROM alert_events ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Kind, &e.Key, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []AlertEvent{}
	}
	return out, rows.Err()
}

// IsAlertSeen, kalici gorulmusluk kontrolu (yeni surec/hedef kurallari icin).
func (s *sqlStore) IsAlertSeen(kind, key string) (bool, error) {
	var one int
	err := s.db.QueryRow(s.q(`SELECT 1 FROM alert_seen WHERE kind = ? AND key = ?`), kind, key).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *sqlStore) MarkAlertSeen(kind, key string) error {
	_, err := s.db.Exec(s.q(`INSERT INTO alert_seen (kind, key, ts) VALUES (?,?,?)
		ON CONFLICT (kind, key) DO NOTHING`), kind, key, time.Now().Unix())
	return err
}

func (s *sqlStore) CountAlertSeen(kind string) (int, error) {
	var n int
	err := s.db.QueryRow(s.q(`SELECT COUNT(*) FROM alert_seen WHERE kind = ?`), kind).Scan(&n)
	return n, err
}

// LoadAlertConfig, tek satirlik JSON yapilandirmasini dondurur; yoksa "" doner.
func (s *sqlStore) LoadAlertConfig() (string, error) {
	var cfg string
	err := s.db.QueryRow(s.q(`SELECT cfg FROM alert_config WHERE id = 1`)).Scan(&cfg)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cfg, err
}

func (s *sqlStore) SaveAlertConfig(cfg string) error {
	_, err := s.db.Exec(s.q(`INSERT INTO alert_config (id, cfg) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET cfg = excluded.cfg`), cfg)
	return err
}
