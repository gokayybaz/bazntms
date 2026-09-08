package alert

// S22.13: bildirim yönlendirme — routeAllows kural değerlendirmesi.

import (
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestRouteAllows(t *testing.T) {
	ev := func(sev, kind, site string) store.AlertEvent {
		return store.AlertEvent{Severity: sev, Kind: kind, Site: site}
	}

	// kural yok → hepsi izinli
	if a := routeAllows(nil, ev("warn", "bw", "dc1")); !a(ChSlack) || !a(ChEmail) {
		t.Fatal("kuralsız: hepsi izinli olmalı")
	}

	routes := []NotifyRoute{
		{Severity: "crit", Channels: []string{"pagerduty", ChSlack}},
		{Kind: "anomaly", Channels: []string{ChSlack}, Continue: true},
		{Site: "dc9", Channels: []string{ChEmail}},
	}

	// crit → pagerduty + slack, email değil
	a := routeAllows(routes, ev("crit", "bw", "dc1"))
	if !a("pagerduty") || !a(ChSlack) || a(ChEmail) {
		t.Fatalf("crit yönlendirmesi yanlış")
	}

	// anomaly (warn) → ilk kural eşleşmez, 2. eşleşir (continue) → slack; 3. site farklı
	a = routeAllows(routes, ev("warn", "anomaly", "dc1"))
	if !a(ChSlack) || a(ChEmail) || a("pagerduty") {
		t.Fatalf("anomaly yönlendirmesi yanlış")
	}

	// hiçbir kural eşleşmez → güvenli-varsayılan hepsi
	a = routeAllows(routes, ev("info", "port", "dc1"))
	if !a(ChSlack) || !a(ChEmail) {
		t.Fatalf("eşleşmeyince hepsi izinli olmalı")
	}

	// continue olmayan ilk kuralda durur: crit + anomaly → yalnız 1. kural
	a = routeAllows(routes, ev("crit", "anomaly", "dc1"))
	if !a("pagerduty") || a(ChEmail) {
		t.Fatalf("continue'suz kuralda durmalı")
	}
}
