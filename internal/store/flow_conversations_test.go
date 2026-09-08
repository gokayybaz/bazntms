package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestFlowConversations(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	// A↔B iki yön + A→C; B→A farklı port ama aynı çift.
	if err := st.SaveFlows([]FlowRow{
		{Ts: now - 60, Device: "fw1", Src: "10.0.0.1", Dst: "8.8.8.8", SrcPort: 5000, DstPort: 443, Proto: "tcp", Packets: 10, Octets: 1000},
		{Ts: now - 30, Device: "fw1", Src: "10.0.0.1", Dst: "8.8.8.8", SrcPort: 5000, DstPort: 443, Proto: "tcp", Packets: 5, Octets: 500},
		{Ts: now - 20, Device: "fw1", Src: "8.8.8.8", Dst: "10.0.0.1", SrcPort: 443, DstPort: 5000, Proto: "tcp", Packets: 8, Octets: 4000},
		{Ts: now - 10, Device: "fw1", Src: "10.0.0.1", Dst: "1.1.1.1", SrcPort: 5001, DstPort: 53, Proto: "udp", Packets: 2, Octets: 120},
	}); err != nil {
		t.Fatalf("saveflows: %v", err)
	}

	since := time.Now().Add(-time.Hour)

	// --- 5'li ---
	five, err := st.FlowConversations(since, "5tuple", "octets", 10, "")
	if err != nil {
		t.Fatalf("5tuple: %v", err)
	}
	if len(five) != 3 {
		t.Fatalf("3 benzersiz 5'li beklenirdi: %+v", five)
	}
	// en yoğun: 8.8.8.8→10.0.0.1 (4000 octet)
	if five[0].Src != "8.8.8.8" || five[0].Octets != 4000 || five[0].Flows != 1 {
		t.Fatalf("ilk 5'li hatalı: %+v", five[0])
	}
	// 10.0.0.1→8.8.8.8: 2 akış, 1500 octet
	var ab *FlowConversation
	for i := range five {
		if five[i].Src == "10.0.0.1" && five[i].Dst == "8.8.8.8" {
			ab = &five[i]
		}
	}
	if ab == nil || ab.Flows != 2 || ab.Octets != 1500 || ab.Packets != 15 {
		t.Fatalf("A→B 5'li hatalı: %+v", ab)
	}

	// --- uç-çifti (yön birleşik) ---
	pair, err := st.FlowConversations(since, "pair", "octets", 10, "")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if len(pair) != 2 {
		t.Fatalf("2 uç-çifti beklenirdi (A↔B, A↔C): %+v", pair)
	}
	// A↔B: 3 akış, 5500 octet, kanonik Src = "10.0.0.1" (sözlüksel küçük)
	top := pair[0]
	if top.Src != "10.0.0.1" || top.Dst != "8.8.8.8" || top.Flows != 3 || top.Octets != 5500 || top.Packets != 23 {
		t.Fatalf("A↔B çifti hatalı: %+v", top)
	}
	// reconcile: toplam octet = ham akış octet toplamı
	var rawTotal uint64
	for _, c := range five {
		rawTotal += c.Octets
	}
	var pairTotal uint64
	for _, c := range pair {
		pairTotal += c.Octets
	}
	if rawTotal != pairTotal {
		t.Fatalf("agregat mutabık değil: 5tuple=%d pair=%d", rawTotal, pairTotal)
	}

	// --- drill-down ---
	det, err := st.FlowConversationDetail(since, "10.0.0.1", "8.8.8.8", "", 100, "")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(det) != 3 {
		t.Fatalf("çift yönlü 3 ham akış beklenirdi: %+v", det)
	}

	// --- süreç korelasyonu ---
	aid, _ := st.RegisterAgent(Agent{Name: "host-a", TokenHash: TokenHash("fa")})
	if err := st.SaveProcessTraffic(aid, now-30, []telemetry.ProcessTrafficSample{
		{PID: 1, Process: "curl", Proto: "tcp", RemoteIP: "8.8.8.8", Port: 443, BytesIn: 100, BytesOut: 50},
	}); err != nil {
		t.Fatalf("pt: %v", err)
	}
	actors, err := st.FlowActorsForConversation(since, "10.0.0.1", "8.8.8.8")
	if err != nil {
		t.Fatalf("actors: %v", err)
	}
	if len(actors) != 1 || actors[0].Process != "curl" || actors[0].IP != "8.8.8.8" || actors[0].AgentName != "host-a" {
		t.Fatalf("korelasyon hatalı: %+v", actors)
	}
}

func TestFlowConversationsSiteScope(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()
	if _, err := st.AddDevice(Device{Name: "fw-a", Host: "10.0.0.254", Kind: "firewall", Site: "a", Vendor: "snmp"}); err != nil {
		t.Fatalf("dev a: %v", err)
	}
	_ = st.SaveFlows([]FlowRow{
		{Ts: now - 10, Device: "10.0.0.254", Src: "10.0.0.1", Dst: "9.9.9.9", Proto: "tcp", Packets: 1, Octets: 100},
		{Ts: now - 10, Device: "10.9.9.9", Src: "10.9.0.1", Dst: "9.9.9.9", Proto: "tcp", Packets: 1, Octets: 999},
	})
	since := time.Now().Add(-time.Hour)

	all, _ := st.FlowConversations(since, "pair", "octets", 10, "")
	if len(all) != 2 {
		t.Fatalf("site'sız 2 konuşma: %+v", all)
	}
	scoped, _ := st.FlowConversations(since, "pair", "octets", 10, "a")
	if len(scoped) != 1 || scoped[0].Dst != "9.9.9.9" || scoped[0].Octets != 100 {
		t.Fatalf("site 'a' yalnız kendi exporter'ı: %+v", scoped)
	}
}
