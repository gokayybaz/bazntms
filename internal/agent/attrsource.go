package agent

import "github.com/gokayybaz/bazntms/pkg/telemetry"

// AttrSource, sürece atıflı trafik + L7 + DNS delta akışlarını üreten atıf
// arka ucudur. Bugün tek implementasyon pcap tabanlı AttrEngine'dir; Faz 20 ile
// Linux eBPF ve Windows ETW arka uçları aynı arayüzü karşılar. Seçici
// (newAttrSource) platform + yapılandırma + çalışma-zamanı yeteneğine göre
// birini kurar ve eskisine zarif düşer.
//
// Üç delta metodunun imzası pcap motorununkiyle birebir aynıdır — çağıran
// tarafın (cmd/bazntms-agent) batch montajı arka uçtan bağımsızdır.
type AttrSource interface {
	// Deltas, son çağrıdan bu yana süreç bazlı trafik farklarını döndürür.
	Deltas() []telemetry.ProcessTrafficSample

	// L7Deltas, süreç bazlı uygulama görünürlüğü (TLS SNI / HTTP Host)
	// farklarını döndürür. Payload gerektirdiği için eBPF/ETW arka uçlarında
	// boş dönebilir (bilinçli degrade — bkz. docs/decisions/0007).
	L7Deltas() []telemetry.L7Sample

	// DNSDeltas, süreç bazlı DNS sorgu/yanıt farklarını döndürür.
	DNSDeltas() []telemetry.DNSSample

	// Stop, arka ucu kapatır. Bir kez çağrılır (idempotent değildir).
	Stop()

	// Method, aktif toplama yöntemini döndürür: "pcap" | "ebpf" | "etw".
	// Telemetri (Status.AttrMethod), log satırları ve UI rozeti için.
	Method() string
}

// derleme-zamanı kontrolü: pcap motoru AttrSource'u karşılar.
var _ AttrSource = (*AttrEngine)(nil)
