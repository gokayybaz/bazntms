package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestTriagerRateLimit(t *testing.T) {
	tr := &Triager{maxPerHr: 3}
	for i := 0; i < 3; i++ {
		if !tr.allow() {
			t.Fatalf("ilk %d çağrı geçmeliydi", i+1)
		}
	}
	if tr.allow() {
		t.Fatal("4. çağrı hız sınırına takılmalıydı")
	}
	// pencere kayması: eski kayıtları geriye at
	tr.mu.Lock()
	tr.window[0] = time.Now().Add(-2 * time.Hour)
	tr.mu.Unlock()
	if !tr.allow() {
		t.Fatal("pencereden düşen kayıt sonrası tekrar izin verilmeliydi")
	}
}

func TestSevRank(t *testing.T) {
	if sevRank("crit") <= sevRank("warn") || sevRank("warn") <= sevRank("info") {
		t.Fatal("severity sıralaması bozuk")
	}
	// minSev=crit → warn incident triyaja girmez
	tr := NewTriager(nil, nil, "crit", 10, nil, 0)
	if sevRank("warn") >= sevRank(tr.minSev) {
		t.Fatal("warn, crit eşiğini geçmemeli")
	}
}

func TestNewTriagerDefaults(t *testing.T) {
	tr := NewTriager(nil, nil, "", 0, nil, 0)
	if tr.minSev != "crit" || tr.maxPerHr != 10 || tr.jevMinConf != 0.55 {
		t.Fatalf("varsayılanlar: minSev=%q maxPerHr=%d jevMinConf=%v", tr.minSev, tr.maxPerHr, tr.jevMinConf)
	}
}

// fakeJev, jevDecider'ın test çift'i — sabit bir noul cevabı ya da hata
// döner (gerçek HTTP çağrısı yapmadan run()/jevWorthTriage davranışını
// sınamak için).
type fakeJev struct {
	noul float64
	err  error
}

func (f fakeJev) Decide(ctx context.Context, state any, questions map[string]JevQuestion) (map[string]JevAnswer, Usage, error) {
	if f.err != nil {
		return nil, Usage{}, f.err
	}
	n := f.noul
	return map[string]JevAnswer{"worth_triage": {Type: JevNoul, Noul: &n}}, Usage{}, nil
}

func TestJevWorthTriage(t *testing.T) {
	in := store.Incident{ID: 1, Severity: "crit"}

	// jev=nil → eski davranış (her zaman evet)
	tr := NewTriager(nil, nil, "crit", 10, nil, 0)
	if !tr.jevWorthTriage(in) {
		t.Fatal("jev=nil iken her zaman true dönmeli")
	}

	// düşük güven → atlanmalı
	tr = NewTriager(nil, nil, "crit", 10, fakeJev{noul: 0.2}, 0.55)
	if tr.jevWorthTriage(in) {
		t.Fatal("noul eşiğin altında — atlanmalıydı")
	}

	// yüksek güven → devam etmeli
	tr = NewTriager(nil, nil, "crit", 10, fakeJev{noul: 0.9}, 0.55)
	if !tr.jevWorthTriage(in) {
		t.Fatal("noul eşiğin üstünde — devam etmeliydi")
	}

	// hata → fail-open (devam etmeli)
	tr = NewTriager(nil, nil, "crit", 10, fakeJev{err: fmt.Errorf("bağlantı hatası")}, 0.55)
	if !tr.jevWorthTriage(in) {
		t.Fatal("jev hatasında fail-open olmalı (devam etmeli)")
	}
}
