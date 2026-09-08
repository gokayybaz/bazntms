//go:build !windows

package store

// Faz 4.1: PostgreSQL/TimescaleDB store entegrasyon testleri (testcontainers).
//   - TestPostgresStore: duz PostgreSQL uzerinde tam Store round-trip
//   - TestTimescaleSetup: hypertable + continuous aggregate + real-time cagg
// Docker yoksa (yerel ortam) testler skip edilir; CI ubuntu runner'inda
// gercekten calisir.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// pgContainerDSN, TAZE (henuz hic Open() ile migrate edilmemis) bir
// Postgres container'i ayaga kaldirip DSN'sini dondurur — cagiran kendi
// Open() cagrisini (tek veya coklu/es zamanli) yapar.
func pgContainerDSN(t *testing.T, image string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("short mod")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// ForSQL, Postgres sorgusu yanitlayana dek bekler; dinlenen port yetmez
	// (init sirasinda sunucu bir kez yeniden baslar)
	dsnFor := func(host string, port network.Port) string {
		return fmt.Sprintf("postgres://bazntms:test@%s:%s/bazntms?sslmode=disable", host, port.Port())
	}
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "bazntms",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "bazntms",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "pgx", dsnFor).WithStartupTimeout(2 * time.Minute),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("docker kullanilamadi, atlanıyor: %v", err)
	}
	t.Cleanup(func() { ctr.Terminate(context.Background()) })

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	return dsnFor(host, port)
}

