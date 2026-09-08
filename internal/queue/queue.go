// Package queue, Faz 4.2 olcek altyapisinin kuyrugudur: ingest → processor
// ayrismasi. HTTP istegi telemetriyi JetStream'e yayinlar ve hemen yanit
// doner; bagimsiz processor'lar mesajlari store'a yazar.
//
// Kazanimlar:
//   - ingest patlamalarinda yazi tarafinin esnekligi (burst: 200K flow/sn)
//   - kayip toleransi: store yazimi hata verirse mesaj Nak edilir ve
//     yeniden islenir (replay); JetStream mesaji diske yazar
//   - birden fazla hub replikasi ayni kuyrugu paylasabilir (LB arkasi)
//
// Kuyruk istenmediyse (-nats bos) hub dogrudan store'a yazmaya devam eder.
package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

const (
	streamName   = "BAZNTMS"
	subTelemetry = "ingest.telemetry"
	subFlows     = "ingest.flows"
	subSyslog    = "ingest.syslog"
	subDead      = "ingest.dead" // DLQ (C4): MaxDeliver'a ulaşan mesajlar
	consumerName = "store-writer"

	maxDeliver     = 10
	defaultMaxAge  = 24 * time.Hour
	deadHeaderSubj = "Bazntms-Orig-Subject"
	deadHeaderErr  = "Bazntms-Error"
)

// Envelope, kuyruktaki telemetri mesajinin icerigidir: agent kimligi + batch.
type Envelope struct {
	AgentID  int64                    `json:"agent_id"`
	Version  string                   `json:"version"`
	RemoteIP string                   `json:"remote_ip"`
	TS       int64                    `json:"ts"`
	Batch    telemetry.TelemetryBatch `json:"batch"`
}

// Queue, JetStream yayincisi + store-writer processor'u.
type Queue struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	cancel   context.CancelFunc
	done     chan struct{}
	deadHook func(subject string) // C4: DLQ'ya düşen mesaj için (metrik); nil olabilir
}

// SetDeadLetterHook, bir mesaj MaxDeliver denemeden sonra DLQ'ya taşındığında
// çağrılır (subject = orijinal ingest.* konusu). Metrik/uyarı için.
func (q *Queue) SetDeadLetterHook(fn func(subject string)) { q.deadHook = fn }

// Connect, NATS sunucusuna baglanir ve BAZNTMS akisini hazirlar. maxAge <= 0 ise
// varsayilan 24 saat (tuketilmese bile birikimin siniri).
func Connect(url string, maxAge time.Duration) (*Queue, error) {
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	nc, err := nats.Connect(url,
		nats.Name("bazntms-hub"),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1), // hub omru boyunca yeniden baglanir
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{subTelemetry, subFlows, subSyslog, subDead},
		Storage:  jetstream.FileStorage,
		MaxMsgs:  5_000_000,
		MaxAge:   maxAge, // tuketilmese bile birikimin siniri
	})
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Queue{nc: nc, js: js}, nil
}

