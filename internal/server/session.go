package server

// Oturum deposu soyutlaması (A4 / Faz 15). AuthManager oturumları bu arayüz
// arkasından tutar; varsayılan bellek-içi gerçekleme bugünkü davranışı birebir
// korur, dbSessionStore (session_db.go) çoklu replika için Postgres kullanır.
//
// Anahtar: sha256(çerez token'ı) hex — ham token asla depoda tutulmaz.
// Karar notu: docs/decisions/0004-shared-sessions.md

import (
	"sync"
	"time"
)

// SessionStore, kimlik taşıyan oturumların kalıcılığı.
type SessionStore interface {
	// Put, tokenHash için kimliği expiresAt'e kadar geçerli olacak şekilde yazar.
	Put(tokenHash string, ident Identity, expiresAt time.Time) error
	// Get, süresi geçmemiş bir oturum varsa kimliği döndürür.
	Get(tokenHash string) (*Identity, bool)
	// Delete, oturumu sonlandırır (logout).
	Delete(tokenHash string) error
	// Prune, süresi geçmiş oturumları temizler.
	Prune() error
}

// memSessionStore, süreç-içi map (tek replika / dev modu varsayılanı).
type memSessionStore struct {
	mu       sync.Mutex
	sessions map[string]memSession
}

type memSession struct {
	ident Identity
	exp   time.Time
}

func newMemSessionStore() *memSessionStore {
	return &memSessionStore{sessions: map[string]memSession{}}
}

func (m *memSessionStore) Put(tokenHash string, ident Identity, expiresAt time.Time) error {
	m.mu.Lock()
	m.sessions[tokenHash] = memSession{ident: ident, exp: expiresAt}
	m.pruneLocked()
	m.mu.Unlock()
	return nil
}

func (m *memSessionStore) Get(tokenHash string) (*Identity, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[tokenHash]
	if !ok || time.Now().After(s.exp) {
		return nil, false
	}
	id := s.ident
	return &id, true
}

func (m *memSessionStore) Delete(tokenHash string) error {
	m.mu.Lock()
	delete(m.sessions, tokenHash)
	m.mu.Unlock()
	return nil
}

func (m *memSessionStore) Prune() error {
	m.mu.Lock()
	m.pruneLocked()
	m.mu.Unlock()
	return nil
}

func (m *memSessionStore) pruneLocked() {
	now := time.Now()
	for k, s := range m.sessions {
		if now.After(s.exp) {
			delete(m.sessions, k)
		}
	}
}
