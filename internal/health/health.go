// Package health, yorumlanabilir bir ağ sağlık skoru (0-100) üretir (Faz
// 25-A). **Deterministik ağırlıklı** — opak AI skoru yok; her kesinti
// (deduction) açıklanabilir ve tekrar üretilebilir.
package health

import "strconv"

// Inputs, skoru besleyen ham sinyaller (server bunları store'dan toplar).
type Inputs struct {
	AgentsTotal     int    `json:"agents_total"`
	AgentsOnline    int    `json:"agents_online"`
	StaleAgents     int    `json:"stale_agents"` // kayıtlı ama uzun süredir görülmeyen
	DevicesTotal    int    `json:"devices_total"`
	DevicesOnline   int    `json:"devices_online"` // son poll başarılı + taze
	IfaceErrors     uint64 `json:"iface_errors"`   // filo 24s toplam
	IfaceDiscards   uint64 `json:"iface_discards"`
	CritAlertsOpen  int    `json:"crit_alerts_open"`
	OpenIncidents   int    `json:"open_incidents"`
	MaxIncidentRisk int    `json:"max_incident_risk"` // 0-100
}

// Deduction, skordan düşülen tek bir kalem.
type Deduction struct {
	Reason string `json:"reason"`
	Points int    `json:"points"`
}

// Score, hesaplanmış sağlık skoru + gerekçesi.
type Score struct {
	Score      int         `json:"score"`
	Deductions []Deduction `json:"deductions"`
	Inputs     Inputs      `json:"inputs"`
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Compute, 100'den başlar ve her sorun için belgeli bir ceza düşer. Ağırlıklar
// sabit (docs/ANALYTICS.md § Sağlık skoru).
func Compute(in Inputs) Score {
	s := Score{Score: 100, Inputs: in}
	add := func(reason string, pts int) {
		if pts <= 0 {
			return
		}
		s.Deductions = append(s.Deductions, Deduction{Reason: reason, Points: pts})
		s.Score -= pts
	}

	// 1) agent erişilebilirliği — çevrimdışı oranı × 25 (maks 25)
	if in.AgentsTotal > 0 {
		off := in.AgentsTotal - in.AgentsOnline
		if off > 0 {
			pts := clamp((off*25+in.AgentsTotal-1)/in.AgentsTotal, 0, 25)
			add(plural(off, "agent")+" çevrimdışı", pts)
		}
	}
	// 2) uzun süredir görülmeyen agent — 3 puan/adet, maks 10
	if in.StaleAgents > 0 {
		add(plural(in.StaleAgents, "agent")+" bayat telemetri", clamp(in.StaleAgents*3, 0, 10))
	}
	// 3) cihaz poll erişilebilirliği — çevrimdışı oranı × 15 (maks 15)
	if in.DevicesTotal > 0 {
		off := in.DevicesTotal - in.DevicesOnline
		if off > 0 {
			pts := clamp((off*15+in.DevicesTotal-1)/in.DevicesTotal, 0, 15)
			add(plural(off, "cihaz")+" yoklanamıyor", pts)
		}
	}
	// 4) açık kritik uyarı — 5 puan/adet, maks 20
	if in.CritAlertsOpen > 0 {
		add(plural(in.CritAlertsOpen, "açık kritik uyarı"), clamp(in.CritAlertsOpen*5, 0, 20))
	}
	// 5) açık olay (incident) — risk ağırlıklı, maks 25
	if in.OpenIncidents > 0 {
		per := 3
		if in.MaxIncidentRisk >= 70 {
			per = 8
		} else if in.MaxIncidentRisk >= 40 {
			per = 5
		}
		add(plural(in.OpenIncidents, "açık olay"), clamp(in.OpenIncidents*per, 0, 25))
	}
	// 6) arayüz hata + iskarta — hacme göre kademeli, maks 10
	if bad := in.IfaceErrors + in.IfaceDiscards; bad > 0 {
		pts := 2
		switch {
		case bad >= 100000:
			pts = 10
		case bad >= 10000:
			pts = 7
		case bad >= 1000:
			pts = 4
		}
		add("arayüz hata/iskarta yüksek (24s)", pts)
	}

	s.Score = clamp(s.Score, 0, 100)
	if s.Deductions == nil {
		s.Deductions = []Deduction{}
	}
	return s
}

func plural(n int, noun string) string {
	// Türkçe: sayı + isim (çoğul eki gerekmez). "3 agent", "1 açık olay".
	return strconv.Itoa(n) + " " + noun
}
