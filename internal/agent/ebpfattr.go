//go:build linux

package agent

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/cilium/ebpf/ringbuf"
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
	l7  *l7Tracker // her zaman kurulu; l7h nil ise boş kalır
	l7h *l7Helper  // dar-filtreli yardımcı pcap handle (nil olabilir)

	mu      sync.Mutex
	pending map[attrKey]*[2]uint64 // [in, out] — Deltas'a dek birikir
	dns     map[dnsKey]*dnsAgg     // süreç × domain — DNSDeltas'a dek birikir
	dnsSent map[dnsKey][2]uint64

	stopCh    chan struct{}
	doneCh    chan struct{}
	dnsDoneCh chan struct{} // dnsLoop çıkışı (yalnız ringbuf varsa çalışır)
}

var _ AttrSource = (*ebpfAttrSource)(nil)

func newEbpfAttrSource(cfg AttrConfig) (*ebpfAttrSource, error) {
	eng, err := bpf.Load()
	if err != nil {
		return nil, err
	}
	e := &ebpfAttrSource{
		eng:       eng,
		l7:        newL7Tracker(),
		pending:   map[attrKey]*[2]uint64{},
		dns:       map[dnsKey]*dnsAgg{},
		dnsSent:   map[dnsKey][2]uint64{},
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		dnsDoneCh: make(chan struct{}),
	}

	// L7 (SNI/Host) eBPF'te görünmez — payload gerektirir. İstenirse dar
	// filtreli bir yardımcı pcap handle açılır (CAP_NET_RAW ister); açılamazsa
	// L7 sessiz boş kalır, sayım + DNS aksamaz.
	if h, herr := newL7Helper(cfg.Iface, e.l7); herr != nil {
		slog.Info("eBPF modunda L7 (SNI/Host) devre dışı — yardımcı pcap handle açılamadı",
			"err", herr, "cozum", "CAP_NET_RAW verin veya -collect-method=pcap kullanın")
	} else {
		e.l7h = h
		slog.Debug("eBPF modunda L7 yardımcı pcap handle aktif", "iface", cfg.Iface)
	}

	go e.drainLoop()
	if eng.DNSAvailable() {
		go e.dnsLoop()
	} else {
		close(e.dnsDoneCh)
	}
	return e, nil
}

// dnsLoop, DNS ringbuf'ını bloke ederek okur; her wire mesajını parseDNSNames
// ile çözüp süreç × domain sayaçlarına ekler. Engine.Close() ringbuf'ı
// kapatınca ErrClosed ile çıkar.
func (e *ebpfAttrSource) dnsLoop() {
	defer close(e.dnsDoneCh)
	for {
		ev, err := e.eng.ReadDNS()
		if err != nil {
			if !errors.Is(err, ringbuf.ErrClosed) {
				slog.Warn("eBPF DNS ringbuf okuma durdu", "err", err)
			}
			return
		}
		names, isResp := parseDNSNames(ev.Payload)
		if len(names) == 0 {
			continue
		}
		e.mu.Lock()
		for _, dom := range names {
			k := dnsKey{pid: int32(ev.PID), process: ev.Process, domain: dom}
			a := e.dns[k]
			if a == nil {
				if len(e.dns) >= 4000 {
					continue
				}
				a = &dnsAgg{}
				e.dns[k] = a
			}
			if isResp {
				a.responses++
			} else {
				a.queries++
			}
		}
		e.mu.Unlock()
	}
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

// L7Deltas, yardımcı pcap handle'ından çıkarılan SNI/Host farklarını döndürür
// (handle açılamadıysa boş).
func (e *ebpfAttrSource) L7Deltas() []telemetry.L7Sample { return e.l7.deltas() }

// DNSDeltas, son çağrıdan bu yana süreç bazlı DNS farklarını döndürür.
// eBPF yalnız yanıtları yakalar (bkz. attr.c) → sorgu/yanıt ayrımı yaklaşık,
// domain görünürlüğü tam.
func (e *ebpfAttrSource) DNSDeltas() []telemetry.DNSSample {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]telemetry.DNSSample, 0, len(e.dns))
	for k, a := range e.dns {
		q := delta(a.queries, e.dnsSent[k][0])
		r := delta(a.responses, e.dnsSent[k][1])
		if q+r == 0 {
			continue
		}
		e.dnsSent[k] = [2]uint64{a.queries, a.responses}
		out = append(out, telemetry.DNSSample{
			PID:       k.pid,
			Process:   k.process,
			Domain:    k.domain,
			Queries:   q,
			Responses: r,
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func (e *ebpfAttrSource) Method() string { return "ebpf" }

func (e *ebpfAttrSource) Stop() {
	close(e.stopCh)
	<-e.doneCh // drainLoop haritayı kullanır — önce onu bekle
	if e.l7h != nil {
		e.l7h.Stop()
	}
	if err := e.eng.Close(); err != nil {
		slog.Debug("eBPF motoru kapatılırken hata", "err", err)
	}
	<-e.dnsDoneCh // Close() ringbuf'ı kapattı → dnsLoop çıkar
}
