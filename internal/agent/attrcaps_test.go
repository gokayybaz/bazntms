package agent

import "testing"

func TestParseKernelVersion(t *testing.T) {
	cases := []struct {
		in       string
		maj, min int
		ok       bool
	}{
		{"6.8.0-51-generic", 6, 8, true},
		{"5.8.0", 5, 8, true},
		{"5.10.0-27-amd64", 5, 10, true},
		{"4.19", 4, 19, true},
		{"6.1.0-rc4+", 6, 1, true},
		{"  5.15.133  ", 5, 15, true},
		{"5", 0, 0, false},
		{"", 0, 0, false},
		{"garip", 0, 0, false},
		{"x.y.z", 0, 0, false},
	}
	for _, c := range cases {
		maj, min, ok := parseKernelVersion(c.in)
		if ok != c.ok || (ok && (maj != c.maj || min != c.min)) {
			t.Errorf("parseKernelVersion(%q) = %d,%d,%v; beklenen %d,%d,%v",
				c.in, maj, min, ok, c.maj, c.min, c.ok)
		}
	}
}

func TestKernelAtLeast(t *testing.T) {
	cases := []struct {
		maj, min, wMaj, wMin int
		want                 bool
	}{
		{5, 8, 5, 8, true},
		{5, 7, 5, 8, false},
		{5, 15, 5, 8, true},
		{6, 0, 5, 8, true},
		{4, 19, 5, 8, false},
		{6, 1, 6, 1, true},
	}
	for _, c := range cases {
		if got := kernelAtLeast(c.maj, c.min, c.wMaj, c.wMin); got != c.want {
			t.Errorf("kernelAtLeast(%d.%d ≥ %d.%d) = %v; beklenen %v",
				c.maj, c.min, c.wMaj, c.wMin, got, c.want)
		}
	}
}

func TestParseCapEff(t *testing.T) {
	status := "Name:\tbazntms-agent\nUid:\t0\t0\t0\t0\nCapEff:\t000001ffffffffff\nSeccomp:\t0\n"
	mask, ok := parseCapEff(status)
	if !ok || mask != 0x000001ffffffffff {
		t.Fatalf("parseCapEff = %x, %v", mask, ok)
	}
	// CAP_SYS_ADMIN (21) ve CAP_BPF (39) bu maskede var
	if !capBit(mask, 21) || !capBit(mask, 39) {
		t.Fatalf("beklenen bitler yok: %x", mask)
	}

	if _, ok := parseCapEff("Name:\tsh\nCapEff:\t0000000000000000\n"); !ok {
		t.Fatal("sıfır maske geçerli olmalı")
	}
	if m, _ := parseCapEff("CapEff:\t0000000000000000\n"); capBit(m, 21) {
		t.Fatal("sıfır maskede bit olmamalı")
	}
	if _, ok := parseCapEff("Name:\tsh\nSeccomp:\t0\n"); ok {
		t.Fatal("CapEff satırı yoksa ok=false olmalı")
	}
	if _, ok := parseCapEff("CapEff:\tnothex\n"); ok {
		t.Fatal("geçersiz hex → ok=false")
	}
}
