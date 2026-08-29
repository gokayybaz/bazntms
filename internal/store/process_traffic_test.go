package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestProcessTraffic(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	samples := []telemetry.ProcessTrafficSample{
		{PID: 100, Process: "chrome", Proto: "tcp", RemoteIP: "1.2.3.4", Port: 443, BytesIn: 900, BytesOut: 100},
		{PID: 100, Process: "chrome", Proto: "tcp", RemoteIP: "1.2.3.4", Port: 443, BytesIn: 50, BytesOut: 0},
		{PID: 200, Process: "spotify", Proto: "tcp", RemoteIP: "5.6.7.8", Port: 443, BytesIn: 300, BytesOut: 400},
	}
	if err := st.SaveProcessTraffic(1, now, samples); err != nil {
		t.Fatalf("kayit: %v", err)
	}
	// bos delta kaydedilmez
	if err := st.SaveProcessTraffic(1, now, []telemetry.ProcessTrafficSample{{PID: 1, Process: "idle"}}); err != nil {
		t.Fatalf("bos kayit: %v", err)
	}

	top, err := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10)
	if err != nil {
		t.Fatalf("sorgu: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("2 surec beklenirdi: %d", len(top))
	}
	if top[0].Process != "chrome" || top[0].Total != 1050 {
		t.Fatalf("chrome toplami hatali: %+v", top[0])
	}
	if top[0].AgentCnt != 1 {
		t.Fatalf("agent sayisi hatali: %+v", top[0])
	}

	// agent filtresi
	byAgent, _ := st.TopProcessTraffic(time.Now().Add(-time.Hour), 1, 10)
	if len(byAgent) != 2 {
		t.Fatalf("agent 1 filtresi: %d", len(byAgent))
	}
	byAgent2, _ := st.TopProcessTraffic(time.Now().Add(-time.Hour), 2, 10)
	if len(byAgent2) != 0 {
		t.Fatalf("olmayan agent icin bos donmeliydi: %d", len(byAgent2))
	}

	// temizlik: taze kayitlar korunur, 1 saat eski "hepsi" silinir
	if err := st.Prune(30 * time.Second); err != nil {
		t.Fatalf("prune: %v", err)
	}
	after, _ := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10)
	if len(after) != 2 {
		t.Fatalf("taze kayitlar prune edilmemeli: %d", len(after))
	}
	if err := st.Prune(-time.Hour); err != nil {
		t.Fatalf("prune2: %v", err)
	}
	after2, _ := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10)
	if len(after2) != 0 {
		t.Fatalf("tam temizlik sonrasi kayit kalmis: %d", len(after2))
	}
}
