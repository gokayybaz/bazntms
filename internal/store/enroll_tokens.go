package store

import "time"

// EnrollToken, hub'in -enroll-token bayragindaki TEK statik sirrina ek
// olarak, DB'de saklanan, isimli ve opsiyonel son kullanma tarihli
// enrollment token'lardir (Faz 10 — plan P2: "sizarsa hub yeniden
// baslatilmadan iptal edilemiyor" sorununu cozer). Site alani su an
// yalniz bilgi amaclidir (agent'in kendi bildirdigi site ile
// karsilastirilip zorlanmiyor) — gelecekte site-scope zorlamasi icin
// genisletilebilir.
type EnrollToken struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"-"`
	Site      string `json:"site"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"` // 0 = suresiz
	LastUsed  int64  `json:"last_used"`
	Revoked   bool   `json:"revoked"`
}

func (s *sqlStore) CreateEnrollToken(t EnrollToken) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO enroll_tokens (name, token_hash, site, created_at, expires_at)
		VALUES (?,?,?,?,?) RETURNING id`),
		t.Name, t.TokenHash, t.Site, time.Now().Unix(), t.ExpiresAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) EnrollTokenByHash(hash string) (*EnrollToken, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, token_hash, site, created_at, expires_at, last_used, revoked
		FROM enroll_tokens WHERE token_hash = ?`), hash)
	var t EnrollToken
	err := row.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Site, &t.CreatedAt, &t.ExpiresAt, &t.LastUsed, &t.Revoked)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *sqlStore) ListEnrollTokens() ([]EnrollToken, error) {
	rows, err := s.db.Query(s.q(`SELECT id, name, token_hash, site, created_at, expires_at, last_used, revoked
		FROM enroll_tokens ORDER BY id`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnrollToken{}
	for rows.Next() {
		var t EnrollToken
		if err := rows.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Site, &t.CreatedAt, &t.ExpiresAt, &t.LastUsed, &t.Revoked); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *sqlStore) RevokeEnrollToken(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE enroll_tokens SET revoked = 1 WHERE id = ?`), id)
	return err
}

func (s *sqlStore) TouchEnrollToken(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE enroll_tokens SET last_used = ? WHERE id = ?`), time.Now().Unix(), id)
	return err
}
