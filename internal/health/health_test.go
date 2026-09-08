package health

import "testing"

func TestComputeHealthy(t *testing.T) {
	s := Compute(Inputs{AgentsTotal: 10, AgentsOnline: 10, DevicesTotal: 3, DevicesOnline: 3})
	if s.Score != 100 || len(s.Deductions) != 0 {
		t.Fatalf("temiz filo 100 olmalı: %+v", s)
	}
}

func TestComputeDeductionsExplainableAndReproducible(t *testing.T) {
	in := Inputs{
		AgentsTotal: 20, AgentsOnline: 16, // 4 çevrimdışı → ceil(4*25/20)=5
		StaleAgents:  2,                   // 6
		DevicesTotal: 4, DevicesOnline: 3, // ceil(15/4)=4
		CritAlertsOpen: 3,                      // 15
		OpenIncidents:  2, MaxIncidentRisk: 85, // 2*8=16
		IfaceErrors: 5000, IfaceDiscards: 8000, // 13000 → 7
	}
	s1 := Compute(in)
	s2 := Compute(in)

	// tekrar üretilebilir
	if s1.Score != s2.Score || len(s1.Deductions) != len(s2.Deductions) {
		t.Fatalf("tekrar üretilebilir değil: %+v vs %+v", s1, s2)
	}
	// her kesinti = pozitif puan + gerekçe; toplam skor düşüşüyle tutarlı
	total := 0
	for _, d := range s1.Deductions {
		if d.Points <= 0 || d.Reason == "" {
			t.Fatalf("kesinti açıklanamıyor: %+v", d)
		}
		total += d.Points
	}
	if s1.Score != clamp(100-total, 0, 100) {
		t.Fatalf("skor kesinti toplamıyla tutarsız: %d vs 100-%d", s1.Score, total)
	}
	// beklenen bileşenler
	want := map[string]int{
		"4 agent çevrimdışı":               5,
		"2 agent bayat telemetri":          6,
		"1 cihaz yoklanamıyor":             4,
		"3 açık kritik uyarı":              15,
		"2 açık olay":                      16,
		"arayüz hata/iskarta yüksek (24s)": 7,
	}
	got := map[string]int{}
	for _, d := range s1.Deductions {
		got[d.Reason] = d.Points
	}
	for r, p := range want {
		if got[r] != p {
			t.Errorf("kesinti %q = %d, beklenen %d (hepsi: %+v)", r, got[r], p, got)
		}
	}
	if s1.Score != 47 { // 100 - (5+6+4+15+16+7)
		t.Fatalf("skor 47 beklenirdi: %d", s1.Score)
	}
}

func TestComputeCatastrophicAndClamp(t *testing.T) {
	s := Compute(Inputs{
		AgentsTotal: 10, AgentsOnline: 0,
		DevicesTotal: 10, DevicesOnline: 0,
		CritAlertsOpen: 20, OpenIncidents: 20, MaxIncidentRisk: 100,
		IfaceErrors: 1e6,
	})
	// her kategori tavanına oturur: 25+15+20+25+10 = 95 → skor 5, [0,100]'de.
	if s.Score < 0 || s.Score > 10 {
		t.Fatalf("aşırı kötü filo çok düşük + kırpılı olmalı: %d", s.Score)
	}
	// her kategori kendi tavanını aşamaz
	for _, d := range s.Deductions {
		if d.Points > 25 {
			t.Fatalf("tek kesinti tavanı aştı: %+v", d)
		}
	}
}
