package main

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	const iv = 30 // sn

	// başarılı → düz aralık, jitter yok
	if d := retryDelay(iv, 0); d != 30*time.Second {
		t.Fatalf("fails=0 → %v, beklenen 30s", d)
	}

	// üstel geri çekilme: ~2×, 4×, 8×, sonra sabit 8× (5 dk kapağı altında)
	for _, tc := range []struct {
		fails    int
		wantBase time.Duration
	}{
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
		{5, 240 * time.Second}, // 1<<min(5,3) = 8× = 240s (kapak)
	} {
		d := retryDelay(iv, tc.fails)
		lo := tc.wantBase - tc.wantBase/10 - time.Second
		hi := tc.wantBase + tc.wantBase/10 + time.Second
		if d < lo || d > hi {
			t.Errorf("fails=%d → %v, ~%v (±%%10 jitter) bekleniyordu", tc.fails, d, tc.wantBase)
		}
	}

	// 5 dk üst kapağı
	if d := retryDelay(600, 9); d > 5*time.Minute+30*time.Second {
		t.Fatalf("5 dk kapağı aşıldı: %v", d)
	}

	// jitter dağılıyor mu (aynı girdide farklı sonuçlar)
	seen := map[time.Duration]bool{}
	for i := 0; i < 20; i++ {
		seen[retryDelay(iv, 2)] = true
	}
	if len(seen) < 5 {
		t.Errorf("jitter dağılmıyor: %d farklı değer", len(seen))
	}
}
