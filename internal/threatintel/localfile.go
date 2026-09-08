package threatintel

import "github.com/gokayybaz/bazntms/internal/ioc"

// LocalFileProvider, `-ioc-file` kara listesini (internal/ioc) bir tehdit
// istihbaratı sağlayıcısına sarar. Eşleşen domain/IP → malicious (yüksek
// güven; operatör listeyi kasıtlı sağladı).
type LocalFileProvider struct {
	list *ioc.List
}

func NewLocalFile(list *ioc.List) *LocalFileProvider { return &LocalFileProvider{list: list} }

func (p *LocalFileProvider) Name() string { return "localfile" }

func (p *LocalFileProvider) Lookup(indicator, typ string) (Indicator, bool) {
	if p.list == nil {
		return Indicator{}, false
	}
	switch typ {
	case "domain":
		if rule, hit := p.list.Match(indicator); hit {
			return Indicator{
				Reputation: Malicious, Confidence: 85, Source: "localfile",
				Categories: []string{"blocklist"}, RawRef: rule,
			}, true
		}
	case "ip":
		if p.list.MatchIP(indicator) {
			return Indicator{
				Reputation: Malicious, Confidence: 85, Source: "localfile",
				Categories: []string{"blocklist"}, RawRef: indicator,
			}, true
		}
	}
	return Indicator{}, false
}
