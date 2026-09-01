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
	"runtime"
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
	if runtime.GOOS == "windows" {
		t.Skip("windows CI'da docker yok")
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
	if err := st.TouchAgent(id, "0.2", "10.0.0.9"); err != nil {
		t.Fatalf("touch: %v", err)
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
	pt, err := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10)
	if err != nil || len(pt) != 1 || pt[0].Process != "curl" || pt[0].Total != 1000 {
		t.Fatalf("surec top: %v %+v", err, pt)
	}

	// cihaz + SNMP ornekleri
	devID, err := st.AddDevice(Device{Name: "rt-1", Host: "10.0.0.1", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60})
	if err != nil || devID == 0 {
		t.Fatalf("cihaz: %v %d", err, devID)
	}
	devices, err := st.ListDevices()
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
	fl, err := st.TopFlows(time.Now().Add(-time.Hour), 10)
	if err != nil || len(fl) != 1 || fl[0].Octets != 9000 {
		t.Fatalf("flow sorgu: %v %+v", err, fl)
	}
	if err := st.SaveSyslogEvent(SyslogEvent{Ts: now, Host: "rt-1", Severity: 4, Tag: "LINK", Message: "up/down"}); err != nil {
		t.Fatalf("syslog: %v", err)
	}
	sy, err := st.RecentSyslog(10)
	if err != nil || len(sy) != 1 || sy[0].Host != "rt-1" {
		t.Fatalf("syslog sorgu: %v %+v", err, sy)
	}

	// uyarilar + config
	if _, err := st.InsertAlertEvent(AlertEvent{Ts: now, Kind: "bw", Key: "en0", Message: "yuksek bant"}); err != nil {
		t.Fatalf("uyari: %v", err)
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
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM timescaledb_information.continuous_aggregates WHERE view_name IN ('samples_1m','samples_1h')`).Scan(&caggs); err != nil || caggs != 2 {
		t.Fatalf("continuous aggregate sayisi: %d (err: %v)", caggs, err)
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
