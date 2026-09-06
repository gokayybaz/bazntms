package server

// dbSessionStore, oturumları store.Store (`sessions` tablosu) üzerinden tutar —
// çoklu hub replikası aynı oturumları görür (panel HA, A4 / Faz 15 S15.3).
// srv.UseDBSessions() ile devreye girer.

import (
	"context"
	"log/slog"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type dbSessionStore struct {
	st store.Store
}

func newDBSessionStore(st store.Store) *dbSessionStore { return &dbSessionStore{st: st} }

func (d *dbSessionStore) Put(tokenHash string, ident Identity, expiresAt time.Time) error {
	return d.st.PutSession(store.Session{
		TokenHash: tokenHash,
		Username:  ident.Username,
		Role:      string(ident.Role),
		Site:      ident.Site,
		Kind:      ident.Kind,
		ExpiresAt: expiresAt.Unix(),
	})
}

func (d *dbSessionStore) Get(tokenHash string) (*Identity, bool) {
	s, err := d.st.GetSession(tokenHash)
	if err != nil {
		slog.Warn("oturum deposu okuma hatası", "err", err)
		return nil, false
	}
	if s == nil {
		return nil, false
	}
	return &Identity{Username: s.Username, Role: Role(s.Role), Site: s.Site, Kind: s.Kind}, true
}

func (d *dbSessionStore) Delete(tokenHash string) error { return d.st.DeleteSession(tokenHash) }

func (d *dbSessionStore) Prune() error { return d.st.PruneSessions() }

// runJanitor, süresi geçmiş oturumları periyodik temizler (ctx bitene dek).
func (d *dbSessionStore) runJanitor(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	_ = d.Prune() // açılışta bir kez
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := d.Prune(); err != nil {
				slog.Warn("oturum janitor hatası", "err", err)
			}
		}
	}
}