func pgStoreFor(t *testing.T, image string) Store {
	t.Helper()
	dsn := pgContainerDSN(t, image)
	st, err := Open(dsn)
	if err != nil {
		t.Fatalf("store acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestPostgresStore(t *testing.T) {
	st := pgStoreFor(t, "postgres:16-alpine")
	if err := st.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	now := time.Now().Unix()

	// ornekler + upsert (ON CONFLICT): son satir (ts=now-1) 9999 ile
	// GUNCELLENIR — satir sayisi 120 kalir, tepe deger upsert'ten gelir
	for i := int64(0); i < 120; i++ {
		if err := st.InsertSample(Sample{
			Ts: now - 120 + i, Device: "en0",
			BpsIn: 8000, BpsOut: 2000, BpsLocal: 100, Pps: 10,
			Protocols: map[string]uint64{"TCP": 5, "UDP": 2},
		}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}
	if err := st.InsertSample(Sample{Ts: now - 1, Device: "en0", BpsIn: 9999, Protocols: map[string]uint64{"TCP": 1}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	buckets, err := st.TimeseriesBuckets(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("buckets: %v", err)
	}
	if len(buckets) < 1 || len(buckets) > 3 {
		t.Fatalf("kova sayisi: %d", len(buckets))
	}
	tot, err := st.PeriodTotals(time.Now().Add(-time.Hour))
	if err != nil || tot.Samples != 120 || tot.PeakBpsIn != 9999 {
		t.Fatalf("totals: %v %+v", err, tot)
	}

	// endpoint farklari
	if err := st.InsertEndpointDeltas([]EndpointDelta{
		{Ts: now, Device: "en0", IP: "1.2.3.4", BytesIn: 1000, BytesOut: 500, Packets: 10},
		{Ts: now, Device: "en0", IP: "5.6.7.8", BytesIn: 2000, BytesOut: 100, Packets: 5},
	}); err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	eps, err := st.TopEndpointsSince(time.Now().Add(-time.Hour), 10)
	if err != nil || len(eps) != 2 || eps[0].IP != "5.6.7.8" {
		t.Fatalf("endpoint sorgu: %v %+v", err, eps)
	}

	protos, err := st.ProtocolTotals(time.Now().Add(-time.Hour))
	// 119 satir x (TCP:5, UDP:2) + upsert satiri (TCP:1)
	if err != nil || protos["TCP"] != 596 || protos["UDP"] != 238 {
		t.Fatalf("protokol: %v %v", err, protos)
	}

	// agent filosu
	id, err := st.RegisterAgent(Agent{Name: "pg-agent", Site: "dc1", TokenHash: TokenHash("tok"), Version: "0.1", ProtocolVersion: 1})
	if err != nil || id == 0 {
		t.Fatalf("register: %v %d", err, id)
	}
	if a, err := st.AgentByTokenHash(TokenHash("tok")); err != nil || a.Name != "pg-agent" {
		t.Fatalf("token sorgu: %v %+v", err, a)
	}
	if err := st.TouchAgent(id, "0.2", 1, "10.0.0.9"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if a, _ := st.AgentByTokenHash(TokenHash("tok")); a == nil || a.Version != "0.2" {
		t.Fatalf("TouchAgent surumu guncellemeli: %+v", a)
	}
	// verim hesabi icin iki farkli zaman damgasi gerekir (ilk/son delta)
	if err := st.SaveIfaceSamples(id, now-60, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 500, TxBytes: 250, RxPackets: 5, TxPackets: 2},
	}); err != nil {
		t.Fatalf("iface ornek1: %v", err)
	}
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
	}); err != nil {
		t.Fatalf("iface ornek2: %v", err)
	}
	agents, err := st.ListAgents(time.Hour, "")
	if err != nil || len(agents) != 1 || !agents[0].Online || len(agents[0].Rates) != 1 {
		t.Fatalf("filo: %v %+v", err, agents)
	}

	// surec trafigi
	if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
		{PID: 42, Process: "curl", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 900, BytesOut: 100},
	}); err != nil {
		t.Fatalf("surec trafik: %v", err)
	}
	pt, err := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10, "")
	if err != nil || len(pt) != 1 || pt[0].Process != "curl" || pt[0].Total != 1000 {
		t.Fatalf("surec top: %v %+v", err, pt)
	}

	// cihaz + SNMP ornekleri
	devID, err := st.AddDevice(Device{Name: "rt-1", Host: "10.0.0.1", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60})
	if err != nil || devID == 0 {
		t.Fatalf("cihaz: %v %d", err, devID)
	}
	devices, err := st.ListDevices("")
	if err != nil || len(devices) != 1 || !devices[0].Enabled {
		t.Fatalf("cihaz listesi: %v %+v", err, devices)
	}
	if err := st.SaveDeviceIfaceSamples(devID, now, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1e9, OperStatus: 1, RxBytes: 5000, TxBytes: 3000},
	}); err != nil {
		t.Fatalf("cihaz arayuz: %v", err)
	}
	ifaces, err := st.LatestDeviceIfaces(devID)
	if err != nil || len(ifaces) != 1 || ifaces[0].Name != "Gi0/1" {
		t.Fatalf("arayuz sorgu: %v %+v", err, ifaces)
	}
	if err := st.UpdateDevicePoll(devID, "rt-1.local", "Router X", ""); err != nil {
		t.Fatalf("poll guncelleme: %v", err)
	}

	// flow + syslog
	if err := st.SaveFlows([]FlowRow{{Ts: now, Device: "rt-1", Src: "1.2.3.4", Dst: "5.6.7.8", SrcPort: 1024, DstPort: 443, Proto: "tcp", Packets: 20, Octets: 9000}}); err != nil {
		t.Fatalf("flow: %v", err)
	}
	fl, err := st.TopFlows(time.Now().Add(-time.Hour), 10, "")
	if err != nil || len(fl) != 1 || fl[0].Octets != 9000 {
		t.Fatalf("flow sorgu: %v %+v", err, fl)
	}

	// S21.8: PG çok-satırlı toplu yazım + chunk sınırı (flows 9 kolon,
	// pgMaxParams/9 ≈ 6666 → 15000 satır 3 chunk).
	big := make([]FlowRow, 15000)
	for i := range big {
		big[i] = FlowRow{Ts: now, Device: "rt-bulk", Src: "10.0.0.1", Dst: "9.9.9.9", SrcPort: uint16(1 + i%60000), DstPort: 53, Proto: "udp", Packets: 1, Octets: uint64(i + 1)}
	}
	if err := st.SaveFlows(big); err != nil {
		t.Fatalf("toplu flow: %v", err)
	}
	var bulkCnt int
	if err := st.(*sqlStore).db.QueryRow("SELECT COUNT(*) FROM flows WHERE device = $1", "rt-bulk").Scan(&bulkCnt); err != nil {
		t.Fatalf("toplu flow count: %v", err)
	}
	if bulkCnt != 15000 {
		t.Fatalf("toplu flow satır sayısı = %d, beklenen 15000", bulkCnt)
	}
	if err := st.SaveSyslogEvent(SyslogEvent{Ts: now, Host: "rt-1", Severity: 4, Tag: "LINK", Message: "up/down"}); err != nil {
		t.Fatalf("syslog: %v", err)
	}
	sy, err := st.RecentSyslog(10, "")
	if err != nil || len(sy) != 1 || sy[0].Host != "rt-1" {
		t.Fatalf("syslog sorgu: %v %+v", err, sy)
	}

	// uyarilar + config
	aeID, err := st.InsertAlertEvent(AlertEvent{Ts: now, Kind: "bw", Key: "en0", Message: "yuksek bant", Severity: "warn", Site: "dc1"})
	if err != nil {
		t.Fatalf("uyari: %v", err)
	}
	// S22.6: yaşam döngüsü — open sorgu + bump (PG)
	open, err := st.OpenAlertEventByKey("bw", "en0")
	if err != nil || open == nil || open.ID != aeID || open.Count != 1 || open.State != "firing" || open.Site != "dc1" {
		t.Fatalf("OpenAlertEventByKey: %v %+v", err, open)
	}
	if err := st.BumpAlertEvent(aeID, now+10, "hala yuksek"); err != nil {
		t.Fatalf("BumpAlertEvent: %v", err)
	}
	if o, _ := st.OpenAlertEventByKey("bw", "en0"); o.Count != 2 || o.LastTs != now+10 {
		t.Fatalf("bump sonrası: %+v", o)
	}
	// S22.8/S22.9: stale + kind + site sorguları, resolve, grup ataması
	if st2, _ := st.OpenAlertEventsStale(now + 20); len(st2) != 1 {
		t.Fatalf("stale sorgu: %d", len(st2))
	}
	if k, _ := st.OpenAlertEventsByKind("bw"); len(k) != 1 {
		t.Fatalf("kind sorgu: %d", len(k))
	}
	if ss, _ := st.OpenAlertEventsBySiteSince("dc1", now-100); len(ss) != 1 {
		t.Fatalf("site-since sorgu: %d", len(ss))
	}
	if err := st.SetAlertEventGroup(aeID, "g-1"); err != nil {
		t.Fatalf("grup ata: %v", err)
	}
	// S22.14: ext_ref + grup bilet sorgusu
	if err := st.SetAlertEventExtRef(aeID, "jira:OPS-9"); err != nil {
		t.Fatalf("ext_ref ata: %v", err)
	}
	if ref, _ := st.GroupExtRef("g-1"); ref != "jira:OPS-9" {
		t.Fatalf("GroupExtRef: %q", ref)
	}
	if err := st.ResolveAlertEvent(aeID, now+15); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if o, _ := st.OpenAlertEventByKey("bw", "en0"); o != nil {
		t.Fatalf("resolved olay açık dönmemeli")
	}
	// S22.10: susturma CRUD (PG)
	sid, err := st.AddAlertSilence(AlertSilence{MatchKind: "anomaly", StartsTs: now - 10, EndsTs: now + 600, Reason: "bakım", CreatedTs: now})
	if err != nil || sid == 0 {
		t.Fatalf("silence add: %v %d", err, sid)
	}
	if a, _ := st.ListAlertSilences(true, now); len(a) != 1 {
		t.Fatalf("aktif silence: %d", len(a))
	}
	if err := st.DeleteAlertSilence(sid); err != nil {
		t.Fatalf("silence delete: %v", err)
	}

	if err := st.MarkAlertSeen("proc", "curl"); err != nil {
		t.Fatalf("seen: %v", err)
	}
	if ok, err := st.IsAlertSeen("proc", "curl"); err != nil || !ok {
		t.Fatalf("seen sorgu: %v %v", ok, err)
	}
	if err := st.SaveAlertConfig(`{"bw_mbps":100}`); err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg, err := st.LoadAlertConfig(); err != nil || cfg != `{"bw_mbps":100}` {
		t.Fatalf("config okuma: %v %q", err, cfg)
	}

	// S22.1: materyalize anomali baseline (tx DELETE+INSERT + ? -> $N)
	if err := st.SaveAnomalyBaseline([]AnomalyBaselineRow{
		{Dim: "fleet", Metric: "bps", Bucket: 3, N: 200, Mean: 1000, M2: 200 * 400},
	}); err != nil {
		t.Fatalf("anomali baseline yaz: %v", err)
	}
	if err := st.SaveAnomalyBaseline([]AnomalyBaselineRow{
		{Dim: "fleet", Metric: "bps", Bucket: 4, N: 210, Mean: 1100, M2: 210 * 441},
	}); err != nil {
		t.Fatalf("anomali baseline yeniden kur: %v", err)
	}
	ab, err := st.LoadAnomalyBaseline("fleet", "bps")
	if err != nil || len(ab) != 1 || ab[0].Bucket != 4 || ab[0].Std() != 21 {
		t.Fatalf("anomali baseline sorgu: %v %+v", err, ab)
	}
	// S22.2: mevsimsel baseline alt-toplamları — alias GROUP BY + tamsayı / %
	// PG semantiği. samples 120 satır (hepsi güncel saat) → en az 1 kova.
	lb, err := st.BaselineDayBuckets("local", "bps", 21, "weekday")
	if err != nil || len(lb) == 0 {
		t.Fatalf("BaselineDayBuckets: %v (%d satır)", err, len(lb))
	}
	if _, err := st.BaselineDayBuckets("fleet", "bps", 21, "dow"); err != nil {
		t.Fatalf("BaselineDayBuckets(fleet): %v", err)
	}
	// S22.3/S22.4: saha/agent boyutu + bps-dışı metrikler — JOIN agents +
	// CAST(agent_id AS TEXT) + SUM path PG'de.
	for _, dm := range [][2]string{{"site", "bps"}, {"agent", "bps"}, {"fleet", "dns_qps"}, {"agent", "proc_bps"}} {
		if _, err := st.BaselineDayBuckets(dm[0], dm[1], 21, "weekday"); err != nil {
			t.Fatalf("BaselineDayBuckets(%s,%s): %v", dm[0], dm[1], err)
		}
		if _, err := st.AvgMetricByDim(dm[0], dm[1], time.Now().Add(-time.Hour)); err != nil {
			t.Fatalf("AvgMetricByDim(%s,%s): %v", dm[0], dm[1], err)
		}
	}

	// S22.18: zamanlanmış işler (PG bool/int enabled + due sorgu)
	sjID, err := st.CreateScheduledJob(ScheduledJob{Kind: "report", Spec: "daily:08:00", Payload: `{"type":"enterprise"}`, Enabled: true, NextRunTs: now - 10, CreatedTs: now})
	if err != nil || sjID == 0 {
		t.Fatalf("scheduled job: %v %d", err, sjID)
	}
	if due, _ := st.DueScheduledJobs(now); len(due) != 1 || !due[0].Enabled {
		t.Fatalf("due job: %+v", due)
	}
	if err := st.MarkScheduledJobRun(sjID, now, now+86400, "ok"); err != nil {
		t.Fatalf("mark run: %v", err)
	}
	if due, _ := st.DueScheduledJobs(now); len(due) != 0 {
		t.Fatalf("koşumdan sonra due olmamalı")
	}
	// S22.19: rapor arşivi
	raID, err := st.InsertReportArchive(ReportArchive{Kind: "enterprise", Site: "dc1", Days: 30, Format: "html", Path: "/data/reports/x.html", Size: 123, GeneratedTs: now, Status: "ok", JobID: sjID})
	if err != nil || raID == 0 {
		t.Fatalf("report archive: %v %d", err, raID)
	}
	if l, _ := st.ListReportArchive("dc1", 10); len(l) != 1 || l[0].Kind != "enterprise" {
		t.Fatalf("archive list: %+v", l)
	}
	if paths, err := st.PruneReportArchive(now + 1); err != nil || len(paths) != 1 {
		t.Fatalf("prune: %v %v", err, paths)
	}
	if err := st.DeleteScheduledJob(sjID); err != nil {
		t.Fatalf("delete job: %v", err)
	}

	// baglanti olaylari + temizlik: son ~30 saniyedeki ornekler kalir
	// (kalan sayi saniye kaymasina bagli; 0'dan fazla, tumunden az olmali)
	if err := st.InsertConnectionEvents([]ConnectionEvent{{Ts: now, Proto: "tcp", LocalAddr: "a", RemoteAddr: "b", Process: "chrome", Count: 3}}); err != nil {
		t.Fatalf("connection: %v", err)
	}
	if err := st.Prune(30 * time.Second); err != nil {
		t.Fatalf("prune: %v", err)
	}
	tot2, _ := st.PeriodTotals(time.Now().Add(-time.Hour))
	if tot2.Samples == 0 || tot2.Samples >= 120 {
		t.Fatalf("prune sonrasi: %d", tot2.Samples)
	}
}

