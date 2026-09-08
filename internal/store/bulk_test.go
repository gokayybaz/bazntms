package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestPlaceholders(t *testing.T) {
	cases := map[int]string{0: "", 1: "?", 3: "?,?,?"}
	for n, want := range cases {
		if got := placeholders(n); got != want {
			t.Errorf("placeholders(%d) = %q, beklenen %q", n, got, want)
		}
	}
}

// TestBulkInsertChunking, tek çağrıda maxRows'u aşan satır sayısının (PG'de
// chunk'lara bölünerek) eksiksiz yazıldığını doğrular — SQLite yolunda da
// hepsi yazılmalı. flows tablosu 9 kolon: pgMaxParams/9 ≈ 6666 → 15000 satır
// PG'de 3 chunk.
func TestBulkInsertChunking(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "bulk.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	const n = 15000
	rows := make([]FlowRow, n)
	base := time.Now().Add(-time.Hour).Unix()
	for i := 0; i < n; i++ {
		rows[i] = FlowRow{
			Ts: base + int64(i%60), Device: "10.0.0.1",
			Src: fmt.Sprintf("10.1.%d.%d", i/250%250, i%250), Dst: "8.8.8.8",
			SrcPort: uint16(1024 + i%60000), DstPort: 443, Proto: "tcp",
			Packets: uint64(i + 1), Octets: uint64((i + 1) * 100),
		}
	}
	if err := st.SaveFlows(rows); err != nil {
		t.Fatalf("SaveFlows(%d): %v", n, err)
	}

	got, err := st.TopFlows(time.Now().Add(-2*time.Hour), 100, "")
	if err != nil {
		t.Fatalf("TopFlows: %v", err)
	}
	// en yoğun akış son satır (octets = n*100)
	if len(got) == 0 || got[0].Octets != uint64(n*100) {
		t.Fatalf("tepe akış hatalı: %+v", got)
	}

	// toplam satır sayısı (doğrudan sorgu — dialect-bağımsız)
	var cnt int
	if err := st.(*sqlStore).db.QueryRow("SELECT COUNT(*) FROM flows").Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != n {
		t.Fatalf("flows satır sayısı = %d, beklenen %d", cnt, n)
	}
}

// TestBulkInsertEmpty, boş girdide no-op.
func TestBulkInsertEmpty(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.SaveFlows(nil); err != nil {
		t.Fatalf("SaveFlows(nil): %v", err)
	}
	if err := st.SaveIfaceSamples(1, time.Now().Unix(), nil); err != nil {
		t.Fatalf("SaveIfaceSamples(nil): %v", err)
	}
}
