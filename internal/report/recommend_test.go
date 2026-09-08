package report

import (
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestRecommendDeterministicTemplates(t *testing.T) {
	// temiz → "sorun tespit edilmedi"
	clean := recommend(&EnterpriseData{
		AgentTotal: 10, AgentOnline: 10, AgentUptime: 100,
		DeviceTotal: 2, DeviceOK: 2, DeviceHealth: 100, HealthScore: 100,
	})
	if len(clean) != 1 || !strings.Contains(clean[0], "tespit edilmedi") {
		t.Fatalf("temiz filo: %+v", clean)
	}

	// eşik aşan sinyaller → ilgili öneriler
	d := &EnterpriseData{
		AgentTotal: 20, AgentOnline: 14, AgentUptime: 70,
		DeviceTotal: 5, DeviceOK: 3, DeviceHealth: 60,
		IfaceErrors: 3000, IfaceDiscards: 5000,
		PrevGB: 10, TotalGB: 15, GrowthPct: 50,
		HealthScore:   45,
		UptimeBreach:  true,
		OpenIncidents: []store.Incident{{ID: 1, RiskScore: 82}},
		TopEndpoints: []store.EndpointDelta{
			{IP: "1.1.1.1", BytesIn: 900, BytesOut: 100},
			{IP: "2.2.2.2", BytesIn: 50, BytesOut: 50},
		},
	}
	recs := recommend(d)
	joined := strings.Join(recs, "\n")
	for _, want := range []string{
		"iskarta+hata",       // 1
		"Filo uptime",        // 2
		"poll sağlığı",       // 3
		"önceki döneme göre", // 4
		"açık olay",          // 5
		"sağlık skoru",       // 6
		"SLA hedefi",         // 7
		"tek bir uzak uca",   // 8
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("öneri %q eksik:\n%s", want, joined)
		}
	}
	// deterministik: iki çağrı aynı çıktı
	if strings.Join(recommend(d), "|") != strings.Join(recommend(d), "|") {
		t.Fatal("deterministik değil")
	}
}
