package store

import (
	"testing"
	"time"
)

// TestEnrollTokenCRUD, olusturma/hash ile bulma/listeleme/iptal temel
// akisini dogrular — api_tokens icin zaten var olan desenin (bkz.
// users_test.go) enroll_tokens karsiligi.
func TestEnrollTokenCRUD(t *testing.T) {
	st := openTest(t)

	id, err := st.CreateEnrollToken(EnrollToken{Name: "windows-filosu", TokenHash: TokenHash("gizli-1"), Site: "ofis-a"})
	if err != nil {
		t.Fatalf("olusturma: %v", err)
	}
	if id == 0 {
		t.Fatal("gecerli bir id beklenirdi")
	}

	tok, err := st.EnrollTokenByHash(TokenHash("gizli-1"))
	if err != nil {
		t.Fatalf("hash ile bulma: %v", err)
	}
	if tok.Name != "windows-filosu" || tok.Site != "ofis-a" || tok.Revoked {
		t.Fatalf("beklenmedik kayit: %+v", tok)
	}

	list, err := st.ListEnrollTokens()
	if err != nil || len(list) != 1 {
		t.Fatalf("liste: %v %+v", err, list)
	}

	if err := st.TouchEnrollToken(id); err != nil {
		t.Fatalf("touch: %v", err)
	}
	tok, _ = st.EnrollTokenByHash(TokenHash("gizli-1"))
	if tok.LastUsed == 0 {
		t.Fatal("touch sonrasi last_used guncellenmeliydi")
	}

	if err := st.RevokeEnrollToken(id); err != nil {
		t.Fatalf("iptal: %v", err)
	}
	tok, _ = st.EnrollTokenByHash(TokenHash("gizli-1"))
	if !tok.Revoked {
		t.Fatal("iptal sonrasi Revoked=true olmali")
	}
}

// TestEnrollTokenExpiry, ExpiresAt gecmiste olan bir token'in DB
// katmaninda hala BULUNABILDIGINI (silinmedigini — sona erme kontrolu
// cagiranin/handler'in sorumlulugunda, bkz. server.validEnrollToken)
// dogrular; bu bilgiyi dogru tasidigini garanti eder.
func TestEnrollTokenExpiry(t *testing.T) {
	st := openTest(t)
	past := time.Now().Add(-time.Hour).Unix()

	if _, err := st.CreateEnrollToken(EnrollToken{Name: "suresi-gecmis", TokenHash: TokenHash("gizli-2"), ExpiresAt: past}); err != nil {
		t.Fatalf("olusturma: %v", err)
	}
	tok, err := st.EnrollTokenByHash(TokenHash("gizli-2"))
	if err != nil {
		t.Fatalf("bulma: %v", err)
	}
	if tok.ExpiresAt != past {
		t.Fatalf("expires_at beklenen %d, gelen %d", past, tok.ExpiresAt)
	}
}
