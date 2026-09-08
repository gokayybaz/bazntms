package alert

// S22.2: mevsimsel kova eşlemesi + EWMA gün ağırlığı birim testleri.

import (
	"math"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestSeasonalBucket(t *testing.T) {
	loc := time.UTC
	mon14 := time.Date(2026, 9, 7, 14, 30, 0, 0, loc) // Pazartesi
	sat09 := time.Date(2026, 9, 12, 9, 5, 0, 0, loc)  // Cumartesi
	sun23 := time.Date(2026, 9, 13, 23, 0, 0, 0, loc) // Pazar

	cases := []struct {
		t          time.Time
		seasonalty string
		want       int
	}{
		{mon14, "hourly", 14},
		{mon14, "weekday", 14},
		{mon14, "dow", 1*24 + 14},
		{sat09, "hourly", 9},
		{sat09, "weekday", 24 + 9},
		{sat09, "dow", 6*24 + 9},
		{sun23, "weekday", 24 + 23},
		{sun23, "dow", 0*24 + 23},
		{mon14, "bilinmeyen", 14}, // → hourly
	}
	for _, c := range cases {
		if got := store.SeasonalBucket(c.t, c.seasonalty); got != c.want {
			t.Errorf("SeasonalBucket(%s, %q) = %d, beklenen %d", c.t.Weekday(), c.seasonalty, got, c.want)
		}
	}
}

func TestCombineBaselineEWMA(t *testing.T) {
	// aynı kova, iki gün grubu: bugün ort 1000 (varyanssız), 10 gün önce ort 2000.
	buckets := []store.BaselineDayBucket{
		{Bucket: 14, DayAge: 0, N: 100, Sum: 100 * 1000, SumSq: 100 * 1000 * 1000},
		{Bucket: 14, DayAge: 10, N: 100, Sum: 100 * 2000, SumSq: 100 * 2000 * 2000},
	}

	// yarı-ömür 10 gün → 10 günlük grup 0.5 ağırlık: mean = 200000 / 150 ≈ 1333.3
	ew := combineBaseline("fleet", "bps", buckets, 10)
	if len(ew) != 1 {
		t.Fatalf("tek kova bekleniyordu, %d", len(ew))
	}
	if math.Abs(ew[0].Mean-1333.333) > 0.5 {
		t.Errorf("EWMA mean = %.3f, ~1333.3 bekleniyordu", ew[0].Mean)
	}
	if ew[0].N != 200 {
		t.Errorf("N ağırlıksız ham sayı olmalı: %d", ew[0].N)
	}

	// yarı-ömür 0 → eşit ağırlık: mean = 1500
	eq := combineBaseline("fleet", "bps", buckets, 0)
	if math.Abs(eq[0].Mean-1500) > 1e-9 {
		t.Errorf("eşit ağırlık mean = %.3f, 1500 bekleniyordu", eq[0].Mean)
	}
	// iki küme arası yayılım → varyans > 0, std ≈ 500
	if s := eq[0].Std(); math.Abs(s-500) > 1 {
		t.Errorf("eşit ağırlık std = %.2f, ~500 bekleniyordu", s)
	}
}
