package store

import (
	"testing"
	"time"
)

func TestSessionStoreRoundTrip(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	if err := st.PutSession(Session{
		TokenHash: "hash-abc", Username: "alice", Role: "site-admin", Site: "dc1", Kind: "user",
		ExpiresAt: now + 3600,
	}); err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	got, err := st.GetSession("hash-abc")
	if err != nil || got == nil {
		t.Fatalf("GetSession: %v %v", got, err)
	}
	if got.Username != "alice" || got.Role != "site-admin" || got.Site != "dc1" || got.Kind != "user" {
		t.Fatalf("oturum bozuk: %+v", got)
	}

	// upsert: aynı token_hash yeniden yazılır
	if err := st.PutSession(Session{TokenHash: "hash-abc", Username: "alice2", ExpiresAt: now + 7200}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, _ = st.GetSession("hash-abc")
	if got == nil || got.Username != "alice2" {
		t.Fatalf("upsert sonrası: %+v", got)
	}

	// süresi geçmiş → yok sayılır
	if err := st.PutSession(Session{TokenHash: "hash-old", Username: "bob", ExpiresAt: now - 10}); err != nil {
		t.Fatalf("eski oturum yaz: %v", err)
	}
	if g, _ := st.GetSession("hash-old"); g != nil {
		t.Fatalf("süresi geçmiş oturum döndü: %+v", g)
	}

	// prune: eski satır fiziksel olarak silinir
	if err := st.PruneSessions(); err != nil {
		t.Fatalf("PruneSessions: %v", err)
	}
	var n int
	sq := st.(*sqlStore)
	sq.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = 'hash-old'`).Scan(&n)
	if n != 0 {
		t.Fatalf("prune eski satırı silmedi: %d", n)
	}

	// delete = logout
	if err := st.DeleteSession("hash-abc"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if g, _ := st.GetSession("hash-abc"); g != nil {
		t.Fatalf("logout sonrası oturum hâlâ var: %+v", g)
	}
}