func TestTimescaleSetup(t *testing.T) {
	st := pgStoreFor(t, "timescale/timescaledb:latest-pg16")

	s, ok := st.(*sqlStore)
	if !ok || !s.ts {
		t.Fatal("timescale eklentisi algilanmadi")
	}
	var hypertables, caggs int
	// katalog tablosu surumler arasinda degistigine (orn. hyp.dropped kolonu
	// kaldırıldı) stabil information view kullanilir
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM timescaledb_information.hypertables`).Scan(&hypertables); err != nil || hypertables < 9 {
		t.Fatalf("hypertable sayisi: %d (err: %v)", hypertables, err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_name IN ('samples_1m','samples_1h','flows_1h','process_traffic_1h',
			'flows_dst_1h','flows_src_1h','agent_iface_1h')`).Scan(&caggs); err != nil || caggs != 7 {
		t.Fatalf("continuous aggregate sayisi: %d (err: %v)", caggs, err)
	}
	// S21.12: process_traffic_1h cagg + yenileme/retention job'ları
	var jobs int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM timescaledb_information.jobs
		WHERE hypertable_name = 'process_traffic_1h'`).Scan(&jobs); err != nil || jobs < 2 {
		t.Fatalf("process_traffic_1h job sayisi: %d (yenileme + retention beklenir; err: %v)", jobs, err)
	}
	// TopProcessTraffic uzun pencerede (>48s) cagg'den okumalı — ham
	// process_traffic 7g retention'da düşse de kapasite raporu doğru.
	old := time.Now().Add(-72 * time.Hour).Unix()
	if err := st.SaveProcessTraffic(7, old, []telemetry.ProcessTrafficSample{
		{PID: 1, Process: "backup", Proto: "tcp", RemoteIP: "9.9.9.9", Port: 443, BytesIn: 100, BytesOut: 900_000},
	}); err != nil {
		t.Fatalf("eski process_traffic: %v", err)
	}
	// cagg'i manuel yenile (job zamanlamasını beklemeden)
	if _, err := s.db.Exec(`CALL refresh_continuous_aggregate('process_traffic_1h', NULL, NULL)`); err != nil {
		t.Fatalf("refresh process_traffic_1h: %v", err)
	}
	tp, err := st.TopProcessTraffic(time.Now().Add(-96*time.Hour), 0, 10, "")
	if err != nil {
		t.Fatalf("TopProcessTraffic (uzun pencere): %v", err)
	}
	var backup *ProcessTrafficUsage
	for i := range tp {
		if tp[i].Process == "backup" {
			backup = &tp[i]
		}
	}
	if backup == nil || backup.BytesOut != 900_000 {
		t.Fatalf("cagg'den backup süreci okunamadı: %+v", tp)
	}

	// flows_1h real-time cagg: FleetProtocolTotals ham `flows` yerine buradan
	// okumalı (retention'dan uzun raporlar için).
	if err := st.SaveFlows([]FlowRow{
		{Ts: time.Now().Unix() - 30, Device: "rt", Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Octets: 5000, Packets: 5},
		{Ts: time.Now().Unix() - 20, Device: "rt", Src: "10.0.0.1", Dst: "1.1.1.1", Proto: "tcp", Octets: 3000, Packets: 3},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}
	pt, err := st.FleetProtocolTotals(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("FleetProtocolTotals: %v", err)
	}
	if pt["UDP"] != 5000 || pt["TCP"] != 3000 {
		t.Fatalf("flows_1h protokol toplamı hatalı: %+v", pt)
	}

	// real-time cagg: ham veri aninda gorunmeli
	now := time.Now().Unix()
	for i := int64(0); i < 10; i++ {
		if err := st.InsertSample(Sample{Ts: now - 10 + i, Device: "en0", BpsIn: 8000, BpsOut: 2000, Pps: 10}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}
	buckets, err := st.TimeseriesBuckets(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("cagg sorgu: %v", err)
	}
	if len(buckets) < 1 {
		t.Fatal("gercek-zamanli cagg veri dondurmedi")
	}
	if got := buckets[0].In * 8; got < 7900 || got > 8100 {
		t.Fatalf("cagg bps_in hatali: %v", got)
	}
}

// TestPostgresConcurrentMigration, docker-compose.scale.yml topolojisinin
// (hub-controller + N x hub-ingest) TAZE bir veritabanina karsi neredeyse
// es zamanli basladigi gercek senaryoyu yeniden uretir. Kilit olmadan bu,
// es zamanli "CREATE TABLE IF NOT EXISTS" DDL'leri arasinda pg_type
// katalog kaydi icin bir yaris durumuna dusup "duplicate key value
// violates unique constraint pg_type_typname_nsp_index" hatasi veriyordu
// — CI'daki scale-smoke-test job'unda (deploy/docker-compose.scale.yml,
// 3 gercek hub instance'i) canli yakalandi. migratePostgres'teki advisory
// lock (bkz. pg.go) bunu N goroutine'in TEK bir TAZE container'a karsi
// Open() cagirmasiyla dogrudan test eder — hepsi hatasiz donmeli.
func TestPostgresConcurrentMigration(t *testing.T) {
	dsn := pgContainerDSN(t, "postgres:16-alpine")

	const n = 5
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st, err := Open(dsn)
			if err != nil {
				errs[i] = err
				return
			}
			defer st.Close()
			errs[i] = st.Ping()
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: es zamanli Open() basarisiz oldu: %v", i, err)
		}
	}
}

// TestPostgresAuditChainConcurrent, deploy/docker-compose.scale.yml'deki 2×
// hub-controller senaryosunu yeniden uretir: AYRI iki Store instance'i (ayri
// baglanti havuzlari = ayri process'ler gibi; her birinin kendi process-ici
// auditMu'su var) ayni anda denetim olayi yazar. Advisory lock olmadan ikisi
// de ayni son satiri prev olarak okuyup ayni prev_hash ile INSERT eder →
// zincir catallanir (canli scale DB'sinde 28 catal noktasi gozlendi). Kilitle
// zincir tek-yonlu kalir; VerifyAuditChain saglam donmeli.
func TestPostgresAuditChainConcurrent(t *testing.T) {
	dsn := pgContainerDSN(t, "postgres:16-alpine")

	stA, err := Open(dsn)
	if err != nil {
		t.Fatalf("controller A: %v", err)
	}
	defer stA.Close()
	stB, err := Open(dsn)
	if err != nil {
		t.Fatalf("controller B: %v", err)
	}
	defer stB.Close()

	const each = 60
	var wg sync.WaitGroup
	errCh := make(chan error, 2*each)
	write := func(st Store, action string) {
		defer wg.Done()
		for i := 0; i < each; i++ {
			if _, err := st.InsertAuditEvent(AuditEvent{
				Username: "admin", Role: "admin", Action: action,
				ActorType: "user", Result: "ok",
			}); err != nil {
				errCh <- err
				return
			}
		}
	}
	wg.Add(2)
	go write(stA, "login")
	go write(stB, "login")
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("eszamanli ekleme: %v", err)
	}

	// catal noktasi olmamali
	var forks int
	if err := stA.(*sqlStore).db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT prev_hash FROM audit_events GROUP BY prev_hash HAVING COUNT(*) > 1
		) f`).Scan(&forks); err != nil {
		t.Fatalf("catal sorgu: %v", err)
	}
	if forks != 0 {
		t.Fatalf("%d catal noktasi (prev_hash birden cok kayitta)", forks)
	}

	ok, brokenAt, checked, err := stA.VerifyAuditChain()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatalf("zincir catallandi (kayit #%d)", brokenAt)
	}
	if checked != 2*each {
		t.Fatalf("%d kayit beklenirdi, dogrulanan: %d", 2*each, checked)
	}
}

