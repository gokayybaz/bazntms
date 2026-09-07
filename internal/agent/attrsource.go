package agent

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// AttrSource, sürece atıflı trafik + L7 + DNS delta akışlarını üreten atıf
// arka ucudur. Bugün pcap tabanlı pcapAttrSource; Faz 20 ile Linux eBPF ve
// Windows ETW arka uçları aynı arayüzü karşılar. Seçici (NewAttrSource)
// platform + yapılandırma + çalışma-zamanı yeteneğine göre birini kurar ve
// eskisine zarif düşer.
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
var _ AttrSource = (*pcapAttrSource)(nil)

// AttrConfig, atıf arka ucu seçicisine verilen parametrelerdir.
type AttrConfig struct {
	// Method: "" veya "auto" → platform tercih sırası; "ebpf"/"pcap"/"etw" →
	// yalnız o arka uç (kurulamıyorsa hata, düşme yok); "off" → seçici çağrılmaz.
	Method string
	// Iface, pcap arka ucu için çözümlenmiş yakalama arayüzü/cihaz adı.
	// eBPF/ETW bunu kullanmaz.
	Iface string
}

// attrCaps, çalışılan platformda hangi atıf arka uçlarının kurulabileceğini
// bildirir. Platform dosyaları (attrsource_{linux,windows,other}.go) doldurur;
// S20.3'te eBPF/ETW için gerçek yetenek ölçümü eklenir.
type attrCaps struct {
	ebpf bool
	etw  bool
	pcap bool
	// note, "auto" modunda tercih edilen arka ucun neden atlandığını açıklar
	// ("kernel 5.4 < 5.8", "BTF yok", "yönetici değil"); yalnızca düşüş olunca
	// loglanır. Boş = engel yok.
	note string
}

// probeAttrCaps, çalışılan platformun atıf yeteneklerini ölçer. Test
// enjeksiyonu için değişken.
var probeAttrCaps = func() attrCaps { return platformAttrCaps() }

// buildAttrSource, tek bir arka ucu kurmayı dener. Platform dosyası
// platformBuildAttrSource'u sarar; test enjeksiyonu için değişken.
var buildAttrSource = func(method string, cfg AttrConfig) (AttrSource, error) {
	return platformBuildAttrSource(method, cfg)
}

// NewAttrSource, yapılandırma + platform yeteneğine göre bir atıf arka ucu
// kurar. "auto" modunda tercih sırası eBPF → ETW → pcap; kurulamayan her
// adımda bir sonrakine düşülür ve nedeni loglanır.
func NewAttrSource(cfg AttrConfig) (AttrSource, error) {
	return newAttrSource(cfg, probeAttrCaps())
}

func newAttrSource(cfg AttrConfig, caps attrCaps) (AttrSource, error) {
	plan := attrPlan(cfg.Method, caps)
	if len(plan) == 0 {
		if cfg.Method == "off" {
			return nil, fmt.Errorf("süreç atfı kapalı (collect.method=off)")
		}
		return nil, fmt.Errorf("kullanılabilir atıf arka ucu yok (method=%q)", methodOrAuto(cfg.Method))
	}
	var tried []string
	for _, m := range plan {
		src, err := buildAttrSource(m, cfg)
		if err == nil {
			if len(tried) > 0 || caps.note != "" {
				slog.Info("süreç atfı arka ucu seçildi", "yöntem", m, "denenen", tried, "not", caps.note)
			}
			return src, nil
		}
		tried = append(tried, m)
		if len(plan) > 1 {
			slog.Warn("atıf arka ucu kurulamadı, sıradaki deneniyor", "yöntem", m, "err", err)
		} else {
			return nil, err // zorlanmış tek yöntem — düşme yok, ham hatayı döndür
		}
	}
	return nil, fmt.Errorf("tüm atıf arka uçları başarısız (%s)", strings.Join(plan, "→"))
}

// attrPlan, method + yeteneklere göre denenecek arka uç sırasını verir (saf).
func attrPlan(method string, caps attrCaps) []string {
	switch method {
	case "off":
		return nil
	case "ebpf", "pcap", "etw":
		return []string{method} // zorlanmış
	default: // "" | "auto"
		return autoAttrPlan(caps)
	}
}

// autoAttrPlan, yalnızca kullanılabilir arka uçları tercih sırasıyla listeler.
// Linux'ta caps.etw, Windows'ta caps.ebpf her zaman false olduğundan tek bir
// sıralama üç platform için de doğrudur.
func autoAttrPlan(caps attrCaps) []string {
	var plan []string
	if caps.ebpf {
		plan = append(plan, "ebpf")
	}
	if caps.etw {
		plan = append(plan, "etw")
	}
	if caps.pcap {
		plan = append(plan, "pcap")
	}
	return plan
}

func methodOrAuto(m string) string {
	if m == "" {
		return "auto"
	}
	return m
}
