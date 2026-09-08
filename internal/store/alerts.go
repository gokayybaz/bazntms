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
	Ts      int64  `json:"ts"` // = FirstTs (geriye uyum: eski istemciler "ts" bekler)
	Kind    string `json:"kind"`
	Key     string `json:"key"`
	Message string `json:"message"`

	// yaşam döngüsü (Faz 22 S22.6 · 0010_alert_lifecycle)
	Severity   string `json:"severity"` // info | warn | crit
	State      string `json:"state"`    // firing | ack | resolved | silenced
	Site       string `json:"site"`
	Count      int64  `json:"count"`
	FirstTs    int64  `json:"first_ts"`
	LastTs     int64  `json:"last_ts"`
	ResolvedTs int64  `json:"resolved_ts,omitempty"`
	AckBy      string `json:"ack_by,omitempty"`
	AckTs      int64  `json:"ack_ts,omitempty"`
	Note       string `json:"note,omitempty"`
	GroupID    string `json:"group_id,omitempty"`
	ExtRef     string `json:"ext_ref,omitempty"`
}

const alertEventCols = `id, ts, kind, key, message, severity, state, site, count,
	first_ts, last_ts, resolved_ts, ack_by, ack_ts, note, group_id, ext_ref`

func scanAlertEvent(sc interface{ Scan(...any) error }) (AlertEvent, error) {
	var e AlertEvent
	err := sc.Scan(&e.ID, &e.Ts, &e.Kind, &e.Key, &e.Message, &e.Severity, &e.State,
		&e.Site, &e.Count, &e.FirstTs, &e.LastTs, &e.ResolvedTs, &e.AckBy, &e.AckTs,
		&e.Note, &e.GroupID, &e.ExtRef)
	return e, err
}

func (s *sqlStore) InsertAlertEvent(e AlertEvent) (int64, error) {
	if e.Severity == "" {
		e.Severity = "warn"
	}
	if e.State == "" {
		e.State = "firing"
	}
	if e.Count == 0 {
		e.Count = 1
	}
	if e.FirstTs == 0 {
		e.FirstTs = e.Ts
	}
	if e.LastTs == 0 {
		e.LastTs = e.Ts
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO alert_events
		(ts, kind, key, message, severity, state, site, count, first_ts, last_ts, group_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		e.Ts, e.Kind, e.Key, e.Message, e.Severity, e.State, e.Site, e.Count,
		e.FirstTs, e.LastTs, e.GroupID).Scan(&id)
	return id, err
}

// OpenAlertEventByKey, verilen (kind,key) için açık (firing|ack) en son olayı
// döndürür — S22.6 dedup: aynı koşul tekrar ateşlenirse yeni satır yerine
// mevcut olay tazelenir (BumpAlertEvent).
func (s *sqlStore) OpenAlertEventByKey(kind, key string) (*AlertEvent, error) {
	e, err := scanAlertEvent(s.db.QueryRow(s.q(`SELECT `+alertEventCols+`
		FROM alert_events WHERE kind = ? AND key = ? AND state IN ('firing','ack')
		ORDER BY id DESC LIMIT 1`), kind, key))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// BumpAlertEvent, açık bir olayın tekrar sayacını + son görülme zamanını
// artırır ve mesajını günceller (yeni satır yaratmadan).
func (s *sqlStore) BumpAlertEvent(id, ts int64, message string) error {
	_, err := s.db.Exec(s.q(`UPDATE alert_events
		SET count = count + 1, last_ts = ?, message = ? WHERE id = ?`), ts, message, id)
	return err
}

// ResolveAlertEvent, açık bir olayı çözüldü işaretler (S22.8 — otomatik veya
// operatör). Zaten resolved ise no-op.
func (s *sqlStore) ResolveAlertEvent(id, ts int64) error {
	_, err := s.db.Exec(s.q(`UPDATE alert_events
		SET state = 'resolved', resolved_ts = ? WHERE id = ? AND state IN ('firing','ack')`), ts, id)
	return err
}

// OpenAlertEventsStale, last_ts'i `before`'dan eski olan açık olayları döndürür
// (S22.8 TTL otomatik çözülme).
func (s *sqlStore) OpenAlertEventsStale(before int64) ([]AlertEvent, error) {
	return s.queryAlertEvents(`SELECT `+alertEventCols+` FROM alert_events
		WHERE state IN ('firing','ack') AND last_ts < ? ORDER BY id`, before)
}

// OpenAlertEventsByKind, verilen türün tüm açık olaylarını döndürür (S22.8
// koşul-tabanlı çözülme — anomali).
func (s *sqlStore) OpenAlertEventsByKind(kind string) ([]AlertEvent, error) {
	return s.queryAlertEvents(`SELECT `+alertEventCols+` FROM alert_events
		WHERE state IN ('firing','ack') AND kind = ? ORDER BY id`, kind)
}

// OpenAlertEventsBySiteSince, verilen sahada last_ts >= since olan açık
// olayları döndürür (S22.9 korelasyon penceresi).
func (s *sqlStore) OpenAlertEventsBySiteSince(site string, since int64) ([]AlertEvent, error) {
	return s.queryAlertEvents(`SELECT `+alertEventCols+` FROM alert_events
		WHERE state IN ('firing','ack') AND site = ? AND last_ts >= ? ORDER BY id`, site, since)
}

// SetAlertEventGroup, bir olayın korelasyon grubunu atar (S22.9).
func (s *sqlStore) SetAlertEventGroup(id int64, groupID string) error {
	_, err := s.db.Exec(s.q(`UPDATE alert_events SET group_id = ? WHERE id = ?`), groupID, id)
	return err
}

func (s *sqlStore) queryAlertEvents(query string, args ...any) ([]AlertEvent, error) {
	rows, err := s.db.Query(s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertEvent{}
	for rows.Next() {
		e, err := scanAlertEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *sqlStore) RecentAlertEvents(limit int) ([]AlertEvent, error) {
	rows, err := s.db.Query(s.q(`SELECT `+alertEventCols+` FROM alert_events ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertEvent{}
	for rows.Next() {
		e, err := scanAlertEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
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