// PublishTelemetry, telemetri batch'ini kuyruga aktarir. Hub, agent'a
// cevabi yayin basarisi alir almaz doner (yazi processor tarafinda).
func (q *Queue) PublishTelemetry(agentID int64, version, remoteIP string, ts int64, batch *telemetry.TelemetryBatch) error {
	data, err := json.Marshal(Envelope{
		AgentID: agentID, Version: version, RemoteIP: remoteIP, TS: ts, Batch: *batch,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = q.js.Publish(ctx, subTelemetry, data)
	return err
}

// PublishFlows, NetFlow satirlarini toplu olarak kuyruga aktarir.
func (q *Queue) PublishFlows(rows []store.FlowRow) error {
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = q.js.Publish(ctx, subFlows, data)
	return err
}

// PublishSyslog, tek cihaz olayini kuyruga aktarir.
func (q *Queue) PublishSyslog(ev store.SyslogEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = q.js.Publish(ctx, subSyslog, data)
	return err
}

// RunProcessor, durable "store-writer" tuketicisini baslatir: mesajlari
// ceker ve store'a yazar. Hata durumunda mesaj gecikmeli Nak edilir
// (replay); MaxDeliver sonunda atilir (kayip toleransi siniri).
func (q *Queue) RunProcessor(ctx context.Context, st store.Store, workers int) error {
	if workers <= 0 {
		workers = 4
	}
	cons, err := q.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable: consumerName,
		// yalnizca gercek ingest konulari; ingest.dead DLQ'dur, tuketilmez.
		FilterSubjects: []string{subTelemetry, subFlows, subSyslog},
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        30 * time.Second,
		MaxDeliver:     maxDeliver,
		MaxAckPending:  4096,
	})
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	q.cancel = cancel
	q.done = make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.workerLoop(runCtx, cons, st)
		}()
	}
	go q.pendingLoop(runCtx, cons)
	go func() {
		wg.Wait()
		close(q.done)
	}()
	slog.Info("kuyruk processor basladi", "stream", streamName, "workers", workers)
	return nil
}

// pendingLoop, tüketicinin bekleyen mesaj sayısını periyodik olarak
// bazntms_queue_pending metriğine yazar — ingest lag göstergesi.
func (q *Queue) pendingLoop(ctx context.Context, cons jetstream.Consumer) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ictx, cancel := context.WithTimeout(ctx, 3*time.Second)
			info, err := cons.Info(ictx)
			cancel()
			if err == nil {
				metrics.SetQueuePending(float64(info.NumPending) + float64(info.NumAckPending))
			}
		}
	}
}

func (q *Queue) workerLoop(ctx context.Context, cons jetstream.Consumer, st store.Store) {
	for ctx.Err() == nil {
		batch, err := cons.Fetch(128, jetstream.FetchMaxWait(time.Second))
		if err != nil {
			continue // bos cekim veya gecici hata
		}
		n := 0
		for msg := range batch.Messages() {
			if ctx.Err() != nil {
				return
			}
			q.handle(msg, st)
			n++
		}
		if n > 0 {
			metrics.ObserveQueueBatch(n)
		}
	}
}

func (q *Queue) handle(msg jetstream.Msg, st store.Store) {
	switch msg.Subject() {
	case subTelemetry:
		var env Envelope
		dec := time.Now()
		if err := json.Unmarshal(msg.Data(), &env); err != nil {
			slog.Error("kuyruk: telemetri cozumlenemedi", "err", err)
			termMsg(msg)
			return
		}
		metrics.ObserveTelemetryDecode(dec)
		ts := env.TS
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if err := st.SaveIfaceSamples(env.AgentID, ts, env.Batch.Interfaces); err != nil {
			q.retry(msg, err)
			return
		}
		if err := st.ReplaceConnLatest(env.AgentID, env.Batch.Connections); err != nil {
			q.retry(msg, err)
			return
		}
		if err := st.TouchAgent(env.AgentID, env.Version, env.Batch.ProtocolVersion, env.RemoteIP); err != nil {
			slog.Error("kuyruk: agent touch hatasi", "agent_id", env.AgentID, "err", err)
		}
		if env.Batch.AttrMethod != "" {
			if err := st.SetAgentAttrMethod(env.AgentID, env.Batch.AttrMethod); err != nil {
				slog.Error("kuyruk: attr_method kaydi hatasi", "agent_id", env.AgentID, "err", err)
			}
		}
		if len(env.Batch.ProcessTraffic) > 0 {
			if err := st.SaveProcessTraffic(env.AgentID, ts, env.Batch.ProcessTraffic); err != nil {
				q.retry(msg, err)
				return
			}
		}
		if len(env.Batch.L7) > 0 {
			if err := st.SaveL7(env.AgentID, ts, env.Batch.L7); err != nil {
				q.retry(msg, err)
				return
			}
		}
		if len(env.Batch.DNS) > 0 {
			if err := st.SaveAgentDNS(env.AgentID, ts, env.Batch.DNS); err != nil {
				q.retry(msg, err)
				return
			}
		}
		if len(env.Batch.Subnets) > 0 {
			// topoloji kesfi (Faz 6.1): agent'in yerel aglari
			if err := st.SaveAgentSubnets(env.AgentID, "", env.Batch.Subnets); err != nil {
				q.retry(msg, err)
				return
			}
		}
	case subFlows:
		var rows []store.FlowRow
		if err := json.Unmarshal(msg.Data(), &rows); err != nil {
			slog.Error("kuyruk: flow cozumlenemedi", "err", err)
			termMsg(msg)
			return
		}
		if err := st.SaveFlows(rows); err != nil {
			q.retry(msg, err)
			return
		}
	case subSyslog:
		var ev store.SyslogEvent
		if err := json.Unmarshal(msg.Data(), &ev); err != nil {
			slog.Error("kuyruk: syslog cozumlenemedi", "err", err)
			termMsg(msg)
			return
		}
		if err := st.SaveSyslogEvent(ev); err != nil {
			q.retry(msg, err)
			return
		}
		// 5651 uyum zinciri: alıcı tarafında SENKRON eklenir (main.go OnEvent) —
		// kuyruk yolu tekrarlamaz, aksi halde çift kayıt olurdu (S15.7).
	default:
		termMsg(msg)
		return
	}
	ackMsg(msg)
}

