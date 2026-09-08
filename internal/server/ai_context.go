package server

// AI bağlam anlık görüntüsü (Faz 26 S26.7). Sohbet oluşturulurken / bağlam
// yenilenirken kapsam (fleet|agent|incident|anomaly|device) için ilgili
// store/alert/health sorgularını çalıştırır ve ai.Snapshot doldurur.
// internal/ai bunu token-bütçeli JSON'a çevirir.

import (
	"fmt"
	"time"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/health"
)

const aiContextWindow = 24 * time.Hour

func (s *Server) BuildAISnapshot(scope, ref, site string) ai.Snapshot {
	snap := ai.Snapshot{Scope: scope, Ref: ref, Period: "son 24 saat"}
	since := time.Now().Add(-aiContextWindow)

	switch scope {
	case "agent":
		s.fillAgentSnapshot(&snap, ref, since, site)
	case "incident":
		s.fillIncidentSnapshot(&snap, ref)
	case "device":
		s.fillDeviceSnapshot(&snap, ref, since, site)
	case "anomaly":
		s.fillAnomalySnapshot(&snap, site)
	default: // fleet
		s.fillFleetSnapshot(&snap, since, site)
	}
	return snap
}

func (s *Server) fillFleetSnapshot(snap *ai.Snapshot, since time.Time, site string) {
	onlineWin := 2 * time.Duration(s.telemetryInterval) * time.Second
	if fs, err := s.store.FleetSummary(onlineWin); err == nil {
		snap.Add("Filo özeti", map[string]any{
			"agent_toplam": fs.AgentsTotal, "agent_online": fs.AgentsOnline,
			"rx_bps": round2(fs.RxBps), "tx_bps": round2(fs.TxBps),
			"pps": round2(fs.Pps), "flow_dk": fs.FlowsPerMin,
		})
	}
	sc := health.Compute(s.collectHealthInputs())
	snap.Add("Ağ sağlık skoru", map[string]any{"skor": sc.Score, "kesintiler": sc.Deductions})
	if pt, err := s.store.FleetProtocolTotals(since); err == nil && len(pt) > 0 {
		snap.Add("Protokol dağılımı", pt)
	}
	if eps, err := s.store.FleetTopEndpoints(since, 12, site); err == nil && len(eps) > 0 {
		snap.Add("En yoğun hedefler", eps)
	}
	if ds, err := s.store.TopDomainsSince(since, 15); err == nil && len(ds) > 0 {
		snap.Add("En çok sorgulanan alan adları", ds)
	}
	if ps, err := s.store.TopProcessTraffic(since, 0, 12, site); err == nil && len(ps) > 0 {
		snap.Add("En aktif süreçler (trafik)", ps)
	}
	if devs := s.anomalyDeviations(site); len(devs) > 0 {
		snap.Add("Aktif anomali sapmaları", devs)
	}
	if incs, err := s.store.RecentOpenIncidents(since); err == nil && len(incs) > 0 {
		snap.Add("Açık olaylar (incident)", incs)
	}
}

func (s *Server) fillAgentSnapshot(snap *ai.Snapshot, ref string, since time.Time, site string) {
	id := parseInt64(ref)
	if a, err := s.store.AgentByID(id); err == nil && a != nil {
		snap.Add("Agent", map[string]any{
			"id": a.ID, "ad": a.Name, "site": a.Site, "sürüm": a.Version,
			"son_görülme": a.LastSeen, "atıf_yöntemi": a.AttrMethod,
		})
	}
	if ps, err := s.store.TopProcessTraffic(since, id, 15, ""); err == nil && len(ps) > 0 {
		snap.Add("Süreç trafiği", ps)
	}
	if l7, err := s.store.TopL7(since, id, 15, ""); err == nil && len(l7) > 0 {
		snap.Add("L7 uygulama görünürlüğü (SNI/Host)", l7)
	}
	if dns, err := s.store.TopAgentDNS(since, id, 15, ""); err == nil && len(dns) > 0 {
		snap.Add("DNS sorguları", dns)
	}
	if h, err := s.store.AgentHistory(id, since); err == nil && len(h) > 0 {
		snap.Add("Trafik geçmişi (kova)", h)
	}
}

func (s *Server) fillIncidentSnapshot(snap *ai.Snapshot, ref string) {
	in, evs, err := s.store.IncidentByID(parseInt64(ref))
	if err != nil || in == nil {
		return
	}
	snap.Period = fmt.Sprintf("olay #%d penceresi", in.ID)
	snap.Add("Olay (incident)", in)
	if len(evs) > 0 {
		snap.Add("Kanıt zaman çizelgesi", evs)
	}
	if in.AgentID > 0 {
		if a, aerr := s.store.AgentByID(in.AgentID); aerr == nil && a != nil {
			snap.Add("İlgili agent", map[string]any{"id": a.ID, "ad": a.Name, "site": a.Site})
		}
	}
}

func (s *Server) fillDeviceSnapshot(snap *ai.Snapshot, ref string, since time.Time, site string) {
	id := parseInt64(ref)
	if d, err := s.store.DeviceByID(id); err == nil && d != nil {
		snap.Add("Cihaz", map[string]any{
			"id": d.ID, "ad": d.Name, "host": d.Host, "vendor": d.Vendor, "site": d.Site,
		})
	}
	if ifs, err := s.store.LatestDeviceIfaces(id); err == nil && len(ifs) > 0 {
		snap.Add("Arayüz oranları", ifs)
	}
}

func (s *Server) fillAnomalySnapshot(snap *ai.Snapshot, site string) {
	if devs := s.anomalyDeviations(site); len(devs) > 0 {
		snap.Add("Aktif anomali sapmaları", devs)
	} else {
		snap.Add("Aktif anomali sapmaları", []string{"şu an eşik aşan sapma yok"})
	}
}

// anomalyDeviations, alert motorundan aktif sapmalar (site-kapsamlı süzülür).
func (s *Server) anomalyDeviations(site string) []any {
	if s.alerts == nil {
		return nil
	}
	raw := s.alerts.AnomalyActive()
	out := make([]any, 0, len(raw))
	for _, d := range raw {
		if site != "" && !(d.Dim == "fleet" || d.Dim == "local" || (d.Dim == "site" && d.Key == site)) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