// TestPostgresComplianceChainConcurrent, TestPostgresAuditChainConcurrent'in
// 5651 compliance_logs karsiligi: AYRI iki Store instance'i (2× hub-controller)
// ayni anda AppendComplianceLog cagirir. complianceChainLockKey advisory lock'u
// olmadan zincir catallanir; kilitle tek-yonlu kalir.
func TestPostgresComplianceChainConcurrent(t *testing.T) {
	dsn := pgContainerDSN(t, "postgres:16-alpine")

	stA, err := Open(dsn)
	if err != nil {
		t.Fatalf("controller A: %v", err)
	}
	defer stA.Close()
	stB, err := Open(dsn)
	if err != nil {
		t.Fatalf("controller B: %v", err)
	}
	defer stB.Close()

	base := time.Now().Unix()
	const each = 60
	var wg sync.WaitGroup
	errCh := make(chan error, 2*each)
	write := func(st Store, name string) {
		defer wg.Done()
		for i := 0; i < each; i++ {
			if _, err := st.AppendComplianceLog(ComplianceLog{
				Ts: base + int64(i), SourceType: "syslog", SourceName: name,
				Category: "syslog", Message: "eszamanli test",
			}); err != nil {
				errCh <- err
				return
			}
		}
	}
	wg.Add(2)
	go write(stA, "swA")
	go write(stB, "swB")
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("eszamanli ekleme: %v", err)
	}

	var forks int
	if err := stA.(*sqlStore).db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT prev_hash FROM compliance_logs GROUP BY prev_hash HAVING COUNT(*) > 1
		) f`).Scan(&forks); err != nil {
		t.Fatalf("catal sorgu: %v", err)
	}
	if forks != 0 {
		t.Fatalf("%d catal noktasi (prev_hash birden cok kayitta)", forks)
	}

	broken, checked := walkComplianceChain(t, stA, base-10, base+3600)
	if broken != 0 {
		t.Fatalf("zincir catallandi (seq %d)", broken)
	}
	if checked != 2*each {
		t.Fatalf("%d kayit beklenirdi, dogrulanan: %d", 2*each, checked)
	}
}
