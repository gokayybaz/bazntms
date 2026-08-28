package geoip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// Info, bir IP'nin coğrafi/ağ sahibi ozeti.
type Info struct {
	Country string `json:"country,omitempty"` // ISO 3166-1 alpha-2 (TR, US, ...)
	ASN     string `json:"asn,omitempty"`     // "AS15169 Google LLC"
}

const (
	maxCache          = 100_000
	batchSize         = 100
	ipAPILimitBackoff = time.Minute
)

// Resolver, IP -> ulke/ASN cozumleme yapar. Iki kaynak destekler:
//  1. MaxMind GeoLite2 MMDB dosyalari (offline, en iyi)
//  2. ip-api.com batch servisi (anahtar gerektirmez, internet ister)
//
// MMDB yoksa ve ipAPI aciksa bilinmeyen genel IP'ler kuyruga alinip
// 3 saniyede bir toplu (batch) cozumlenir.
type Resolver struct {
	mu       sync.RWMutex
	cache    map[string]Info
	queue    []string
	queued   map[string]struct{}
	cooldown time.Time

	countryDB *geoip2.Reader
	asnDB     *geoip2.Reader
	ipAPI     bool
	apiURL    string // test icin degistirilebilir
	hc        *http.Client
}

// New, cozumleyiciyi kurar. countryPath/asnPath bos degilse MMDB kullanilir.
func New(countryPath, asnPath string, ipAPI bool) *Resolver {
	r := &Resolver{
		cache:  map[string]Info{},
		queued: map[string]struct{}{},
		ipAPI:  ipAPI,
		apiURL: "http://ip-api.com/batch",
		hc:     &http.Client{Timeout: 10 * time.Second},
	}

	if countryPath != "" {
		db, err := geoip2.Open(countryPath)
		if err != nil {
			log.Printf("GeoIP: %s acilamadi: %v", countryPath, err)
		} else {
			r.countryDB = db
			log.Printf("GeoIP: ulke veritabani aktif (%s)", filepathBase(countryPath))
		}
	}
	if asnPath != "" {
		db, err := geoip2.Open(asnPath)
		if err != nil {
			log.Printf("GeoIP: %s acilamadi: %v", asnPath, err)
		} else {
			r.asnDB = db
			log.Printf("GeoIP: ASN veritabani aktif (%s)", filepathBase(asnPath))
		}
	}

	// ip-api yalnizca MMDB yokken devreye girer
	useIPAPI := ipAPI && r.countryDB == nil
	if useIPAPI {
		log.Printf("GeoIP: ip-api.com toplu cozumleme modu (internet gerektirir; kapatmak icin -ip-api-lookup=false)")
	}
	r.ipAPI = useIPAPI

	if r.ipAPI {
		go r.worker()
	}
	return r
}

// Enabled, herhangi bir kaynak aktif mi?
func (r *Resolver) Enabled() bool {
	return r.countryDB != nil || r.asnDB != nil || r.ipAPI
}

// Lookup, IP icin Info dondurur. Cozunemeyen genel IP'ler ip-api modunda
// kuyruga alinir; sonraki cagrilarda cache'den döner. Ozel/loopback IP'ler
// hic cozumlenmez.
func (r *Resolver) Lookup(ipStr string) Info {
	ip := net.ParseIP(ipStr)
	if ip == nil || !isPublic(ip) {
		return Info{}
	}

	r.mu.RLock()
	info, ok := r.cache[ipStr]
	r.mu.RUnlock()
	if ok {
		return info
	}

	if r.countryDB != nil || r.asnDB != nil {
		info = r.mmdbLookup(ip)
		r.store(ipStr, info)
		return info
	}

	if r.ipAPI {
		r.enqueue(ipStr)
	}
	return Info{}
}

func (r *Resolver) store(ip string, info Info) {
	r.mu.Lock()
	if len(r.cache) > maxCache {
		r.cache = map[string]Info{} // basit tasma korumasi
	}
	r.cache[ip] = info
	r.mu.Unlock()
}

func (r *Resolver) enqueue(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.queued[ip]; ok {
		return
	}
	if len(r.queue) >= batchSize*4 {
		return // kuyruk tasma korumasi
	}
	r.queue = append(r.queue, ip)
	r.queued[ip] = struct{}{}
}

func (r *Resolver) worker() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for range t.C {
		r.flush()
	}
}

func (r *Resolver) flush() {
	r.mu.Lock()
	if time.Now().Before(r.cooldown) {
		r.mu.Unlock()
		return
	}
	n := len(r.queue)
	if n > batchSize {
		n = batchSize
	}
	batch := r.queue[:n]
	r.queue = r.queue[n:]
	for _, ip := range batch {
		delete(r.queued, ip)
	}
	r.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return
	}
	url := r.apiURL + "?fields=status,countryCode,as,query"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.hc.Do(req)
	if err != nil {
		log.Printf("GeoIP ip-api: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		r.mu.Lock()
		r.cooldown = time.Now().Add(ipAPILimitBackoff)
		r.mu.Unlock()
		return
	}
	if resp.StatusCode != http.StatusOK {
		return
	}

	var results []struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
		AS          string `json:"as"`
		Query       string `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return
	}
	for _, res := range results {
		if res.Query == "" || res.Status != "success" {
			continue
		}
		info := Info{Country: res.CountryCode, ASN: res.AS}
		if strings.HasPrefix(info.Country, "A") && len(info.Country) != 2 {
			info.Country = ""
		}
		r.store(res.Query, info)
	}
}

// mmdbLookup, yerel MaxMind dosyalarindan cozumler.
func (r *Resolver) mmdbLookup(ip net.IP) Info {
	var info Info
	if r.countryDB != nil {
		if rec, err := r.countryDB.Country(ip); err == nil && rec.Country.IsoCode != "" {
			info.Country = rec.Country.IsoCode
		}
	}
	if r.asnDB != nil {
		if rec, err := r.asnDB.ASN(ip); err == nil && rec.AutonomousSystemNumber > 0 {
			org := rec.AutonomousSystemOrganization
			if org == "" {
				info.ASN = fmt.Sprintf("AS%d", rec.AutonomousSystemNumber)
			} else {
				info.ASN = fmt.Sprintf("AS%d %s", rec.AutonomousSystemNumber, org)
			}
		}
	}
	return info
}

// isPublic, cozumlemeye deger genel IP'leri ayirir.
func isPublic(ip net.IP) bool {
	// IPv4-mapped IPv6 adreslerini IPv4 temsiline indir
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}

func filepathBase(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
