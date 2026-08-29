package proctraffic

import "testing"

// Non-root'ta lsof yalnizca kendi sureclerini gosterir; test, saglayicinin
// calistigini ve makul esleme dondurdugunu dogrular.
func TestDarwinSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("short mod")
	}
	p := NewProvider()
	snap := p.Snapshot()
	if len(snap) == 0 {
		t.Skip("lsof bos dondu (ortam sinirli)")
	}
	var withPID int
	for _, pi := range snap {
		if pi.PID > 0 && pi.Process != "" {
			withPID++
		}
	}
	if withPID == 0 {
		t.Fatal("hicbir baglanti surec eslesmedi")
	}
	t.Logf("esleme: %d anahtar, %d surec eslesmis", len(snap), withPID)
}
