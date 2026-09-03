package server

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/store"
)

// geoCountry, GeoIP haritasında tek bir ülke balonu.
type geoCountry struct {
	Country  string  `json:"country"` // ISO2
	Name     string  `json:"name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Bytes    uint64  `json:"bytes"`
	Sessions int     `json:"sessions"` // farklı uzak IP sayısı
}

// handleGeo, son `minutes` dakikadaki uzak uç noktaları (NetFlow + agent süreç
// trafiği) ülkeye göre toplar ve haritada gösterilebilir merkez koordinatıyla
// döndürür. GeoIP kaynağı yoksa (MMDB/ip-api kapalı) boş liste.
func (s *Server) handleGeo(w http.ResponseWriter, r *http.Request) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	eps, err := s.store.FleetTopEndpoints(since, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lookup := func(ip string) string { return "" }
	if s.geo != nil {
		lookup = func(ip string) string { return s.geo.Lookup(ip).Country }
	}
	writeJSON(w, aggregateGeo(eps, lookup))
}

// aggregateGeo, uzak uç noktaları ülkeye göre toplar; `lookup` bir IP için
// ISO2 ülke kodu döndürür (boş = bilinmiyor/özel IP). Merkez koordinatı
// olmayan ülkeler atlanır. Çıktı bytes'a göre azalan.
func aggregateGeo(eps []store.EndpointDelta, lookup func(string) string) []geoCountry {
	type agg struct {
		bytes    uint64
		sessions int
	}
	byCountry := map[string]*agg{}
	for _, e := range eps {
		iso := lookup(e.IP)
		if iso == "" {
			continue
		}
		a := byCountry[iso]
		if a == nil {
			a = &agg{}
			byCountry[iso] = a
		}
		a.bytes += e.BytesIn + e.BytesOut
		a.sessions++
	}

	out := make([]geoCountry, 0, len(byCountry))
	for iso, a := range byCountry {
		c, ok := geoip.CountryCentroid(iso)
		if !ok {
			continue
		}
		out = append(out, geoCountry{
			Country: iso, Name: c.Name, Lat: c.Lat, Lon: c.Lon,
			Bytes: a.bytes, Sessions: a.sessions,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}
