package store

// Tek-sahipli roller için DB tabanlı liderlik (C1 / Faz 15 S15.5–S15.6).
//
// Uyarı motoru ve SNMP poller çoklu controller replikasında YALNIZCA BİRİNDE
// çalışmalı (aksi halde alarmlar çift ateşlenir, cihazlar iki kez pollanır).
// PostgreSQL session-scoped advisory lock: kilidi tutan replika lider; süreç
// ölünce bağlantı düşer, PG kilidi otomatik bırakır, başka replika devralır.
//
// SQLite modunda (tek süreç) her zaman lider — no-op.

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

// Advisory lock anahtarları (rastgele sabitler; migrateLockKey ailesinden).
const (
	LeaderKeyAlerts = 8823101
	LeaderKeyPoller = 8823102
)

// leaderLock, liderliği tutan özel bağlantı. pg modunda conn kapanınca PG kilidi
// bırakır; sqlite modunda anlamsız (conn nil).
type leaderLock struct {
	conn *sql.Conn
	key  int64
}

func (l *leaderLock) release() {
	if l.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = l.conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", l.key)
	_ = l.conn.Close()
}

func (l *leaderLock) healthy() bool {
	if l.conn == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return l.conn.PingContext(ctx) == nil
}

// tryLeadership, key için advisory lock'u dener. Alınırsa non-nil leaderLock
// döner (release edilmeli). SQLite modunda daima alınır (conn nil).
func (s *sqlStore) tryLeadership(key int64) (*leaderLock, error) {
	if !s.pg {
		return &leaderLock{key: key}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var ok bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !ok {
		_ = conn.Close()
		return nil, nil
	}
	return &leaderLock{conn: conn, key: key}, nil
}

// Leader, bir rol için liderliği arka planda korur. IsLeader() rol döngüsünden
// her turda kontrol edilir.
type Leader struct {
	st   *sqlStore
	key  int64
	name string

	mu   sync.RWMutex
	lock *leaderLock
}

// Leader, verilen anahtar için bir liderlik koordinatörü döndürür.
func (s *sqlStore) Leader(key int64, name string) *Leader {
	return &Leader{st: s, key: key, name: name}
}

// IsLeader, bu replika şu an lider mi.
func (l *Leader) IsLeader() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lock != nil
}

func (l *Leader) set(lk *leaderLock) {
	l.mu.Lock()
	l.lock = lk
	l.mu.Unlock()
}

func (l *Leader) drop() {
	l.mu.Lock()
	lk := l.lock
	l.lock = nil
	l.mu.Unlock()
	if lk != nil {
		lk.release()
	}
}

// Run, liderliği ctx bitene dek korur: lider değilse ~10 sn'de bir dener, lider
// ise bağlantının canlılığını kontrol eder (düşmüşse bırakır, tekrar yarışır).
func (l *Leader) Run(ctx context.Context) {
	defer l.drop()
	// SQLite: tek süreç, hemen ve kalıcı lider.
	if !l.st.pg {
		lk, _ := l.st.tryLeadership(l.key)
		l.set(lk)
		<-ctx.Done()
		return
	}
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	l.tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.tick()
		}
	}
}

func (l *Leader) tick() {
	if l.IsLeader() {
		l.mu.RLock()
		ok := l.lock.healthy()
		l.mu.RUnlock()
		if !ok {
			slog.Warn("liderlik bağlantısı düştü — bırakılıyor", "rol", l.name)
			l.drop()
		}
		return
	}
	lk, err := l.st.tryLeadership(l.key)
	if err != nil {
		slog.Warn("liderlik denemesi hatası", "rol", l.name, "err", err)
		return
	}
	if lk != nil {
		l.set(lk)
		slog.Info("liderlik alındı", "rol", l.name)
	}
}
