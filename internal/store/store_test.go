package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestStoreRoundTrip(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	// ornekler
	for i := int64(0); i < 120; i++ {
		err := st.InsertSample(Sample{
			Ts: now - 120 + i, Device: "en0",
			BpsIn: 8000, BpsOut: 2000, BpsLocal: 100, Pps: 10,
			Protocols: map[string]uint64{"TCP": 5, "UDP": 2},
		})
		if err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}

	buckets, err := st.TimeseriesBuckets(time.Now().Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("buckets: %v", err)
	}
	if len(buckets) < 1 || len(buckets) > 3 {
		t.Fatalf("beklenmedik kova sayisi: %d", len(buckets))
	}
	if got := buckets[0].In * 8; got < 7900 || got > 8100 {
		t.Fatalf("bps_in donusumu hatali: %v", got)
	}

	tot, err := st.PeriodTotals(time.Now().Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if tot.Samples != 120 || tot.PeakBpsIn != 8000 {
		t.Fatalf("totals hatali: %+v", tot)
	}

	// endpoint farklari
	if err := st.InsertEndpointDeltas([]EndpointDelta{
		{Ts: now, Device: "en0", IP: "1.2.3.4", BytesIn: 1000, BytesOut: 500, Packets: 10},
		{Ts: now, Device: "en0", IP: "5.6.7.8", BytesIn: 2000, BytesOut: 100, Packets: 5},
	}); err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	eps, err := st.TopEndpointsSince(time.Now().Add(-time.Hour), 10)
	if err != nil || len(eps) != 2 {
		t.Fatalf("endpoint sorgu: %v %d", err, len(eps))
	}
	if eps[0].IP != "5.6.7.8" {
		t.Fatalf("siralama hatali: %s", eps[0].IP)
	}

	// protokoller
	protos, err := st.ProtocolTotals(time.Now().Add(-time.Hour))
	if err != nil || protos["TCP"] != 600 || protos["UDP"] != 240 {
		t.Fatalf("protokol toplami: %v %v", err, protos)
	}

	// surecler
	if err := st.InsertConnectionEvents([]ConnectionEvent{
		{Ts: now, Proto: "tcp", LocalAddr: "a", RemoteAddr: "b", Process: "chrome", Count: 3},
		{Ts: now, Proto: "tcp", LocalAddr: "c", RemoteAddr: "d", Process: "chrome", Count: 1},
		{Ts: now, Proto: "udp", LocalAddr: "e", RemoteAddr: "f", Process: "spotify", Count: 1},
	}); err != nil {
		t.Fatalf("connection: %v", err)
	}
	procs, err := st.TopProcessesSince(time.Now().Add(-time.Hour), 10)
	if err != nil || len(procs) != 2 || procs[0].Process != "chrome" || procs[0].Connections != 2 {
		t.Fatalf("surec sorgu: %v %+v", err, procs)
	}

	// temizlik: 30 saniyeden eski kayitlar silinir, yeniler kalir. Kalan
	// sayi prune ile test baslangici arasindaki saniye kaymasina bagli
	// oldugundan (30-29 gibi) olcek kontrolu yapilir: kalan her ornek
	// TCP:5, UDP:2 tasir.
	if err := st.Prune(30 * time.Second); err != nil {
		t.Fatalf("prune: %v", err)
	}
	rem, err := st.PeriodTotals(time.Now().Add(-time.Hour))
	if err != nil || rem.Samples == 0 || rem.Samples >= 120 {
		t.Fatalf("prune sonrasi kalan ornek sayisi hatali: %v %+v", err, rem)
	}
	protos2, _ := st.ProtocolTotals(time.Now().Add(-time.Hour))
	if protos2["TCP"] != uint64(5*rem.Samples) || protos2["UDP"] != uint64(2*rem.Samples) {
		t.Fatalf("prune sonrasi kalan kayitlar hatali: %v (%d ornek)", protos2, rem.Samples)
	}
}

func TestDailyTotalsAndHourly(t *testing.T) {
	st := openTest(t)
	localMid := func(offsetDays int) time.Time {
		n := time.Now()
		return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location()).AddDate(0, 0, offsetDays)
	}

	// dun: 05:00 ve 17:00'de ornekler; bugun: 08:00'de ornek
	insert := func(base time.Time, hour int, in float64) {
		ts := base.Add(time.Duration(hour) * time.Hour).Unix()
		if err := st.InsertSample(Sample{Ts: ts, Device: "en0", BpsIn: in, BpsOut: 1000, Pps: 10}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}
	yest := localMid(-1)
	insert(yest, 5, 50_000_000)
	insert(yest, 17, 10_000_000)
	insert(localMid(0), 8, 20_000_000)

	daily, err := st.DailyTotals(7)
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(daily) != 2 {
		t.Fatalf("2 gun beklenirdi: %d", len(daily))
	}
	// sirali: dun, bugun
	if daily[0].Day != yest.Unix() {
		t.Fatalf("gun hizalamasi hatali: %d != %d", daily[0].Day, yest.Unix())
	}
	if daily[0].PeakBpsIn != 50_000_000 || daily[0].AvgBpsIn != 30_000_000 {
		t.Fatalf("dunun ozeti hatali: %+v", daily[0])
	}
	if daily[1].AvgBpsIn != 20_000_000 {
		t.Fatalf("bugunun ortalamasi hatali: %+v", daily[1])
	}

	todayH, err := st.HourlyAverages(localMid(0))
	if err != nil {
		t.Fatalf("hourly: %v", err)
	}
	if len(todayH) != 24 {
		t.Fatalf("24 saat beklenirdi: %d", len(todayH))
	}
	if todayH[8].BpsIn != 20_000_000 {
		t.Fatalf("08:00 ortalamasi hatali: %+v", todayH[8])
	}
	if todayH[5].BpsIn != 0 {
		t.Fatalf("bos saat 0 olmali: %+v", todayH[5])
	}

	yestH, _ := st.HourlyAverages(yest)
	if yestH[5].BpsIn != 50_000_000 || yestH[17].BpsIn != 10_000_000 {
		t.Fatalf("dunun saatlik serisi hatali: %+v", yestH[5])
	}
}

func TestInsights(t *testing.T) {
	st := openTest(t)
	id, err := st.InsertInsight(Insight{Ts: time.Now().Unix(), Model: "test-model", PeriodMinutes: 30, Summary: "merhaba"})
	if err != nil {
		t.Fatalf("insight: %v", err)
	}
	if id == 0 {
		t.Fatal("id donmedi")
	}
	list, err := st.RecentInsights(5)
	if err != nil || len(list) != 1 || list[0].Summary != "merhaba" || list[0].Model != "test-model" {
		t.Fatalf("insight listesi: %v %+v", err, list)
	}
}
