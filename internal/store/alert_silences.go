package store

// Bakım pencereleri / susturma (Faz 22 S22.10). Bir eşleşme (kind / site /
// key alt-dizesi — boş alan = joker) ve zaman aralığı. Aktif pencere
// içindeyken eşleşen yeni uyarılar bildirilmez, state='silenced' kaydedilir.

import "strings"

type AlertSilence struct {
	ID        int64  `json:"id"`
	MatchKind string `json:"match_kind"` // "" = her tür
	MatchSite string `json:"match_site"` // "" = her saha
	MatchKey  string `json:"match_key"`  // "" = her anahtar; aksi alt-dize
	StartsTs  int64  `json:"starts_ts"`
	EndsTs    int64  `json:"ends_ts"`
	Reason    string `json:"reason"`
	CreatedBy string `json:"created_by"`
	CreatedTs int64  `json:"created_ts"`
}

// Active, verilen an için pencere açık mı.
func (s AlertSilence) Active(now int64) bool { return now >= s.StartsTs && now < s.EndsTs }

// Matches, bir uyarının bu susturmaya uyup uymadığı (boş alanlar joker).
func (s AlertSilence) Matches(kind, site, key string) bool {
	if s.MatchKind != "" && s.MatchKind != kind {
		return false
	}
	if s.MatchSite != "" && s.MatchSite != site {
		return false
	}
	if s.MatchKey != "" && !strings.Contains(key, s.MatchKey) {
		return false
	}
	return true
}

func (s *sqlStore) AddAlertSilence(sl AlertSilence) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO alert_silences
		(match_kind, match_site, match_key, starts_ts, ends_ts, reason, created_by, created_ts)
		VALUES (?,?,?,?,?,?,?,?) RETURNING id`),
		sl.MatchKind, sl.MatchSite, sl.MatchKey, sl.StartsTs, sl.EndsTs,
		sl.Reason, sl.CreatedBy, sl.CreatedTs).Scan(&id)
	return id, err
}

// ListAlertSilences, tüm susturmaları (bitiş zamanı azalan) döndürür.
// activeOnly ise yalnız şu an açık olanlar.
func (s *sqlStore) ListAlertSilences(activeOnly bool, now int64) ([]AlertSilence, error) {
	q := `SELECT id, match_kind, match_site, match_key, starts_ts, ends_ts, reason, created_by, created_ts
		FROM alert_silences`
	var args []any
	if activeOnly {
		q += ` WHERE starts_ts <= ? AND ends_ts > ?`
		args = append(args, now, now)
	}
	q += ` ORDER BY ends_ts DESC`
	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertSilence{}
	for rows.Next() {
		var sl AlertSilence
		if err := rows.Scan(&sl.ID, &sl.MatchKind, &sl.MatchSite, &sl.MatchKey,
			&sl.StartsTs, &sl.EndsTs, &sl.Reason, &sl.CreatedBy, &sl.CreatedTs); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

func (s *sqlStore) DeleteAlertSilence(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM alert_silences WHERE id = ?`), id)
	return err
}
