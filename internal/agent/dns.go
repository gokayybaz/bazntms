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

// parseDNSNames, bir UDP/53 payload'inda sorulan domain adlarini (ters arama
// hariç) ve mesajin yanit olup olmadigini dondurur. gopacket layers.DNS ile.
func parseDNSNames(payload []byte) (names []string, isResp bool) {
	if len(payload) < 12 {
		return nil, false
	}
	d := &layers.DNS{}
	if d.DecodeFromBytes(payload, gopacket.NilDecodeFeedback) != nil {
		return nil, false
	}
	if d.QDCount == 0 {
		return nil, false
	}
	for i := range d.Questions {
		n := strings.ToLower(strings.TrimSuffix(string(d.Questions[i].Name), "."))
		if n == "" || len(n) > 253 || !strings.Contains(n, ".") ||
			strings.HasSuffix(n, ".in-addr.arpa") || strings.HasSuffix(n, ".ip6.arpa") ||
			strings.HasSuffix(n, ".local") || strings.HasSuffix(n, ".arpa") {
			continue
		}
		names = append(names, n)
	}
	return names, d.QR
}

// DNSDeltas, son gonderimden bu yana surec bazli DNS sorgu/yanit farklarini
// dondurur.
func (e *AttrEngine) DNSDeltas() []telemetry.DNSSample {
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
