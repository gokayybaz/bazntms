//go:build !windows

package queue

// Faz 4.2: NATS JetStream kuyruk entegrasyon testi (testcontainers).
// publish → processor → store yazimi hattinin uc uca calismasini dogrular.
// Docker yoksa skip edilir; CI ubuntu runner'inda calisir.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func natsQueueFor(t *testing.T) (*Queue, store.Store) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mod")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2-alpine",
			Cmd:          []string{"-js"},
			ExposedPorts: []string{"4222/tcp"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("docker kullanilamadi, atlanıyor: %v", err)
	}
	t.Cleanup(func() { ctr.Terminate(context.Background()) })

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}

	q, err := Connect("nats://" + host + ":" + port.Port())
	if err != nil {
		t.Fatalf("nats baglantisi: %v", err)
	}
	t.Cleanup(q.Close)

	st, err := store.Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	if err := q.RunProcessor(context.Background(), st, 2); err != nil {
		t.Fatalf("processor: %v", err)
	}
	return q, st
}

// waitFor, kosul saglanana kadar yoklar (asenkron processor icin).
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestQueueRoundTrip(t *testing.T) {
	q, st := natsQueueFor(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(store.Agent{Name: "q-agent", TokenHash: TokenHashForTest(), ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}

	// telemetri
	if err := q.PublishTelemetry(id, "0.1", "10.0.0.5", now, &telemetry.TelemetryBatch{
		TS: now,
		Interfaces: []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: 9000, TxBytes: 4000, RxPackets: 90, TxPackets: 40},
		},
		ProcessTraffic: []telemetry.ProcessTrafficSample{
			{PID: 7, Process: "nginx", Proto: "tcp", RemoteIP: "2.2.2.2", Port: 443, BytesIn: 500, BytesOut: 250},
		},
	}); err != nil {
		t.Fatalf("telemetri yayini: %v", err)
	}

	// flow + syslog
	if err := q.PublishFlows([]store.FlowRow{{Ts: now, Device: "fw-1", Src: "1.1.1.1", Dst: "2.2.2.2", DstPort: 443, Proto: "tcp", Octets: 1234}}); err != nil {
		t.Fatalf("flow yayini: %v", err)
	}
	if err := q.PublishSyslog(store.SyslogEvent{Ts: now, Host: "fw-1", Severity: 3, Tag: "SEC", Message: "deny"}); err != nil {
		t.Fatalf("syslog yayini: %v", err)
	}

	waitFor(t, func() bool {
		agents, err := st.ListAgents(time.Hour, "")
		return err == nil && len(agents) == 1 && agents[0].Online
	}, "telemetri store'a yazilmadi")

	waitFor(t, func() bool {
		pt, err := st.TopProcessTraffic(time.Now().Add(-time.Hour), 0, 10)
		return err == nil && len(pt) == 1 && pt[0].Total == 750
	}, "surec trafigi store'a yazilmadi")

	waitFor(t, func() bool {
		fl, err := st.TopFlows(time.Now().Add(-time.Hour), 10, "")
		return err == nil && len(fl) == 1 && fl[0].Octets == 1234
	}, "flow store'a yazilmadi")

	waitFor(t, func() bool {
		sy, err := st.RecentSyslog(10, "")
		return err == nil && len(sy) == 1 && sy[0].Message == "deny"
	}, "syslog store'a yazilmadi")
}

// TokenHashForTest, test agent'ina ozel token hash'i.
func TokenHashForTest() string {
	return store.TokenHash("queue-test-token")
}
