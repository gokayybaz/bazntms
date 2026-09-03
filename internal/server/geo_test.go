package server

import (
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAggregateGeo(t *testing.T) {
	eps := []store.EndpointDelta{
		{IP: "8.8.8.8", BytesIn: 100, BytesOut: 50},
		{IP: "8.8.4.4", BytesIn: 200, BytesOut: 0},    // ayni ulke (US)
		{IP: "212.156.4.4", BytesIn: 10, BytesOut: 5}, // TR
		{IP: "10.0.0.1", BytesIn: 999, BytesOut: 999}, // ozel IP -> lookup bos
		{IP: "1.2.3.4", BytesIn: 7, BytesOut: 3},      // XX -> merkez yok, atlanir
	}
	lookup := func(ip string) string {
		switch ip {
		case "8.8.8.8", "8.8.4.4":
			return "US"
		case "212.156.4.4":
			return "TR"
		case "1.2.3.4":
			return "XX"
		}
		return ""
	}

	out := aggregateGeo(eps, lookup)
	if len(out) != 2 {
		t.Fatalf("2 ulke beklenirdi (US, TR), gelen %d: %+v", len(out), out)
	}
	// bytes'a gore azalan: US once
	if out[0].Country != "US" || out[0].Bytes != 350 || out[0].Sessions != 2 {
		t.Errorf("US yanlis: %+v", out[0])
	}
	if out[1].Country != "TR" || out[1].Bytes != 15 || out[1].Sessions != 1 {
		t.Errorf("TR yanlis: %+v", out[1])
	}
	if out[0].Name == "" || out[0].Lat == 0 {
		t.Errorf("US merkez koordinati doldurulmadi: %+v", out[0])
	}
}

func TestAggregateGeoNilLookup(t *testing.T) {
	eps := []store.EndpointDelta{{IP: "8.8.8.8", BytesIn: 1}}
	out := aggregateGeo(eps, func(string) string { return "" })
	if len(out) != 0 {
		t.Fatalf("GeoIP kaynagi yokken bos liste beklenirdi: %+v", out)
	}
}
