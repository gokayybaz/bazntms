// Package metrics, paketler arası kesişen ingest hattı metriklerini tek bir
// Prometheus kayıt defterinde toplar (S21.5). Amaç: ölçek koşularında
// (bkz. docs/perf/, loadtest/) darboğazın nerede olduğunu okunur kılmak —
// kuyruk birikimi mi, depo yazımı mı, DB havuzu mu, flow alımı mı, cihaz poll
// döngüsü mü.
//
// server paketi kendi HTTP-seviyesi metriklerini instance-başına bir registry'de
// tutar (testlerde New() defalarca çağrılır); bu paketin registry'si ise
// GLOBAL — süreç ömrü boyunca tek. server.handleMetrics ikisini
// prometheus.Gatherers ile birleştirip /metrics'te sunar.
//
// Metrik adları çakışmaz: buradakiler bazntms_queue_*, bazntms_store_write_*,
// bazntms_db_pool_*, bazntms_telemetry_decode_*, bazntms_flows_*,
// bazntms_devpoll_* öneklerini kullanır; server'ınkiler bazntms_http_*,
// bazntms_ws_*, bazntms_capture_*, bazntms_notify_*, bazntms_ingest_dead_*.
package metrics

import (
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var reg = prometheus.NewRegistry()

// Registry, birleştirilmiş /metrics çıktısı için server'a verilir.
func Registry() *prometheus.Registry { return reg }

var (
	queuePending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bazntms_queue_pending",
		Help: "JetStream store-writer tüketicisinde henüz teslim edilmemiş mesaj sayısı (ingest lag)",
	})
	queueBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "bazntms_queue_batch_size",
		Help:    "İşlemci Fetch() başına çekilen mesaj sayısı",
		Buckets: []float64{1, 2, 4, 8, 16, 32, 64, 128, 256},
	})
	storeWriteDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bazntms_store_write_duration_seconds",
		Help:    "Toplu depo yazımı süresi (tablo bazında)",
		Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"table"})
	storeWriteRows = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_store_write_rows_total",
		Help: "Toplu depo yazımına giden satır sayısı (tablo bazında)",
	}, []string{"table"})
	telemetryDecodeDur = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "bazntms_telemetry_decode_duration_seconds",
		Help:    "Agent telemetri batch/zarf JSON çözme süresi",
		Buckets: []float64{.0001, .00025, .0005, .001, .0025, .005, .01, .025, .05, .1},
	})
	flowsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_flows_received_total",
		Help: "Ayrıştırılan flow kaydı sayısı (datagram protokolü bazında)",
	}, []string{"version"})
	flowsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_flows_dropped_total",
		Help: "Uygulama katmanında düşürülen flow datagramı sayısı (sebep bazında; çekirdek soket taşması bu katmanda görünmez — bkz. S21.9)",
	}, []string{"reason"})
	devpollCycleDur = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "bazntms_devpoll_cycle_duration_seconds",
		Help:    "Cihaz poll döngüsü (pollAll) süresi — hedef: bütçenin %80'i altında",
		Buckets: []float64{.5, 1, 2, 5, 10, 20, 30, 45, 60, 90, 120, 180},
	})
	devpollInflight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bazntms_devpoll_inflight",
		Help: "Şu an devam eden eşzamanlı cihaz poll işlemi sayısı",
	})
)

func init() {
	reg.MustRegister(
		queuePending, queueBatchSize,
		storeWriteDur, storeWriteRows,
		telemetryDecodeDur,
		flowsReceived, flowsDropped,
		devpollCycleDur, devpollInflight,
	)
}

// ObserveStoreWrite, bir toplu yazım çağrısının süresini ve satır sayısını
// kaydeder. start çağrının başında time.Now() ile alınır.
func ObserveStoreWrite(table string, rows int, start time.Time) {
	storeWriteDur.WithLabelValues(table).Observe(time.Since(start).Seconds())
	storeWriteRows.WithLabelValues(table).Add(float64(rows))
}

// ObserveTelemetryDecode, bir telemetri batch/zarf JSON çözme süresini kaydeder.
func ObserveTelemetryDecode(start time.Time) {
	telemetryDecodeDur.Observe(time.Since(start).Seconds())
}

// SetQueuePending, tüketici bekleyen mesaj sayısını günceller (processor
// periyodik olarak cons.Info'dan çeker).
func SetQueuePending(n float64) { queuePending.Set(n) }

// ObserveQueueBatch, bir Fetch() çağrısında işlenen mesaj sayısını kaydeder.
func ObserveQueueBatch(n int) { queueBatchSize.Observe(float64(n)) }

// AddFlowsReceived, ayrıştırılan n flow kaydını protokol etiketiyle sayar
// (version ∈ v5|v9|ipfix|sflow).
func AddFlowsReceived(version string, n int) {
	if n > 0 {
		flowsReceived.WithLabelValues(version).Add(float64(n))
	}
}

// IncFlowsDropped, uygulama katmanında düşürülen bir datagramı sayar
// (reason ∈ unknown_version|empty_parse|short).
func IncFlowsDropped(reason string) { flowsDropped.WithLabelValues(reason).Inc() }

// ObserveDevpollCycle, bir pollAll döngüsünün süresini kaydeder.
func ObserveDevpollCycle(start time.Time) { devpollCycleDur.Observe(time.Since(start).Seconds()) }

// AddDevpollInflight, eşzamanlı poll sayacını delta kadar değiştirir
// (girişte +1, çıkışta -1).
func AddDevpollInflight(delta int) { devpollInflight.Add(float64(delta)) }

var dbPoolOnce sync.Once

// dbPoolCollectors, bir database/sql havuzunun 6 istatistiğini Prometheus
// collector'larına sarar. Her collector toplama anında stats() çağırır (lazy).
func dbPoolCollectors(stats func() sql.DBStats) []prometheus.Collector {
	return []prometheus.Collector{
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bazntms_db_pool_open_connections",
			Help: "Açık DB bağlantısı sayısı (kullanımda + boşta)",
		}, func() float64 { return float64(stats().OpenConnections) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bazntms_db_pool_in_use",
			Help: "Şu an kullanımda olan DB bağlantısı sayısı",
		}, func() float64 { return float64(stats().InUse) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bazntms_db_pool_idle",
			Help: "Havuzda boşta bekleyen DB bağlantısı sayısı",
		}, func() float64 { return float64(stats().Idle) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bazntms_db_pool_max_open_connections",
			Help: "Havuz üst sınırı (SetMaxOpenConns)",
		}, func() float64 { return float64(stats().MaxOpenConnections) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "bazntms_db_pool_wait_count_total",
			Help: "Boş bağlantı beklemek zorunda kalınan toplam kez sayısı",
		}, func() float64 { return float64(stats().WaitCount) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "bazntms_db_pool_wait_seconds_total",
			Help: "Boş bağlantı beklerken geçen toplam süre (saniye)",
		}, func() float64 { return stats().WaitDuration.Seconds() }),
	}
}

// RegisterDBPool, verilen database/sql havuzunun istatistiklerini
// bazntms_db_pool_* metrikleri olarak yayınlar. İlk çağrı kaydeder; sonraki
// çağrılar yok sayılır (süreçte tek DB havuzu var).
func RegisterDBPool(stats func() sql.DBStats) {
	dbPoolOnce.Do(func() {
		reg.MustRegister(dbPoolCollectors(stats)...)
	})
}
