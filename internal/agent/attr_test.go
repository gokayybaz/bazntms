package agent

import (
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// AttrEngine, pcap olmadan da delta matematiğini dogru yapmalidir.
func TestAttrDeltas(t *testing.T) {
	e := &AttrEngine{
		totals: map[attrKey][2]uint64{
			{pid: 100, process: "chrome", proto: "tcp", remoteIP: "1.2.3.4", port: 443}: {5000, 1000},
			{pid: 200, process: "curl", proto: "udp", remoteIP: "5.6.7.8", port: 53}:    {0, 300},
		},
		lastSent: map[attrKey][2]uint64{
			{pid: 100, process: "chrome", proto: "tcp", remoteIP: "1.2.3.4", port: 443}: {4000, 800},
		},
	}

	got := e.Deltas()
	if len(got) != 2 {
		t.Fatalf("2 delta beklenirdi: %d", len(got))
	}
	byProc := map[string]telemetry.ProcessTrafficSample{}
	for _, s := range got {
		byProc[s.Process] = s
	}
	if s := byProc["chrome"]; s.BytesIn != 1000 || s.BytesOut != 200 {
		t.Fatalf("chrome deltasi hatali: %+v", s)
	}
	if s := byProc["curl"]; s.BytesIn != 0 || s.BytesOut != 300 {
		t.Fatalf("curl deltasi hatali: %+v", s)
	}

	// ayni verilerle ikinci cagri: deltalar 0 olmali (bos liste)
	if got := e.Deltas(); len(got) != 0 {
		t.Fatalf("ikinci cagri bos donmeliydi: %d", len(got))
	}
}

func TestAttrDeltaCounterReset(t *testing.T) {
	e := &AttrEngine{
		totals:   map[attrKey][2]uint64{{pid: 1, process: "x", proto: "tcp", remoteIP: "9.9.9.9", port: 80}: {50, 0}},
		lastSent: map[attrKey][2]uint64{{pid: 1, process: "x", proto: "tcp", remoteIP: "9.9.9.9", port: 80}: {5000, 0}},
	}
	got := e.Deltas()
	if len(got) != 1 || got[0].BytesIn != 50 {
		t.Fatalf("sayaç sifirlanmasi delta olarak yazilmali: %+v", got)
	}
}
