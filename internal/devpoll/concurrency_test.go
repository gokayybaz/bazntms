package devpoll

import (
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/driver"
	"github.com/gokayybaz/bazntms/internal/store"
)

// TestPollAllConcurrencyBounded, tek poll döngüsünün eşzamanlı yoklama
// sayısını SetConcurrency tavanının üstüne çıkarmadığını doğrular (S21.10 —
// 1000 cihazda sınırsız goroutine yerine bounded havuz).
func TestPollAllConcurrencyBounded(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "dp.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	const devices = 120
	const limit = 15
	for i := 0; i < devices; i++ {
		if _, err := st.AddDevice(store.Device{
			Name: "mock", Host: "10.0.0.1", Kind: "switch", Vendor: "mock",
			PollSeconds: 60, Enabled: true,
		}); err != nil {
			t.Fatalf("cihaz ekle: %v", err)
		}
	}

	driver.ResetMockPeak()
	p := New(st, nil)
	p.SetConcurrency(limit)
	p.pollAll()

	peak := driver.MockPeakInflight()
	if peak > limit {
		t.Fatalf("eşzamanlı poll tavanı aşıldı: peak=%d, sınır=%d", peak, limit)
	}
	if peak < 3 {
		t.Fatalf("hiç paralellik yok (peak=%d) — test kurulumu bozuk olabilir", peak)
	}

	// çoğu cihaz yoklandı (birkaçı SQLite yazıcı kilidine takılabilir — dev modu;
	// asıl kanıt eşzamanlılık sınırı, hepsinin yazılması değil)
	list, _ := st.ListDevices("")
	polled := 0
	for _, d := range list {
		if d.LastPoll > 0 {
			polled++
		}
	}
	if polled < devices*9/10 {
		t.Fatalf("yoklanan cihaz sayısı = %d/%d (çok düşük)", polled, devices)
	}
}

func TestSetConcurrencyDefault(t *testing.T) {
	p := New(nil, nil)
	if p.concurrency != defaultConcurrency {
		t.Fatalf("varsayılan eşzamanlılık = %d, beklenen %d", p.concurrency, defaultConcurrency)
	}
	p.SetConcurrency(0)
	if p.concurrency != defaultConcurrency {
		t.Fatalf("0 → varsayılan olmalı, %d geldi", p.concurrency)
	}
	p.SetConcurrency(250)
	if p.concurrency != 250 {
		t.Fatalf("250 ayarlanmalı, %d geldi", p.concurrency)
	}
}
