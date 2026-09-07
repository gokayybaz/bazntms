//go:build linux

// Package bpf'in Linux tarafı: gömülü CO-RE nesnesini kernele yükler, fentry
// programlarını bağlar ve akış haritasını (süreç × uzak uç → bayt) boşaltarak
// okur. Generated bpf2go kodu (attrprog_bpfel.go) bu paket içinde kapsüllenir;
// dışarıya yalnızca Engine + Flow verilir.
package bpf

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// Flow, akış haritasından okunan bir (süreç × uzak uç) kaydının o okuma
// dönemindeki bayt farkıdır — harita her okumada LookupAndDelete ile boşaltılır,
// dolayısıyla her Flow doğrudan bir deltadır.
type Flow struct {
	PID      uint32
	Proto    uint8 // 6 = TCP, 17 = UDP
	RemoteIP netip.Addr
	Port     uint16 // host bayt sırası
	Process  string // kernel comm (best-effort, 15 karakter sınırı)
	BytesOut uint64
	BytesIn  uint64
}

// Engine, yüklenmiş eBPF programlarını, bağlantılarını ve akış haritasını tutar.
type Engine struct {
	objs  attrprogObjects
	links []link.Link
}

// fentry hedefleri — kernelde bulunmayan / bağlanamayan bir program atlanır
// (o telemetri yolu eksilir ama motor çalışır); hiçbiri bağlanamazsa hata.
type fentryTarget struct {
	name string
	prog func(*attrprogPrograms) *ebpf.Program
}

var fentryTargets = []fentryTarget{
	{"tcp_sendmsg", func(p *attrprogPrograms) *ebpf.Program { return p.TcpSendmsg }},
	{"tcp_cleanup_rbuf", func(p *attrprogPrograms) *ebpf.Program { return p.TcpCleanupRbuf }},
	{"udp_sendmsg", func(p *attrprogPrograms) *ebpf.Program { return p.UdpSendmsg }},
	{"skb_consume_udp", func(p *attrprogPrograms) *ebpf.Program { return p.SkbConsumeUdp }},
}

// Load, gömülü nesneyi yükler ve fentry programlarını bağlar. Nesne
// yüklenemezse (kernel < 5.8 / BTF yok / verifier reddi) hata döner — çağıran
// taraf bunu ölümcül saymaz, pcap'e düşer.
func Load() (*Engine, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("memlock rlimit: %w", err)
	}

	e := &Engine{}
	if err := loadAttrprogObjects(&e.objs, nil); err != nil {
		return nil, fmt.Errorf("eBPF nesnesi yüklenemedi: %w", err)
	}

	for _, t := range fentryTargets {
		l, err := link.AttachTracing(link.TracingOptions{Program: t.prog(&e.objs.attrprogPrograms)})
		if err != nil {
			slog.Warn("eBPF fentry bağlanamadı — bu yol atlanıyor", "program", t.name, "err", err)
			continue
		}
		e.links = append(e.links, l)
	}
	if len(e.links) == 0 {
		_ = e.objs.Close()
		return nil, errors.New("hiçbir eBPF fentry programı bağlanamadı")
	}
	slog.Debug("eBPF atıf motoru yüklendi", "bagli_program", len(e.links))
	return e, nil
}

// Flows, akış haritasını boşaltarak son okumadan bu yana biriken kayıtları
// döndürür. İki geçiş: önce anahtarlar toplanır (iterasyon sırasında silme
// kernel sürümüne göre güvensiz), sonra her anahtar atomik LookupAndDelete ile
// alınır.
func (e *Engine) Flows() ([]Flow, error) {
	m := e.objs.Flows
	var (
		key  attrprogFlowKey
		val  attrprogFlowStat
		keys []attrprogFlowKey
	)
	it := m.Iterate()
	for it.Next(&key, &val) {
		keys = append(keys, key)
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("akış haritası iterasyonu: %w", err)
	}

	out := make([]Flow, 0, len(keys))
	for i := range keys {
		if err := m.LookupAndDelete(&keys[i], &val); err != nil {
			if errors.Is(err, ebpf.ErrKeyNotExist) {
				continue
			}
			return nil, fmt.Errorf("akış haritası oku+sil: %w", err)
		}
		if val.BytesOut == 0 && val.BytesIn == 0 {
			continue
		}
		out = append(out, Flow{
			PID:      keys[i].Pid,
			Proto:    keys[i].Proto,
			RemoteIP: flowAddr(keys[i].Family, keys[i].Daddr),
			Port:     ntohs(keys[i].Dport),
			Process:  commString(val.Comm),
			BytesOut: val.BytesOut,
			BytesIn:  val.BytesIn,
		})
	}
	return out, nil
}

// Close, bağlantıları ve yüklü nesneleri serbest bırakır.
func (e *Engine) Close() error {
	for _, l := range e.links {
		_ = l.Close()
	}
	return e.objs.Close()
}

func ntohs(x uint16) uint16 { return x<<8 | x>>8 }

func flowAddr(family uint8, d [16]uint8) netip.Addr {
	if family == 2 { // AF_INET — daddr'ın ilk 4 baytı ağ sırasında
		return netip.AddrFrom4([4]byte{d[0], d[1], d[2], d[3]})
	}
	return netip.AddrFrom16(d) // AF_INET6
}

func commString(b [16]uint8) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b[:])
}
