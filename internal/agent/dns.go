package agent

import (
	"strings"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type dnsKey struct {
	pid     int32
	process string
	domain  string
}

type dnsAgg struct {
	queries   uint64
	responses uint64
}

// normalizeDomain, bir DNS adını küçük harfe indirip kök noktasını atar.
func normalizeDomain(s string) string {
	return strings.ToLower(strings.TrimSuffix(s, "."))
}

// keepDomain, normalize edilmiş bir domain adının kaydedilmeye değer olup
// olmadığını söyler: ters arama (.in-addr.arpa / .ip6.arpa / .arpa), .local,
// noktasız ve aşırı uzun adlar elenir. pcap, eBPF ve ETW yolları ortak kullanır.
func keepDomain(n string) bool {
	if n == "" || len(n) > 253 || !strings.Contains(n, ".") {
		return false
	}
	return !strings.HasSuffix(n, ".in-addr.arpa") &&
		!strings.HasSuffix(n, ".ip6.arpa") &&
		!strings.HasSuffix(n, ".local") &&
		!strings.HasSuffix(n, ".arpa")
}

// parseDNSNames, bir UDP/53 payload'inda sorulan domain adlarini (ters arama
// hariç) ve mesajin yanit olup olmadigini dondurur. gopacket layers.DNS ile.
func parseDNSNames(payload []byte) (names []string, isResp bool) {
	if len(payload) < 12 {
		return nil, false
	}
	d := &layers.DNS{}
	if !safeDecodeDNS(d, payload) {
		return nil, false
	}
	if d.QDCount == 0 {
		return nil, false
	}
	for i := range d.Questions {
		n := normalizeDomain(string(d.Questions[i].Name))
		if keepDomain(n) {
			names = append(names, n)
		}
	}
	return names, d.QR
}

// safeDecodeDNS, gopacket'in DNS decode'unu calistirir. gopacket@v1.1.19'un
// DNS layer'i bazi bozuk/sikistirilmis-isim-isaretcili paketlerde hata
// dondurmek yerine slice sinirlari disina cikip panic atiyor (canli LAN'da
// mDNS/SSDP gurultusuyle tekrar tekrar tetiklenip surecin tamamini cokertti
// — bu loop() kendi goroutine'inde calisir, recover olmadan panic tum
// bazntms-agent'i sonlandirir). Burada izole bir recover ile bozuk paket
// normal "cozemedim" yoluna (false donus) dusuruluyor.
func safeDecodeDNS(d *layers.DNS, payload []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return d.DecodeFromBytes(payload, gopacket.NilDecodeFeedback) == nil
}

// DNSDeltas, son gonderimden bu yana surec bazli DNS sorgu/yanit farklarini
// dondurur.
func (e *pcapAttrSource) DNSDeltas() []telemetry.DNSSample {
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
