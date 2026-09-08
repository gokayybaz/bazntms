package metrics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestObserveStoreWrite(t *testing.T) {
	before := testutil.ToFloat64(storeWriteRows.WithLabelValues("agent_iface_samples"))
	ObserveStoreWrite("agent_iface_samples", 7, time.Now().Add(-3*time.Millisecond))
	after := testutil.ToFloat64(storeWriteRows.WithLabelValues("agent_iface_samples"))
	if after-before != 7 {
		t.Fatalf("store_write_rows_total delta = %v, beklenen 7", after-before)
	}
	if got := testutil.CollectAndCount(storeWriteDur); got == 0 {
		t.Fatal("store_write_duration_seconds hiç örnek toplamadı")
	}
}

func TestFlowsCounters(t *testing.T) {
	b := testutil.ToFloat64(flowsReceived.WithLabelValues("v9"))
	AddFlowsReceived("v9", 12)
	AddFlowsReceived("v9", 0) // 0 sayılmamalı
	if got := testutil.ToFloat64(flowsReceived.WithLabelValues("v9")) - b; got != 12 {
		t.Fatalf("flows_received_total delta = %v, beklenen 12", got)
	}
	d := testutil.ToFloat64(flowsDropped.WithLabelValues("unknown_version"))
	IncFlowsDropped("unknown_version")
	if got := testutil.ToFloat64(flowsDropped.WithLabelValues("unknown_version")) - d; got != 1 {
		t.Fatalf("flows_dropped_total delta = %v, beklenen 1", got)
	}
}

func TestDevpollInflight(t *testing.T) {
	b := testutil.ToFloat64(devpollInflight)
	AddDevpollInflight(3)
	if got := testutil.ToFloat64(devpollInflight); got != b+3 {
		t.Fatalf("devpoll_inflight = %v, beklenen %v", got, b+3)
	}
	AddDevpollInflight(-3)
}

func TestDBPoolCollectors(t *testing.T) {
	fake := sql.DBStats{
		OpenConnections: 5, InUse: 2, Idle: 3, MaxOpenConnections: 32,
		WaitCount: 9, WaitDuration: 250 * time.Millisecond,
	}
	cs := dbPoolCollectors(func() sql.DBStats { return fake })
	want := []float64{5, 2, 3, 32, 9, 0.25} // open, in_use, idle, max, wait_count, wait_seconds
	if len(cs) != len(want) {
		t.Fatalf("collector sayısı = %d, beklenen %d", len(cs), len(want))
	}
	for i, c := range cs {
		if got := testutil.ToFloat64(c); got != want[i] {
			t.Errorf("dbPoolCollectors[%d] = %v, beklenen %v", i, got, want[i])
		}
	}
}

func TestRegisterDBPoolIdempotent(t *testing.T) {
	// çift çağrı panik/çift-kayıt etmemeli
	RegisterDBPool(func() sql.DBStats { return sql.DBStats{InUse: 1} })
	RegisterDBPool(func() sql.DBStats { return sql.DBStats{InUse: 2} })
	if got := testutil.CollectAndCount(reg, "bazntms_db_pool_in_use"); got != 1 {
		t.Fatalf("bazntms_db_pool_in_use seri sayısı = %d, beklenen 1", got)
	}
}

func TestRegistryHasIngestFamilies(t *testing.T) {
	// vec metrikleri en az bir gözlem olmadan toplama çıktısında görünmez —
	// hepsine birer örnek ver.
	ObserveStoreWrite("t", 1, time.Now())
	ObserveTelemetryDecode(time.Now())
	ObserveQueueBatch(1)
	AddFlowsReceived("v5", 1)
	IncFlowsDropped("short")
	ObserveDevpollCycle(time.Now())
	SetQueuePending(0)

	for _, name := range []string{
		"bazntms_queue_pending",
		"bazntms_queue_batch_size",
		"bazntms_store_write_duration_seconds",
		"bazntms_store_write_rows_total",
		"bazntms_telemetry_decode_duration_seconds",
		"bazntms_flows_received_total",
		"bazntms_flows_dropped_total",
		"bazntms_devpoll_cycle_duration_seconds",
		"bazntms_devpoll_inflight",
	} {
		if got := testutil.CollectAndCount(reg, name); got == 0 {
			t.Errorf("registry'de metrik ailesi eksik: %s", name)
		}
	}
}
