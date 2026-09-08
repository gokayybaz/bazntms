package store

import "time"

// EnrollToken, hub'in -enroll-token bayragindaki TEK statik sirrina ek
// olarak, DB'de saklanan, isimli ve opsiyonel son kullanma tarihli
// enrollment token'lardir (Faz 10 — plan P2: "sizarsa hub yeniden
// baslatilmadan iptal edilemiyor" sorununu cozer). Site alani su an
// yalniz bilgi amaclidir (agent'in kendi bildirdigi site ile
// karsilastirilip zorlanmiyor) — gelecekte site-scope zorlamasi icin
// genisletilebilir.
//
// Faz 25-D sertlestirme: MaxUses (0 = sinirsiz), UsedCount, AllowedCIDRs
// (virgullu liste; bos = her IP), CreatedBy, RevokedAt.
type EnrollToken struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	TokenHash    string `json:"-"`
	Site         string `json:"site"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"` // 0 = suresiz
	LastUsed     int64  `json:"last_used"`
	Revoked      bool   `json:"revoked"`
	MaxUses      int    `json:"max_uses"`      // 0 = sinirsiz
	UsedCount    int    `json:"used_count"`    // yapilan enrollment sayisi
	AllowedCIDRs string `json:"allowed_cidrs"` // virgullu CIDR; bos = her IP
	CreatedBy    string `json:"created_by"`
	RevokedAt    int64  `json:"revoked_at"`
}

const enrollTokenCols = `id, name, token_hash, site, created_at, expires_at, last_used, revoked,
	max_uses, used_count, allowed_cidrs, created_by, revoked_at`

func scanEnrollToken(sc interface{ Scan(...any) error }) (EnrollToken, error) {
	var t EnrollToken
	err := sc.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Site, &t.CreatedAt, &t.ExpiresAt, &t.LastUsed, &t.Revoked,
		&t.MaxUses, &t.UsedCount, &t.AllowedCIDRs, &t.CreatedBy, &t.RevokedAt)
	return t, err
}

func (s *sqlStore) CreateEnrollToken(t EnrollToken) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO enroll_tokens
		(name, token_hash, site, created_at, expires_at, max_uses, allowed_cidrs, created_by)
		VALUES (?,?,?,?,?,?,?,?) RETURNING id`),
		t.Name, t.TokenHash, t.Site, time.Now().Unix(), t.ExpiresAt, t.MaxUses, t.AllowedCIDRs, t.CreatedBy).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) EnrollTokenByHash(hash string) (*EnrollToken, error) {
	row := s.db.QueryRow(s.q(`SELECT `+enrollTokenCols+` FROM enroll_tokens WHERE token_hash = ?`), hash)
	t, err := scanEnrollToken(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *sqlStore) ListEnrollTokens() ([]EnrollToken, error) {
	rows, err := s.db.Query(s.q(`SELECT ` + enrollTokenCols + ` FROM enroll_tokens ORDER BY id`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnrollToken{}
	for rows.Next() {
		t, err := scanEnrollToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *sqlStore) RevokeEnrollToken(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE enroll_tokens SET revoked = 1, revoked_at = ? WHERE id = ?`),
		time.Now().Unix(), id)
	return err
}

// ConsumeEnrollToken, bir enrollment'i atomik olarak sayar: used_count'u
// artirir ve last_used'i gunceller — ancak token iptal edilmemisse VE
// (max_uses = 0 VEYA used_count < max_uses) ise. RowsAffected 0 ise kullanim
// hakki dolmus / iptal edilmis demektir (ok = false). Es zamanli iki agent
// son slotu paylasamaz (tek UPDATE atomik).
func (s *sqlStore) ConsumeEnrollToken(id int64) (bool, error) {
	res, err := s.db.Exec(s.q(`UPDATE enroll_tokens
		SET used_count = used_count + 1, last_used = ?
		WHERE id = ? AND revoked = 0 AND (max_uses = 0 OR used_count < max_uses)`),
		time.Now().Unix(), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
