package report

import "fmt"

// recommend, kurumsal rapor için deterministik öneriler üretir (Faz 25-B).
// **LLM YOK** — tablo-güdümlü şablonlar; her öneri bir eşiğe bağlıdır ve
// gerekçesiyle birlikte metne yazılır.
func recommend(d *EnterpriseData) []string {
	var out []string
	add := func(s string) { out = append(out, s) }

	// 1) arayüz hata / iskarta
	if bad := d.IfaceErrors + d.IfaceDiscards; bad >= 1000 {
		add(fmt.Sprintf("SNMP izlenen cihaz arayüzlerinde dönem içi %d iskarta+hata sayaç artışı — "+
			"fiziksel hat, dupleks uyuşmazlığı veya QoS drop'larını kontrol edin.", bad))
	}
	// 2) agent uptime
	if d.AgentTotal > 0 && d.AgentUptime < 90 {
		off := d.AgentTotal - d.AgentOnline
		add(fmt.Sprintf("Filo uptime %%%.1f — %d agent çevrimdışı. Servis/ağ erişimini doğrulayın; "+
			"kalıcı çevrimdışı agent'ları arşivleyin (-agent-archive-days).", d.AgentUptime, off))
	}
	// 3) cihaz sağlığı
	if d.DeviceTotal > 0 && d.DeviceHealth < 90 {
		add(fmt.Sprintf("Cihaz poll sağlığı %%%.1f — %d/%d cihaz güncel veri vermiyor. "+
			"SNMP community/sürüm, ACL ve erişilebilirliği kontrol edin.", d.DeviceHealth, d.DeviceTotal-d.DeviceOK, d.DeviceTotal))
	}
	// 4) trafik büyümesi
	if d.PrevGB > 0 && d.GrowthPct >= 25 {
		add(fmt.Sprintf("Toplam trafik önceki döneme göre %%%.0f arttı (%.1f → %.1f GB) — "+
			"uplink kapasitesi ve arayüz kullanım eşiklerini gözden geçirin.", d.GrowthPct, d.PrevGB, d.TotalGB))
	}
	// 5) açık olaylar
	if n := len(d.OpenIncidents); n > 0 {
		maxRisk := 0
		for _, in := range d.OpenIncidents {
			if in.RiskScore > maxRisk {
				maxRisk = in.RiskScore
			}
		}
		add(fmt.Sprintf("%d açık olay (incident) var (en yüksek risk %d/100) — /uyarilar → Olaylar "+
			"sekmesinden inceleyip çözün.", n, maxRisk))
	}
	// 6) ağ sağlık skoru
	if d.HealthScore < 60 {
		add(fmt.Sprintf("Ağ sağlık skoru %d/100 — yukarıdaki kesinti kalemlerini önceliklendirin.", d.HealthScore))
	}
	// 7) SLA ihlalleri
	if d.UptimeBreach || d.DeviceBreach || d.IfaceBreach {
		add("Bir veya daha çok SLA hedefi ihlal edildi (yukarıdaki SLA tablosuna bakın) — " +
			"hedef sahibiyle kök-neden analizini paylaşın.")
	}
	// 8) baskın tek hedef
	if len(d.TopEndpoints) >= 2 {
		top := d.TopEndpoints[0]
		var total uint64
		for _, e := range d.TopEndpoints {
			total += e.BytesIn + e.BytesOut
		}
		if share := float64(top.BytesIn+top.BytesOut) / float64(total); total > 0 && share >= 0.5 {
			add(fmt.Sprintf("Dönem trafiğinin ~%%%.0f'i tek bir uzak uca (%s) gitti — "+
				"beklenen bir yedekleme/replikasyon mu, doğrulayın.", share*100, top.IP))
		}
	}

	if len(out) == 0 {
		out = append(out, "Dönem içinde eşik aşan bir sorun tespit edilmedi.")
	}
	return out
}
