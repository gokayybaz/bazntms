package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestMaintainerPrunes, Maintainer.once()'in retention penceresi disindaki
// filo satirlarini (agent_iface_samples, flows) ve bayat topoloji kenarlarini
// sildigini, guncel satirlari koruzdugunu dogrular — Collector calismadan
// (yani -capture=false coklu-hub kurulumunda) da temizlik yapilmali.
func TestMaintainerPrunes(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "maint.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	old := time.Now().Add(-10 * 24 * time.Hour).Unix()
	fresh := time.Now().Add(-1 * time.Hour).Unix()

	aid, _ := st.RegisterAgent(Agent{Name: "a1", TokenHash: "h1"})
	for _, ts := range []int64{old, fresh} {
		if err := st.SaveIfaceSamples(aid, ts, []telemetry.InterfaceSample{{Name: "eth0", RxBytes: 1000}}); err != nil {
			t.Fatalf("iface: %v", err)
		}
	}
	if err := st.SaveFlows([]FlowRow{
		{Ts: old, Device: "r1", Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Octets: 100},
		{Ts: fresh, Device: "r1", Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Octets: 100},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}
	if err := st.UpsertTopologyLink(TopologyLink{Ts: old, Kind: "l2", SourceType: "device", SourceID: 1, SourceName: "sw1", PeerName: "sw2"}); err != nil {
		t.Fatalf("topo old: %v", err)
	}
	if err := st.UpsertTopologyLink(TopologyLink{Ts: fresh, Kind: "l2", SourceType: "device", SourceID: 1, SourceName: "sw1", PeerName: "sw3"}); err != nil {
		t.Fatalf("topo fresh: %v", err)
	}

	NewMaintainer(st, 7*24*time.Hour).once()

	sql := st.(*sqlStore).db
	count := func(tbl string) int {
		var n int
		if err := sql.QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		return n
	}
	if n := count("agent_iface_samples"); n != 1 {
		t.Fatalf("agent_iface_samples: 1 guncel satir bekleniyordu, %d var", n)
	}
	if n := count("flows"); n != 1 {
		t.Fatalf("flows: 1 satir bekleniyordu, %d var", n)
	}

	links, err := st.RecentTopologyLinks(time.Now().Add(-365 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if len(links) != 1 || links[0].PeerName != "sw3" {
		t.Fatalf("topoloji: yalniz guncel kenar (sw3) kalmaliydi, gelen: %+v", links)
	}
}

// TestConfigureRetentionSQLiteNoop, SQLite modunda ConfigureRetention'in
// sessizce basarili donmesini (native TS politikasi yok) dogrular.
func TestConfigureRetentionSQLiteNoop(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "cr.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.ConfigureRetention(72 * time.Hour); err != nil {
		t.Fatalf("SQLite'ta no-op beklenirdi: %v", err)
	}
}
