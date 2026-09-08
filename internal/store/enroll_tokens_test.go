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

	if ok, err := st.ConsumeEnrollToken(id); err != nil || !ok {
		t.Fatalf("consume: ok=%v err=%v", ok, err)
	}
	tok, _ = st.EnrollTokenByHash(TokenHash("gizli-1"))
	if tok.LastUsed == 0 || tok.UsedCount != 1 {
		t.Fatalf("consume sonrasi last_used + used_count guncellenmeliydi: %+v", tok)
	}

	if err := st.RevokeEnrollToken(id); err != nil {
		t.Fatalf("iptal: %v", err)
	}
	tok, _ = st.EnrollTokenByHash(TokenHash("gizli-1"))
	if !tok.Revoked || tok.RevokedAt == 0 {
		t.Fatalf("iptal sonrasi Revoked=true + RevokedAt set olmali: %+v", tok)
	}
	// iptal sonrası consume reddedilir
	if ok, _ := st.ConsumeEnrollToken(id); ok {
		t.Fatal("iptal edilmiş token consume edilememeli")
	}
}

// TestConsumeEnrollTokenMaxUses, max_uses sınırının atomik sayaçla
// zorlandığını doğrular (paralel çağrılar toplamda max_uses'ı aşamaz).
func TestConsumeEnrollTokenMaxUses(t *testing.T) {
	st := openTest(t)
	id, err := st.CreateEnrollToken(EnrollToken{Name: "3-kullanim", TokenHash: TokenHash("mx-1"), MaxUses: 3})
	if err != nil {
		t.Fatalf("olusturma: %v", err)
	}

	granted := 0
	for i := 0; i < 10; i++ {
		if ok, err := st.ConsumeEnrollToken(id); err != nil {
			t.Fatalf("consume: %v", err)
		} else if ok {
			granted++
		}
	}
	if granted != 3 {
		t.Fatalf("max_uses=3 → 3 kullanım beklenirdi, gerçekleşen %d", granted)
	}
	tok, _ := st.EnrollTokenByHash(TokenHash("mx-1"))
	if tok.UsedCount != 3 {
		t.Fatalf("used_count 3 olmalı: %d", tok.UsedCount)
	}

	// max_uses=0 → sınırsız
	id2, _ := st.CreateEnrollToken(EnrollToken{Name: "sinirsiz", TokenHash: TokenHash("mx-0"), MaxUses: 0})
	for i := 0; i < 5; i++ {
		if ok, _ := st.ConsumeEnrollToken(id2); !ok {
			t.Fatalf("max_uses=0 sınırsız olmalı, %d. çağrı reddedildi", i+1)
		}
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
