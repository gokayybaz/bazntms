package ai

import (
	"testing"
	"time"
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
	tr := NewTriager(nil, nil, "crit", 10)
	if sevRank("warn") >= sevRank(tr.minSev) {
		t.Fatal("warn, crit eşiğini geçmemeli")
	}
}

func TestNewTriagerDefaults(t *testing.T) {
	tr := NewTriager(nil, nil, "", 0)
	if tr.minSev != "crit" || tr.maxPerHr != 10 {
		t.Fatalf("varsayılanlar: minSev=%q maxPerHr=%d", tr.minSev, tr.maxPerHr)
	}
}
