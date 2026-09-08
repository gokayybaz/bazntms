package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
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

// CountAdmins, etkin (enabled) admin rollu kullanici sayisini dondurur.
// "Son admin" korumasi (kullanici silme/rol dusurme/pasiflestirme) bunu
// kullanir.
func (s *sqlStore) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(s.q(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND enabled = 1`)).Scan(&n)
	return n, err
}

// AdminUserExists, etkin (enabled) en az bir admin rollu kullanici olup
// olmadigini dondurur. Legacy tek-sifre girisi bu durumda devre disi
// birakilir (B6) — etkin admin yoksa legacy sifre "break-glass" olarak
// calismaya devam eder.
func (s *sqlStore) AdminUserExists() (bool, error) {
	n, err := s.CountAdmins()
	return n > 0, err
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
// v2 (Faz 25-C): actor_type/result/request_id/user_agent/before_json/after_json
// alanlarindan en az biri doluysa hash'e ek bir segment eklenir (asagi bkz.).
// Zincir kopmussa (silme/değistirme) VerifyAuditChain bulur. Kayitlar
// UPDATE/DELETE icin API tarafindan hicbir yol yoktur (append-only).

type AuditEvent struct {
	ID       int64  `json:"id"`
	Ts       int64  `json:"ts"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Site     string `json:"site"`   // aktörün sahası (S14.B2); hash zincirine DAHİL DEĞİL
	Action   string `json:"action"` // login|logout|user.create|device.add|capture.start|...
	Target   string `json:"target"` // etkilenecek nesne (agent:3, user:bob)
	Detail   string `json:"detail"` // kisa insan-okur aciklama
	IP       string `json:"ip"`
	PrevHash string `json:"prev_hash"`
	Hash     string `json:"hash"`

	// v2 alanlari (Faz 25-C). Eski kayitlarda hepsi "" → hash zincirinde yok.
	ActorType  string `json:"actor_type,omitempty"`  // user | legacy | token | oidc | system
	RequestID  string `json:"request_id,omitempty"`  // X-Request-Id (log korelasyonu)
	UserAgent  string `json:"user_agent,omitempty"`  // istemci imzasi (256'ya kirpili)
	Result     string `json:"result,omitempty"`      // ok | error | denied
	BeforeJSON string `json:"before_json,omitempty"` // islem oncesi durum (maskeli)
	AfterJSON  string `json:"after_json,omitempty"`  // islem sonrasi durum (maskeli)
}

// auditV2 raporlar: v2 alanlarindan en az biri dolu mu? Doluysa auditHash
// bu alanlari da zincire katar; degilse (tum eski kayitlar) atlar → mevcut
// zincir birebir dogrulanmaya devam eder.
func (e AuditEvent) auditV2() bool {
	return e.ActorType != "" || e.Result != "" || e.RequestID != "" ||
		e.UserAgent != "" || e.BeforeJSON != "" || e.AfterJSON != ""
}

func auditHash(prev string, e AuditEvent) string {
	h := sha256.New()
	h.Write([]byte(prev))
	fmt.Fprintf(h, "|%d|%s|%s|%s|%s|%s|%s",
		e.Ts, e.Username, e.Role, e.Action, e.Target, e.Detail, e.IP)
	if e.auditV2() {
		fmt.Fprintf(h, "|%s|%s|%s|%s|%s|%s",
			e.ActorType, e.Result, e.RequestID, e.UserAgent, e.BeforeJSON, e.AfterJSON)
	}
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
	if len(e.UserAgent) > 256 {
		e.UserAgent = e.UserAgent[:256]
	}
	e.Hash = auditHash(prev, e)

	var id int64
	err = s.db.QueryRow(s.q(`INSERT INTO audit_events
		(ts, username, role, site, action, target, detail, ip, prev_hash, hash,
		 actor_type, request_id, user_agent, result, before_json, after_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		e.Ts, e.Username, e.Role, e.Site, e.Action, e.Target, e.Detail, e.IP, e.PrevHash, e.Hash,
		e.ActorType, e.RequestID, e.UserAgent, e.Result, e.BeforeJSON, e.AfterJSON).Scan(&id)
	return id, err
}

// AuditFilter, denetim kaydı sorgusu için isteğe bağlı süzgeçler. Boş alan
// = süzme yok. Site RBAC kapsamı (S14.B2) çağıran tarafından set edilir.
type AuditFilter struct {
	Site     string // aktör sahası (site-admin kapsamı) — "" = hepsi
	Actor    string // username LIKE
	Action   string // action tam eşleşme veya "prefix.*"
	Resource string // target LIKE
	IP       string // ip tam eşleşme
	Result   string // ok | error | denied
	Since    int64  // ts >= (0 = sınır yok)
	Until    int64  // ts <= (0 = sınır yok)
	Limit    int
}

// RecentAuditEvents, son denetim olaylarini dondurur. site "" ise hepsi,
// dolu ise yalnizca o sahanin (aktör-site) olaylari (S14.B2 — site-admin).
func (s *sqlStore) RecentAuditEvents(limit int, site string) ([]AuditEvent, error) {
	return s.QueryAuditEvents(AuditFilter{Site: site, Limit: limit})
}

// QueryAuditEvents, süzgeçli denetim kaydı sorgusu (Faz 25-C). Parametreli;
// action "x.*" verilirse "x." önekiyle eşleşir.
func (s *sqlStore) QueryAuditEvents(f AuditFilter) ([]AuditEvent, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	q := `SELECT id, ts, username, role, site, action, target, detail, ip, prev_hash, hash,
		actor_type, request_id, user_agent, result, before_json, after_json
		FROM audit_events WHERE (? = '' OR site = ?)`
	args := []any{f.Site, f.Site}
	if f.Actor != "" {
		q += ` AND username LIKE ?`
		args = append(args, "%"+f.Actor+"%")
	}
	if f.Action != "" {
		if strings.HasSuffix(f.Action, ".*") {
			q += ` AND action LIKE ?`
			args = append(args, strings.TrimSuffix(f.Action, "*")+"%")
		} else {
			q += ` AND action = ?`
			args = append(args, f.Action)
		}
	}
	if f.Resource != "" {
		q += ` AND target LIKE ?`
		args = append(args, "%"+f.Resource+"%")
	}
	if f.IP != "" {
		q += ` AND ip = ?`
		args = append(args, f.IP)
	}
	if f.Result != "" {
		q += ` AND result = ?`
		args = append(args, f.Result)
	}
	if f.Since > 0 {
		q += ` AND ts >= ?`
		args = append(args, f.Since)
	}
	if f.Until > 0 {
		q += ` AND ts <= ?`
		args = append(args, f.Until)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, f.Limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Username, &e.Role, &e.Site, &e.Action, &e.Target, &e.Detail, &e.IP, &e.PrevHash, &e.Hash,
			&e.ActorType, &e.RequestID, &e.UserAgent, &e.Result, &e.BeforeJSON, &e.AfterJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// VerifyAuditChain, zinciri bastan sona dogrular; ilk bozuk kaydin ID'sini
// dondurur (ok=false). ok=true ise bozuk kayit yoktur.
func (s *sqlStore) VerifyAuditChain() (ok bool, brokenAt int64, checked int, err error) {
	rows, err := s.db.Query(s.q(`SELECT id, ts, username, role, action, target, detail, ip, prev_hash, hash,
		actor_type, request_id, user_agent, result, before_json, after_json
		FROM audit_events ORDER BY id ASC`))
	if err != nil {
		return false, 0, 0, err
	}
	defer rows.Close()

	prev := ""
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Username, &e.Role, &e.Action, &e.Target, &e.Detail, &e.IP, &e.PrevHash, &e.Hash,
			&e.ActorType, &e.RequestID, &e.UserAgent, &e.Result, &e.BeforeJSON, &e.AfterJSON); err != nil {
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
