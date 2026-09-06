//go:build !windows

package store

// Faz 15 S15.5–S15.6 (C1): DB tabanlı liderlik. Advisory lock exclusivity
// yalnızca Postgres'te anlamlı — testcontainer gerekir.

import (
	"context"
	"testing"
	"time"
)

func TestLeaderExclusivePostgres(t *testing.T) {
	dsn := pgContainerDSN(t, "postgres:16-alpine")

	stA, err := Open(dsn)
	if err != nil {
		t.Fatalf("A açılamadı: %v", err)
	}
	defer stA.Close()
	stB, err := Open(dsn)
	if err != nil {
		t.Fatalf("B açılamadı: %v", err)
	}
	defer stB.Close()

	type node struct {
		l      *Leader
		cancel context.CancelFunc
	}
	nodes := []*node{}
	for _, st := range []Store{stA, stB} {
		ctx, cancel := context.WithCancel(context.Background())
		n := &node{l: st.Leader(LeaderKeyAlerts, "alerts"), cancel: cancel}
		go n.l.Run(ctx)
		nodes = append(nodes, n)
	}
	defer func() {
		for _, n := range nodes {
			n.cancel()
		}
	}()

	leaderCount := func() int {
		c := 0
		for _, n := range nodes {
			if n.l.IsLeader() {
				c++
			}
		}
		return c
	}

	// tam olarak bir lider
	waitFor(t, 5*time.Second, func() bool { return leaderCount() == 1 })
	if leaderCount() != 1 {
		t.Fatalf("tam olarak bir lider bekleniyordu, %d var", leaderCount())
	}

	// lideri düşür → diğeri <15sn'de devralmalı
	var other *node
	for _, n := range nodes {
		if n.l.IsLeader() {
			n.cancel()
		} else {
			other = n
		}
	}
	waitFor(t, 15*time.Second, other.l.IsLeader)
	if !other.l.IsLeader() {
		t.Fatal("lider düştükten sonra diğer replika devralmalıydı")
	}
}

// TestLeaderSQLiteAlwaysLeader, tek süreç modunda Leader hemen lider olur.
func TestLeaderSQLiteAlwaysLeader(t *testing.T) {
	st := openTest(t)
	l := st.Leader(LeaderKeyPoller, "poller")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Run(ctx)
	waitFor(t, 2*time.Second, l.IsLeader)
	if !l.IsLeader() {
		t.Fatal("SQLite modunda daima lider olmalı")
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
