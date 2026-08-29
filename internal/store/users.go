package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// --- kullanicilar ve roller (Faz 5.1 RBAC) ---

// Rolleri: admin > netops > analyst > viewer. Yetki matrisi server
// paketinde (rbac.go) tutulur; store yalnizca kayit saklar.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // bcrypt; API'ye donmez
	Role         string `json:"role"`
	Site         string `json:"site"` // bos = tum siteler
	Enabled      bool   `json:"enabled"`
	CreatedAt    int64  `json:"created_at"`
	LastLogin    int64  `json:"last_login"`
}

func (s *sqlStore) CreateUser(u User) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO users (username, password_hash, role, site, enabled, created_at)
		VALUES (?,?,?,?,?,?) RETURNING id`),
		u.Username, u.PasswordHash, u.Role, u.Site, btoi(u.Enabled), time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) UserByName(username string) (*User, error) {
	row := s.db.QueryRow(s.q(`SELECT id, username, password_hash, role, site, enabled, created_at, last_login
		FROM users WHERE username = ?`), username)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Site, &u.Enabled, &u.CreatedAt, &u.LastLogin)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *sqlStore) UserByID(id int64) (*User, error) {
	row := s.db.QueryRow(s.q(`SELECT id, username, password_hash, role, site, enabled, created_at, last_login
		FROM users WHERE id = ?`), id)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Site, &u.Enabled, &u.CreatedAt, &u.LastLogin)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *sqlStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query(s.q(`SELECT id, username, password_hash, role, site, enabled, created_at, last_login
		FROM users ORDER BY username`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Site, &u.Enabled, &u.CreatedAt, &u.LastLogin); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUser, kullaniciyi gunceller; sifir ID'li alanlar korunur.
func (s *sqlStore) UpdateUser(u User) error {
	_, err := s.db.Exec(s.q(`UPDATE users SET username = ?, role = ?, site = ?, enabled = ? WHERE id = ?`),
		u.Username, u.Role, u.Site, btoi(u.Enabled), u.ID)
	return err
}

func (s *sqlStore) UpdateUserPassword(id int64, passwordHash string) error {
	_, err := s.db.Exec(s.q(`UPDATE users SET password_hash = ? WHERE id = ?`), passwordHash, id)
	return err
}

func (s *sqlStore) TouchUserLogin(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE users SET last_login = ? WHERE id = ?`), time.Now().Unix(), id)
	return err
}

func (s *sqlStore) DeleteUser(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM users WHERE id = ?`), id)
	return err
}

// --- entegrasyon API token'lari (Faz 5.2) ---

type APIToken struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"-"`
	Role      string `json:"role"`
	Site      string `json:"site"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used"`
	Revoked   bool   `json:"revoked"`
}

func (s *sqlStore) CreateAPIToken(t APIToken) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO api_tokens (name, token_hash, role, site, created_at)
		VALUES (?,?,?,?,?) RETURNING id`),
		t.Name, t.TokenHash, t.Role, t.Site, time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) APITokenByHash(hash string) (*APIToken, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, token_hash, role, site, created_at, last_used, revoked
		FROM api_tokens WHERE token_hash = ?`), hash)
	var t APIToken
	err := row.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Role, &t.Site, &t.CreatedAt, &t.LastUsed, &t.Revoked)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *sqlStore) ListAPITokens() ([]APIToken, error) {
	rows, err := s.db.Query(s.q(`SELECT id, name, token_hash, role, site, created_at, last_used, revoked
		FROM api_tokens ORDER BY id`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIToken{}
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Role, &t.Site, &t.CreatedAt, &t.LastUsed, &t.Revoked); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *sqlStore) RevokeAPIToken(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE api_tokens SET revoked = 1 WHERE id = ?`), id)
	return err
}

func (s *sqlStore) DeleteAPIToken(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM api_tokens WHERE id = ?`), id)
	return err
}

func (s *sqlStore) TouchAPIToken(id int64) error {
	_, err := s.db.Exec(s.q(`UPDATE api_tokens SET last_used = ? WHERE id = ?`), time.Now().Unix(), id)
	return err
}

// --- denetim kaydi (Faz 5.3) — append-only hash zinciri ---
//
// Her kayit bir onceki kaydin hash'ini prev_hash alanina gomulur:
//   hash = SHA-256(prev_hash | ts | username | role | action | target | detail | ip)
// Zincir kopmussa (silme/değistirme) VerifyAuditChain bulur. Kayitlar
// UPDATE/DELETE icin API tarafindan hicbir yol yoktur (append-only).

type AuditEvent struct {
	ID       int64  `json:"id"`
	Ts       int64  `json:"ts"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Action   string `json:"action"` // login|logout|user.create|device.add|capture.start|...
	Target   string `json:"target"` // etkilenecek nesne (agent:3, user:bob)
	Detail   string `json:"detail"` // kisa insan-okur aciklama
	IP       string `json:"ip"`
	PrevHash string `json:"prev_hash"`
	Hash     string `json:"hash"`
}

func auditHash(prev string, e AuditEvent) string {
	h := sha256.New()
	h.Write([]byte(prev))
	fmt.Fprintf(h, "|%d|%s|%s|%s|%s|%s|%s",
		e.Ts, e.Username, e.Role, e.Action, e.Target, e.Detail, e.IP)
	return hex.EncodeToString(h.Sum(nil))
}

// InsertAuditEvent, kaydi zincire ekler. Eşzamanli yazimlarda zincir
// tutarliligi icin kilitlenir.
func (s *sqlStore) InsertAuditEvent(e AuditEvent) (int64, error) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	var prev string
	err := s.db.QueryRow(s.q(`SELECT hash FROM audit_events ORDER BY id DESC LIMIT 1`)).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	e.PrevHash = prev
	e.Ts = time.Now().Unix()
	e.Hash = auditHash(prev, e)

	var id int64
	err = s.db.QueryRow(s.q(`INSERT INTO audit_events (ts, username, role, action, target, detail, ip, prev_hash, hash)
		VALUES (?,?,?,?,?,?,?,?,?) RETURNING id`),
		e.Ts, e.Username, e.Role, e.Action, e.Target, e.Detail, e.IP, e.PrevHash, e.Hash).Scan(&id)
	return id, err
}

func (s *sqlStore) RecentAuditEvents(limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.Query(s.q(`SELECT id, ts, username, role, action, target, detail, ip, prev_hash, hash
		FROM audit_events ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Username, &e.Role, &e.Action, &e.Target, &e.Detail, &e.IP, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// VerifyAuditChain, zinciri bastan sona dogrular; ilk bozuk kaydin ID'sini
// dondurur (ok=false). ok=true ise bozuk kayit yoktur.
func (s *sqlStore) VerifyAuditChain() (ok bool, brokenAt int64, checked int, err error) {
	rows, err := s.db.Query(s.q(`SELECT id, ts, username, role, action, target, detail, ip, prev_hash, hash
		FROM audit_events ORDER BY id ASC`))
	if err != nil {
		return false, 0, 0, err
	}
	defer rows.Close()

	prev := ""
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Username, &e.Role, &e.Action, &e.Target, &e.Detail, &e.IP, &e.PrevHash, &e.Hash); err != nil {
			return false, 0, checked, err
		}
		if e.PrevHash != prev || auditHash(prev, e) != e.Hash {
			return false, e.ID, checked, nil
		}
		prev = e.Hash
		checked++
	}
	return true, 0, checked, rows.Err()
}
