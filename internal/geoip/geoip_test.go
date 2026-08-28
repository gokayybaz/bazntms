package geoip

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrivateIPsNeverResolved(t *testing.T) {
	r := New("", "", true)
	for _, ip := range []string{"192.168.1.43", "127.0.0.1", "10.0.0.1", "::1", "fe80::1", "172.16.5.4"} {
		if info := r.Lookup(ip); info != (Info{}) {
			t.Fatalf("ozel IP cozumlenmemeli: %s -> %+v", ip, info)
		}
	}
	if r.Enabled() != true {
		t.Fatal("ip-api modu acik olmali")
	}
}

func TestIPAPIBatchFlow(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"status":"success","countryCode":"US","as":"AS15169 Google LLC","query":"8.8.8.8"},
			{"status":"success","countryCode":"TR","as":"AS9121 Turk Telekom","query":"212.156.4.4"}
		]`))
	}))
	defer srv.Close()

	r := New("", "", true)
	r.apiURL = srv.URL

	// ilk lookup: kuyruga alinir, bos doner
	if info := r.Lookup("8.8.8.8"); info != (Info{}) {
		t.Fatalf("kuyruk oncesi bos donmeliydi: %+v", info)
	}
	r.Lookup("212.156.4.4")

	r.flush() // kuyrugu servise gonder
	if calls.Load() != 1 {
		t.Fatalf("1 batch cagrisi beklenirdi: %d", calls.Load())
	}

	got := r.Lookup("8.8.8.8")
	if got.Country != "US" || got.ASN != "AS15169 Google LLC" {
		t.Fatalf("cache doldurulmadi: %+v", got)
	}
	got = r.Lookup("212.156.4.4")
	if got.Country != "TR" {
		t.Fatalf("TR beklenirdi: %+v", got)
	}

	// flush bosa cikar: yeni ip yok, yeni cagri olmamali
	r.flush()
	if calls.Load() != 1 {
		t.Fatalf("bostan cagri olmamaliydi: %d", calls.Load())
	}
}

func TestAPILimitBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	r := New("", "", true)
	r.apiURL = srv.URL
	r.Lookup("8.8.4.4")
	r.flush()

	r.mu.Lock()
	inCooldown := time.Now().Before(r.cooldown)
	r.mu.Unlock()
	if !inCooldown {
		t.Fatal("429 sonrasi cooldown beklenirdi")
	}
}
