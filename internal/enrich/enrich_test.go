package enrich

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/geoip"
)

type fakeGeo map[string]geoip.Info

func (f fakeGeo) Lookup(ip string) geoip.Info { return f[ip] }

func TestIPEnrichment(t *testing.T) {
	s := New(fakeGeo{
		"160.79.104.10": {Country: "US", ASN: "AS399358 Anthropic, PBC"},
		"8.8.8.8":       {Country: "US", ASN: "AS15169"},
	}, "")

	got := s.IP("160.79.104.10")
	if got.Private || got.Country != "US" || got.ASN != "AS399358" || got.Org != "Anthropic, PBC" {
		t.Fatalf("public IP zenginleştirme hatalı: %+v", got)
	}
	// org'suz ASN
	if g := s.IP("8.8.8.8"); g.ASN != "AS15169" || g.Org != "" {
		t.Fatalf("org'suz ASN: %+v", g)
	}
	// RFC1918 → Private, lookup yok
	for _, ip := range []string{"10.1.2.3", "192.168.1.1", "172.16.0.1", "127.0.0.1", "::1", "169.254.1.1"} {
		g := s.IP(ip)
		if !g.Private || g.Country != "" {
			t.Fatalf("%s → Private beklenirdi: %+v", ip, g)
		}
	}
	// geçersiz
	if g := s.IP("not-an-ip"); g.Private || g.Country != "" {
		t.Fatalf("geçersiz IP: %+v", g)
	}
}

func TestDomainEnrichment(t *testing.T) {
	dir := t.TempDir()
	catFile := filepath.Join(dir, "cats.txt")
	os.WriteFile(catFile, []byte("# yorum\nanthropic.com  AI\napi.openai.com,AI\ngithub.com Dev\n"), 0o644)

	s := New(nil, catFile)

	cases := []struct {
		in             string
		norm, reg, cat string
	}{
		{"API.Anthropic.com.", "api.anthropic.com", "anthropic.com", "AI"}, // kayıtlı-alan kategorisi
		{"api.openai.com", "api.openai.com", "openai.com", "AI"},           // tam eşleşme
		{"sub.github.com", "sub.github.com", "github.com", "Dev"},
		{"foo.co.uk", "foo.co.uk", "foo.co.uk", ""}, // çok parçalı eTLD
		{"a.b.foo.co.uk", "a.b.foo.co.uk", "foo.co.uk", ""},
		{"nokategori.net", "nokategori.net", "nokategori.net", ""},
	}
	for _, c := range cases {
		got := s.Domain(c.in)
		if got.Normalized != c.norm || got.Registrable != c.reg || got.Category != c.cat {
			t.Errorf("Domain(%q) = %+v, beklenen norm=%q reg=%q cat=%q", c.in, got, c.norm, c.reg, c.cat)
		}
	}
	// IP / noktasız → boş
	if g := s.Domain("localhost"); g.Normalized != "" {
		t.Errorf("localhost → boş beklenirdi: %+v", g)
	}
	if g := s.Domain("1.2.3.4"); g.Normalized != "" {
		t.Errorf("IP → boş beklenirdi: %+v", g)
	}
	// önbellek: ikinci çağrı aynı sonuç
	if s.Domain("api.openai.com") != s.Domain("api.openai.com") {
		t.Error("önbellek tutarsız")
	}
}
