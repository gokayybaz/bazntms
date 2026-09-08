package store

// Faz 6 testleri: topoloji upsert/dedup, agent subnetleri, baseline
// istatistikleri ve drop istatistikleri (SQLite modu).

import (
	"testing"
	"time"
)

func TestTopologyLinks(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	link := TopologyLink{
		Ts: now, Kind: "lldp", SourceType: "device", SourceID: 1, SourceName: "core-sw",
		LocalPort: "Gi0/1", PeerType: "device", PeerName: "edge-fw", PeerIP: "10.0.0.2",
	}
	if err := st.UpsertTopologyLink(link); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// ayni kenar tekrar: dedup, ts guncellenir
	link.Ts = now + 60
	link.PeerName = "edge-fw [port5]"
	if err := st.UpsertTopologyLink(link); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	// farklı peer adı → ayrı kenar (dedup anahtari peer_name icerir)
	link2 := link
	link2.PeerName = "edge-fw [port5]" // aynı
	if err := st.UpsertTopologyLink(link2); err != nil {
		t.Fatalf("upsert3: %v", err)
	}

	links, err := st.RecentTopologyLinks(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// lldp (peer_name=edge-fw) + lldp (peer_name=edge-fw [port5]) = 2
	if len(links) != 2 {
		t.Fatalf("kenar sayisi: %d (%+v)", len(links), links)
	}

	// subnet kenarlari
	if err := st.SaveAgentSubnets(7, "agent-1", []string{"10.1.0.0/24", "bogus", "192.168.5.0/24"}); err != nil {
		t.Fatalf("subnet: %v", err)
	}
	if err := st.SaveAgentSubnets(7, "agent-1", []string{"10.1.0.0/24"}); err != nil { // dedup
		t.Fatalf("subnet2: %v", err)
	}
	links, _ = st.RecentTopologyLinks(time.Now().Add(-time.Hour))
	subnets := 0
	for _, l := range links {
		if l.Kind == "subnet" {
			subnets++
		}
	}
	if subnets != 2 {
		t.Fatalf("subnet kenari: %d", subnets)
	}

	// prune: 1 saatlik pencere disindakiler silinir
	if err := st.PruneTopology(time.Minute); err != nil {
		t.Fatalf("prune: %v", err)
	}
	links, _ = st.RecentTopologyLinks(time.Unix(0, 0))
	if len(links) != 4 {
		t.Fatalf("prune sonrasi kenar: %d", len(links))
	}
}

func TestBaselineStats(t *testing.T) {
	st := openTest(t)
	now := time.Now()

	// 120 ornek (baseline guvenilirlik esigi) — mevcut SAATIN BASINDAN
	// itibaren serpistirilir (simdiden geriye 120sn degil), boylece test
	// saatin ilk 2 dakikasinda kosarsa kosun (ki bu koşumda 04:01:59'da
	// tam olarak boyle oldu) ornekler saat sinirini asip bir onceki saat
	// kovasina dusmez.
	hourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	for i := int64(0); i < 120; i++ {
		if err := st.InsertSample(Sample{Ts: hourStart.Unix() + i, Device: "en0", BpsIn: 1000, BpsOut: 500, Pps: 10}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}
	stats, err := st.BaselineDayBuckets("local", "bps", 21, "hourly")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("baseline bos")
	}
	var n int64
	var sum float64
	h := time.Now().Hour()
	for _, b := range stats {
		if b.Bucket == h {
			n += b.N
			sum += b.Sum
		}
	}
	if n != 120 {
		t.Fatalf("saatlik baseline ornek sayisi: %d (beklenen saat %d)", n, h)
	}
	if mean := sum / float64(n); mean < 1499 || mean > 1501 {
		t.Fatalf("ortalama: %v", mean)
	}

	avg, err := st.AvgBpsSince(time.Now().Add(-time.Hour))
	if err != nil || avg < 1499 || avg > 1501 {
		t.Fatalf("pencere ortalamasi: %v %v", avg, err)
	}

	dropped, pps, err := st.DropStats(time.Now().Add(-time.Hour))
	if err != nil || pps != 1200 || dropped != 0 {
		t.Fatalf("drop stats: %d %d %v", dropped, pps, err)
	}
}
