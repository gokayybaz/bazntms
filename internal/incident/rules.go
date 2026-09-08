package incident

import (
	"fmt"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// match, bir kural eşleşmesinin incident'a çevrilmiş hali.
type match struct {
	rule     int
	key      string // correlation_key (dedup) — "r<rule>|agent<id>"
	title    string
	severity string
	site     string
	agentID  int64
	reason   string
	summary  string
	risk     int
	firstTs  int64
	lastTs   int64
	evidence []store.IncidentEvidence
}

// "şüpheli" sayılan uyarı türleri (kural 5 sayımı + genel risk).
var suspiciousKinds = map[string]bool{
	"proc": true, "port": true, "target": true, "ioc": true,
	"anomaly": true, "bw": true, "iface_util": true,
}

// matchRules, tek bir agent'ın (ts artan sıralı) uyarı kümesine deterministik
// kuralları uygular. Bkz. docs/decisions/0011.
func matchRules(agentID int64, cluster []store.AlertEvent, short, long time.Duration, rule5Min int) []match {
	var out []match
	site := ""
	for _, a := range cluster {
		if a.Site != "" {
			site = a.Site
		}
	}

	has := func(kinds map[string]bool, window time.Duration) ([]store.AlertEvent, bool) {
		// pencere içinde `kinds`'in HEPSİNDEN en az bir tane var mı — evet ise
		// katkıda bulunan uyarıları döndür.
		for i := range cluster {
			anchor := cluster[i]
			if !kinds[anchor.Kind] {
				continue
			}
			seen := map[string]store.AlertEvent{anchor.Kind: anchor}
			for j := range cluster {
				if j == i {
					continue
				}
				c := cluster[j]
				if !kinds[c.Kind] {
					continue
				}
				if absTs(c.LastTs, anchor.LastTs) > int64(window.Seconds()) {
					continue
				}
				// kind başına EN ŞİDDETLİ (sonra en yeni) uyarıyı tut →
				// severity monoton yükselebilsin.
				if cur, ok := seen[c.Kind]; !ok ||
					sevRank(c.Severity) > sevRank(cur.Severity) ||
					(sevRank(c.Severity) == sevRank(cur.Severity) && c.LastTs > cur.LastTs) {
					seen[c.Kind] = c
				}
			}
			if len(seen) == len(kinds) {
				evs := make([]store.AlertEvent, 0, len(seen))
				for _, v := range seen {
					evs = append(evs, v)
				}
				return evs, true
			}
		}
		return nil, false
	}

	mk := func(rule int, title, sevFloor string, evs []store.AlertEvent, reason string) match {
		sort.Slice(evs, func(i, j int) bool { return evs[i].LastTs < evs[j].LastTs })
		sev := sevFloor
		for _, ev := range evs {
			sev = maxSev(sev, ev.Severity)
		}
		ev0, evN := evs[0], evs[len(evs)-1]
		m := match{
			rule:     rule,
			key:      fmt.Sprintf("r%d|agent%d", rule, agentID),
			title:    title,
			severity: sev,
			site:     site,
			agentID:  agentID,
			reason:   reason,
			summary:  fmt.Sprintf("%s → %s", ev0.Message, evN.Message),
			firstTs:  ev0.LastTs,
			lastTs:   evN.LastTs,
			risk:     riskScore(rule, sev, evs),
		}
		for _, ev := range evs {
			m.evidence = append(m.evidence, evidenceFor(ev))
		}
		return m
	}

	// Kural 1: yeni süreç + (yeni hedef | IOC) ≤ short
	if evs, ok := has(map[string]bool{"proc": true, "target": true}, short); ok {
		out = append(out, mk(1, "Şüpheli çıkış aktivitesi", "crit", evs, "yeni süreç + yeni hedef aynı agent'ta kısa aralıkta"))
	} else if evs, ok := has(map[string]bool{"proc": true, "ioc": true}, short); ok {
		out = append(out, mk(1, "Şüpheli çıkış aktivitesi (IOC)", "crit", evs, "yeni süreç + tehdit istihbaratı eşleşmesi"))
	}
	// Kural 2: yeni süreç + şüpheli port ≤ short
	if evs, ok := has(map[string]bool{"proc": true, "port": true}, short); ok {
		out = append(out, mk(2, "Yeni süreç + şüpheli port", "crit", evs, "yeni süreç + şüpheli porta bağlantı"))
	}
	// Kural 3: (yeni hedef | IOC) + yüksek trafik ≤ long
	if evs, ok := has(map[string]bool{"target": true, "bw": true}, long); ok {
		out = append(out, mk(3, "Yeni hedefe yüksek transfer", "warn", evs, "yeni hedef + eşik üstü giden trafik"))
	} else if evs, ok := has(map[string]bool{"ioc": true, "bw": true}, long); ok {
		out = append(out, mk(3, "Tehdit hedefine yüksek transfer", "crit", evs, "IOC eşleşmesi + eşik üstü giden trafik"))
	}
	// Kural 4: anomali + bant sıçraması ≤ short
	if evs, ok := has(map[string]bool{"anomaly": true, "bw": true}, short); ok {
		out = append(out, mk(4, "Anomali + bant genişliği sıçraması", "warn", evs, "istatistiksel anomali + eşik üstü verim"))
	}
	// Kural 5: ≥ N şüpheli uyarı ≤ long
	if evs := clusterInWindow(cluster, long); len(evs) >= rule5Min {
		sev := "warn"
		if len(evs) >= rule5Min+2 {
			sev = "crit"
		}
		out = append(out, mk(5, fmt.Sprintf("Çoklu şüpheli aktivite (%d uyarı)", len(evs)), sev, evs,
			fmt.Sprintf("aynı agent'ta %d dk içinde %d şüpheli uyarı", int(long.Minutes()), len(evs))))
	}
	return out
}

// clusterInWindow, `cluster`'daki şüpheli uyarılardan pencereye sığan en büyük
// ardışık grubu döndürür (kayan pencere).
func clusterInWindow(cluster []store.AlertEvent, window time.Duration) []store.AlertEvent {
	var best []store.AlertEvent
	w := int64(window.Seconds())
	for i := range cluster {
		if !suspiciousKinds[cluster[i].Kind] {
			continue
		}
		var grp []store.AlertEvent
		for j := i; j < len(cluster); j++ {
			if !suspiciousKinds[cluster[j].Kind] {
				continue
			}
			if cluster[j].LastTs-cluster[i].LastTs > w {
				break
			}
			grp = append(grp, cluster[j])
		}
		if len(grp) > len(best) {
			best = grp
		}
	}
	return best
}

func absTs(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

// riskScore, 0-100 — açıklanabilir: kural tabanı + önem + kanıt sayısı.
func riskScore(rule int, sev string, evs []store.AlertEvent) int {
	base := map[int]int{1: 55, 2: 55, 3: 40, 4: 35, 5: 30}[rule]
	s := base
	switch sev {
	case "crit":
		s += 25
	case "warn":
		s += 10
	}
	// her ek kanıt +5, en fazla +20
	if n := len(evs) - 2; n > 0 {
		s += min(20, n*5)
	}
	// crit-önem kanıt başına +5
	for _, ev := range evs {
		if ev.Severity == "crit" {
			s += 5
		}
	}
	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return s
}
