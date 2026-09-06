package store

// Paylaşımlı oturum deposu (A4 / Faz 15 S15.3). -session-store=db ile hub
// replikaları aynı `sessions` tablosunu kullanır. Anahtar sha256(çerez token'ı)
// — ham token diskte tutulmaz (server katmanında hash'lenir).

import (
	"database/sql"
	"time"
)

type Session struct {
	TokenHash string
	Username  string
	Role      string
	Site      string
	Kind      string
	ExpiresAt int64 // unix saniye
}

// PutSession, oturumu yazar/günceller (upsert).
func (s *sqlStore) PutSession(sess Session) error {
	_, err := s.db.Exec(s.q(`INSERT INTO sessions (token_hash, username, role, site, kind, expires_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT (token_hash) DO UPDATE SET
			username = excluded.username, role = excluded.role, site = excluded.site,
			kind = excluded.kind, expires_at = excluded.expires_at`),
		sess.TokenHash, sess.Username, sess.Role, sess.Site, sess.Kind, sess.ExpiresAt)
	return err
}

// GetSession, süresi geçmemiş oturumu döndürür; yoksa (nil, nil).
func (s *sqlStore) GetSession(tokenHash string) (*Session, error) {
	var sess Session
	sess.TokenHash = tokenHash
	err := s.db.QueryRow(s.q(`SELECT username, role, site, kind, expires_at
		FROM sessions WHERE token_hash = ? AND expires_at > ?`), tokenHash, time.Now().Unix()).
		Scan(&sess.Username, &sess.Role, &sess.Site, &sess.Kind, &sess.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteSession, oturumu sonlandırır (logout).
func (s *sqlStore) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(s.q(`DELETE FROM sessions WHERE token_hash = ?`), tokenHash)
	return err
}

// PruneSessions, süresi geçmiş oturumları siler.
func (s *sqlStore) PruneSessions() error {
	_, err := s.db.Exec(s.q(`DELETE FROM sessions WHERE expires_at < ?`), time.Now().Unix())
	return err
}
