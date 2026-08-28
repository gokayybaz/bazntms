package capture

import (
	"net"
	"sort"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	maxDomains    = 4096
	maxIPsPerHost = 4
	topDomains    = 30
)

type DomainStat struct {
	Domain    string   `json:"domain"`
	Queries   uint64   `json:"queries"`
	Responses uint64   `json:"responses"`
	IPs       []string `json:"ips,omitempty"`
}

type dnsAgg struct {
	Queries   uint64
	Responses uint64
}

type dnsParsed struct {
	names   []string
	isResp  bool
	answers []string
}

// parseDNS, bir UDP/53 payload'ini ayristirir (kilitlenmesiz, saf fonksiyon).
func parseDNS(payload []byte) *dnsParsed {
	if len(payload) < 12 {
		return nil
	}
	d := &layers.DNS{}
	if err := d.DecodeFromBytes(payload, gopacket.NilDecodeFeedback); err != nil {
		return nil
	}
	if d.QDCount == 0 && d.ANCount == 0 {
		return nil
	}
	p := &dnsParsed{isResp: d.QR}
	for i := range d.Questions {
		name := normalizeDomain(string(d.Questions[i].Name))
		if name == "" || isReverseLookup(name) {
			continue
		}
		p.names = append(p.names, name)
	}
	if !p.isResp {
		return p
	}
	// yanitlarda A/AAAA kayitlarindan IP topla
	addrs := make([]net.IP, 0, len(d.Answers)+len(d.Additionals))
	for i := range d.Answers {
		addrs = append(addrs, d.Answers[i].IP)
	}
	for i := range d.Additionals {
		addrs = append(addrs, d.Additionals[i].IP)
	}
	for _, ip := range addrs {
		if ip == nil {
			continue
		}
		p.answers = append(p.answers, ip.String())
	}
	return p
}

func normalizeDomain(s string) string {
	s = strings.ToLower(strings.TrimSuffix(s, "."))
	if s == "" || len(s) > 253 {
		return ""
	}
	return s
}

func isReverseLookup(name string) bool {
	return strings.HasSuffix(name, ".in-addr.arpa") || strings.HasSuffix(name, ".ip6.arpa")
}

// applyDNS, ayristirilmis DNS bilgisini sayaclara isler (e.mu kilitliyken cagir).
func (e *Engine) applyDNS(p *dnsParsed) {
	if p == nil || len(p.names) == 0 {
		return
	}
	for _, name := range p.names {
		agg, ok := e.dnsCounts[name]
		if !ok {
			if len(e.dnsCounts) >= maxDomains {
				continue // tasma korumasi
			}
			agg = &dnsAgg{}
			e.dnsCounts[name] = agg
		}
		if p.isResp {
			agg.Responses++
		} else {
			agg.Queries++
		}
	}
	// yanit IP'lerini sorulan domainlere bagla
	if p.isResp && len(p.answers) > 0 {
		for _, name := range p.names {
			set, ok := e.domainIPs[name]
			if !ok {
				if _, tracked := e.dnsCounts[name]; !tracked {
					continue
				}
				set = map[string]struct{}{}
				e.domainIPs[name] = set
			}
			for _, ip := range p.answers {
				if len(set) >= maxIPsPerHost {
					break
				}
				set[ip] = struct{}{}
			}
		}
	}
}

func (e *Engine) topDomainStats() []DomainStat {
	out := make([]DomainStat, 0, len(e.dnsCounts))
	for name, agg := range e.dnsCounts {
		if agg.Queries+agg.Responses == 0 {
			continue
		}
		var ips []string
		if set, ok := e.domainIPs[name]; ok {
			ips = make([]string, 0, len(set))
			for ip := range set {
				ips = append(ips, ip)
			}
			sort.Strings(ips)
		}
		out = append(out, DomainStat{
			Domain:    name,
			Queries:   agg.Queries,
			Responses: agg.Responses,
			IPs:       ips,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].Queries+out[i].Responses, out[j].Queries+out[j].Responses
		if ti != tj {
			return ti > tj
		}
		return out[i].Domain < out[j].Domain
	})
	if len(out) > topDomains {
		out = out[:topDomains]
	}
	return out
}
