//go:build linux

package agent

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/agent/bpf"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// ebpfDrainInterval, akış haritasının ne sıklıkla boşaltılacağı. LRU_HASH
// dolarsa en eski kayıtları düşürür — yeterince sık okumak eviction kaybını
// önler. Telemetri aralığından (10-60 sn) bağımsızdır; Deltas() yalnızca
// biriktiriciyi hasat eder.
const ebpfDrainInterval = 2500 * time.Millisecond

// ebpfAttrSource, AttrSource'un Linux eBPF implementasyonu (S20.5). Bayt
// sayımı çekirdek fentry programlarından gelir; L7 (S20.7) ve DNS (S20.6)
// henüz boş döner — pcap arka ucundan farkı budur (bilinçli degrade).
type ebpfAttrSource struct {
	eng *bpf.Engine

	mu      sync.Mutex
	pending map[attrKey]*[2]uint64 // [in, out] — Deltas'a dek birikir

	stopCh chan struct{}
	doneCh chan struct{}
}

var _ AttrSource = (*ebpfAttrSource)(nil)

func newEbpfAttrSource(_ AttrConfig) (*ebpfAttrSource, error) {
	eng, err := bpf.Load()
	if err != nil {
		return nil, err
	}
	e := &ebpfAttrSource{
		eng:     eng,
		pending: map[attrKey]*[2]uint64{},
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	go e.drainLoop()
	return e, nil
}

func (e *ebpfAttrSource) drainLoop() {
	defer close(e.doneCh)
	t := time.NewTicker(ebpfDrainInterval)
	defer t.Stop()
	for {
		select {
		case <-e.stopCh:
			e.drainOnce() // kapanışta son tur
			return
		case <-t.C:
			e.drainOnce()
		}
	}
}

func (e *ebpfAttrSource) drainOnce() {
	flows, err := e.eng.Flows()
	if err != nil {
		slog.Warn("eBPF akış haritası okunamadı", "err", err)
		return
	}
	if len(flows) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range flows {
		proto := "tcp"
		if f.Proto == 17 {
			proto = "udp"
		}
		k := attrKey{
			pid:      int32(f.PID),
			process:  f.Process,
			proto:    proto,
			remoteIP: f.RemoteIP.String(),
			port:     f.Port,
		}
		acc := e.pending[k]
		if acc == nil {
			if len(e.pending) >= 4000 {
				continue
			}
			acc = &[2]uint64{}
			e.pending[k] = acc
		}
		acc[0] += f.BytesIn
		acc[1] += f.BytesOut
	}
}

// Deltas, son çağrıdan bu yana biriken süreç bazlı trafik farklarını döndürür.
func (e *ebpfAttrSource) Deltas() []telemetry.ProcessTrafficSample {
	e.mu.Lock()
	pending := e.pending
	e.pending = map[attrKey]*[2]uint64{}
	e.mu.Unlock()

	out := make([]telemetry.ProcessTrafficSample, 0, len(pending))
	for k, acc := range pending {
		if acc[0]+acc[1] == 0 {
			continue
		}
		out = append(out, telemetry.ProcessTrafficSample{
			PID:      k.pid,
			Process:  k.process,
			Proto:    k.proto,
			RemoteIP: k.remoteIP,
			Port:     k.port,
			BytesIn:  acc[0],
			BytesOut: acc[1],
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

// L7Deltas — eBPF payload görmez; S20.7'de dar-filtreli yardımcı pcap handle.
func (e *ebpfAttrSource) L7Deltas() []telemetry.L7Sample { return nil }

// DNSDeltas — S20.6'da udp/53 payload'ı ringbuf ile eklenecek.
func (e *ebpfAttrSource) DNSDeltas() []telemetry.DNSSample { return nil }

func (e *ebpfAttrSource) Method() string { return "ebpf" }

func (e *ebpfAttrSource) Stop() {
	close(e.stopCh)
	<-e.doneCh
	if err := e.eng.Close(); err != nil {
		slog.Debug("eBPF motoru kapatılırken hata", "err", err)
	}
}
