// Package enrich, uzak IP / alan adı telemetrisini bağlamsal meta ile
// zenginleştiren paylaşılan servistir (Faz 23-E). Süreç detayı, NetFlow
// konuşmaları, coğrafi harita ve (ileride) olay korelasyonu aynı servisi
// kullanır — tek önbellek, tek normalizasyon.
//
//   - IP  : ülke/ASN/organizasyon (geoip.Resolver'a devreder — o zaten 100k
//     LRU önbellekli) + özel/genel sınıflandırma (RFC1918/ULA/loopback).
//   - alan: normalize (küçük harf, kök nokta), kayıtlı alan (eTLD+1,
//     publicsuffix), opsiyonel kategori (statik dosya).
//
// Zenginleştirme salt okuma-yolu: hata/eksik veri ingest'i asla bloklamaz.
package enrich

import (
	"bufio"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"

	"github.com/gokayybaz/bazntms/internal/geoip"
)

// IPInfo, bir IP'nin zenginleştirilmiş görünümü.
type IPInfo struct {
	IP      string `json:"ip"`
	Private bool   `json:"private"` // RFC1918 / ULA / loopback / link-local / multicast
	Country string `json:"country,omitempty"`
	ASN     string `json:"asn,omitempty"` // "AS15169"
	Org     string `json:"org,omitempty"` // "Google LLC"
}

// DomainInfo, bir alan adının normalize + kayıtlı-alan + kategori görünümü.
type DomainInfo struct {
	Domain      string `json:"domain"`
	Normalized  string `json:"normalized"`
	Registrable string `json:"registrable"`        // eTLD+1
	Category    string `json:"category,omitempty"` // -domain-category-file'dan
}

// geoLookuper, enrich'in ihtiyaç duyduğu tek geoip yeteneği (test için mock'lanır).
type geoLookuper interface {
	Lookup(ip string) geoip.Info
}

// Service, zenginleştirme servisidir. New ile kurulur; nil geo verilirse IP
// tarafı yalnız özel/genel sınıflandırma yapar (alan tarafı her zaman çalışır).
type Service struct {
	geo geoLookuper

	mu       sync.RWMutex
	domCache map[string]DomainInfo // normalize edilmiş alan → sonuç (bounded)
	cat      map[string]string     // alan / kayıtlı-alan → kategori (dosyadan)
}

const domCacheMax = 20_000

// New, servisi kurar. categoryFile boş değilse yüklenir (biçim: her satır
// "<alan> <kategori>", "#" yorum; alan tam ya da kayıtlı-alan eşleşir).
func New(geo geoLookuper, categoryFile string) *Service {
	// typed-nil koruması: nil bir *geoip.Resolver interface'e sarılınca
	// "nil değil" görünür → s.geo.Lookup nil-receiver panik'i. Burada düzelt.
	if g, ok := geo.(*geoip.Resolver); ok && g == nil {
		geo = nil
	}
	s := &Service{geo: geo, domCache: map[string]DomainInfo{}, cat: map[string]string{}}
	if categoryFile != "" {
		if err := s.loadCategories(categoryFile); err != nil {
			// yükleme hatası fatal değil — kategori olmadan devam
			s.cat = map[string]string{}
		}
	}
	return s
}

func (s *Service) loadCategories(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cat := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// ayraç: virgül ya da boşluk
		var dom, c string
		if i := strings.IndexAny(line, ", \t"); i > 0 {
			dom = strings.ToLower(strings.TrimSpace(line[:i]))
			c = strings.TrimSpace(strings.TrimLeft(line[i+1:], ", \t"))
		}
		if dom != "" && c != "" {
			cat[dom] = c
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cat = cat
	s.mu.Unlock()
	return nil
}

// IP, bir IP'yi zenginleştirir. Özel/loopback/link-local IP'ler için lookup
// yapılmaz (Private=true).
func (s *Service) IP(ipStr string) IPInfo {
	out := IPInfo{IP: ipStr}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return out
	}
	if !isPublic(ip) {
		out.Private = true
		return out
	}
	if s.geo == nil {
		return out
	}
	info := s.geo.Lookup(ipStr)
	out.Country = info.Country
	out.ASN, out.Org = splitASN(info.ASN)
	return out
}

// Domain, bir alan adını normalize eder + kayıtlı alanı ve kategoriyi ekler.
func (s *Service) Domain(d string) DomainInfo {
	norm := normalizeDomain(d)
	if norm == "" {
		return DomainInfo{Domain: d}
	}
	s.mu.RLock()
	cached, ok := s.domCache[norm]
	s.mu.RUnlock()
	if ok {
		return cached
	}

	out := DomainInfo{Domain: d, Normalized: norm}
	if reg, err := publicsuffix.EffectiveTLDPlusOne(norm); err == nil {
		out.Registrable = reg
	} else {
		out.Registrable = norm
	}

	s.mu.RLock()
	if c, ok := s.cat[norm]; ok {
		out.Category = c
	} else if c, ok := s.cat[out.Registrable]; ok {
		out.Category = c
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if len(s.domCache) >= domCacheMax {
		s.domCache = map[string]DomainInfo{} // basit tam-temizleme (LRU'ya gerek yok)
	}
	s.domCache[norm] = out
	s.mu.Unlock()
	return out
}

// normalizeDomain, küçük harf + kök nokta/whitespace temizler; IP ya da
// noktasız değerleri ("localhost") boş döndürür.
func normalizeDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.TrimSuffix(d, ".")
	if d == "" || !strings.Contains(d, ".") {
		return ""
	}
	if net.ParseIP(d) != nil {
		return ""
	}
	return d
}

// splitASN, "AS15169 Google LLC" → ("AS15169", "Google LLC").
func splitASN(s string) (asn, org string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// isPublic, geoip.isPublic ile aynı ölçüt (o paketten dışa açık değil).
func isPublic(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}
