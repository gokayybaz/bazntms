package telemetry

import "testing"

func TestClampTS(t *testing.T) {
	const now = 1_700_000_000
	const day = 86400
	cases := []struct {
		name   string
		ts     int64
		expect int64
	}{
		{"normal", now - 30, now - 30},
		{"tam şimdi", now, now},
		{"sıfır → now", 0, now},
		{"negatif → now", -5, now},
		{"offline replay (2g eski) korunur", now - 2*day, now - 2*day},
		{"7g sınırı korunur", now - 7*day + 1, now - 7*day + 1},
		{"8g eski → now (saat geri)", now - 8*day, now},
		{"1g ileri korunur", now + day - 1, now + day - 1},
		{"2g ileri → now (saat ileri)", now + 2*day, now},
	}
	for _, c := range cases {
		if got := ClampTS(c.ts, now); got != c.expect {
			t.Errorf("%s: ClampTS(%d) = %d, beklenen %d", c.name, c.ts, got, c.expect)
		}
	}
}
