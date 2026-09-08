package store

import (
	"math"
	"path/filepath"
	"testing"
)

// TestAnomalyBaselineRoundTrip, SaveAnomalyBaseline + LoadAnomalyBaseline'in
// dilimleri koruduğunu ve aynı (dim, metric) için yeniden kurmanın eski
// dilimleri değiştirdiğini (birikmediğini) doğrular.
func TestAnomalyBaselineRoundTrip(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "ab.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	first := []AnomalyBaselineRow{
		{Dim: "fleet", Metric: "bps", Bucket: 9, N: 200, Mean: 1000, M2: 200 * 400},
		{Dim: "fleet", Metric: "bps", Bucket: 10, N: 180, Mean: 1200, M2: 180 * 900},
		{Dim: "local", Metric: "bps", Bucket: 9, N: 50, Mean: 500, M2: 50 * 100},
	}
	if err := st.SaveAnomalyBaseline(first); err != nil {
		t.Fatalf("save: %v", err)
	}

	fleet, err := st.LoadAnomalyBaseline("fleet", "bps")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(fleet) != 2 {
		t.Fatalf("fleet/bps 2 dilim bekleniyordu, %d", len(fleet))
	}
	if fleet[0].Bucket != 9 || fleet[1].Bucket != 10 {
		t.Fatalf("kova sırası bozuk: %+v", fleet)
	}
	if fleet[0].N != 200 || fleet[0].Mean != 1000 {
		t.Fatalf("dilim değeri bozuk: %+v", fleet[0])
	}
	// std = sqrt(m2/n) = sqrt(400) = 20
	if got := fleet[0].Std(); math.Abs(got-20) > 1e-9 {
		t.Fatalf("Std() = %v, 20 bekleniyordu", got)
	}

	// yeniden kur: fleet/bps tek dilime iner, local/bps dokunulmaz
	if err := st.SaveAnomalyBaseline([]AnomalyBaselineRow{
		{Dim: "fleet", Metric: "bps", Bucket: 11, N: 300, Mean: 1500, M2: 300 * 100},
	}); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	fleet, _ = st.LoadAnomalyBaseline("fleet", "bps")
	if len(fleet) != 1 || fleet[0].Bucket != 11 {
		t.Fatalf("yeniden kurulum eski dilimleri değiştirmedi: %+v", fleet)
	}
	local, _ := st.LoadAnomalyBaseline("local", "bps")
	if len(local) != 1 || local[0].Bucket != 9 {
		t.Fatalf("local/bps yeniden kurulumdan etkilendi: %+v", local)
	}
}

// TestAnomalyBaselineStdSmallN, n < 2 iken standart sapmanın 0 döndüğünü
// doğrular (z-skoru bölmesi bu durumda atlanmalı).
func TestAnomalyBaselineStdSmallN(t *testing.T) {
	for _, r := range []AnomalyBaselineRow{
		{N: 0, M2: 100},
		{N: 1, M2: 100},
	} {
		if got := r.Std(); got != 0 {
			t.Fatalf("n=%d için Std() = %v, 0 bekleniyordu", r.N, got)
		}
	}
}

// TestLoadAnomalyBaselineEmpty, hiç dilim yokken boş (nil değil) slice döndüğünü
// doğrular — çağıran nil kontrolü yapmak zorunda kalmasın.
func TestLoadAnomalyBaselineEmpty(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "abe.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	rows, err := st.LoadAnomalyBaseline("fleet", "bps")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("boş slice bekleniyordu, %#v", rows)
	}
	// boş girdi no-op
	if err := st.SaveAnomalyBaseline(nil); err != nil {
		t.Fatalf("nil save: %v", err)
	}
}