func (q *Queue) retry(msg jetstream.Msg, cause error) {
	// C4: son denemede (MaxDeliver'a ulasmadan) mesaji DLQ'ya tasi — aksi halde
	// JetStream sessizce atardi ve kayip yalnizca loglardan anlasilirdi.
	if md, err := msg.Metadata(); err == nil && md.NumDelivered >= maxDeliver-1 {
		q.deadLetter(msg, cause)
		return
	}
	slog.Error("kuyruk: yazim hatasi, mesaj yeniden kuyruga alindi", "err", cause)
	if err := msg.NakWithDelay(2 * time.Second); err != nil {
		slog.Warn("kuyruk: NakWithDelay basarisiz (ack-wait ile yine de teslim edilir)", "err", err)
	}
}

// deadLetter, mesaji ingest.dead konusuna kopyalar, orijinali Term eder ve
// deadHook'u cagirir (metrik). DLQ tuketilmez — nats stream'de incelenir.
func (q *Queue) deadLetter(msg jetstream.Msg, cause error) {
	orig := msg.Subject()
	slog.Error("kuyruk: mesaj DLQ'ya tasindi (MaxDeliver asildi)", "subject", orig, "err", cause)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hdr := nats.Header{}
	hdr.Set(deadHeaderSubj, orig)
	hdr.Set(deadHeaderErr, cause.Error())
	if _, err := q.js.PublishMsg(ctx, &nats.Msg{Subject: subDead, Data: msg.Data(), Header: hdr}); err != nil {
		slog.Error("kuyruk: DLQ yayini basarisiz — mesaj Nak ediliyor", "err", err)
		_ = msg.NakWithDelay(5 * time.Second)
		return
	}
	termMsg(msg)
	if q.deadHook != nil {
		q.deadHook(orig)
	}
}

// termMsg/ackMsg: JetStream ack sinyali hatasi kritik degil — mesaj
// ack-wait sonrasi yeniden teslim edilir; yine de loglanir.
func termMsg(msg jetstream.Msg) {
	if err := msg.Term(); err != nil {
		slog.Warn("kuyruk: msg.Term basarisiz", "err", err)
	}
}

func ackMsg(msg jetstream.Msg) {
	if err := msg.Ack(); err != nil {
		slog.Warn("kuyruk: msg.Ack basarisiz", "err", err)
	}
}

// Close, processor'u ve NATS baglantisini kapatir.
func (q *Queue) Close() {
	if q.cancel != nil {
		q.cancel()
		<-q.done
	}
	if q.nc != nil {
		q.nc.Close()
	}
}
