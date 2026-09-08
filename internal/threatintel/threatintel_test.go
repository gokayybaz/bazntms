package threatintel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/ioc"
)

type fakeProvider struct {
	name string
	m    map[string]Indicator
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Lookup(indicator, typ string) (Indicator, bool) {
	if ind, ok := f.m[typ+"|"+indicator]; ok {
		return ind, true
	}
	return Indicator{}, false
}

func TestServiceMergesMostSevere(t *testing.T) {
	p1 := fakeProvider{name: "p1", m: map[string]Indicator{
		"domain|evil.example": {Reputation: Suspicious, Confidence: 40},
	}}
	p2 := fakeProvider{name: "p2", m: map[string]Indicator{
		"domain|evil.example": {Reputation: Malicious, Confidence: 90},
	}}
	s := New(time.Minute, p1, p2)

	got := s.Domain("EVIL.example.")
	if got.Reputation != Malicious || got.Confidence != 90 || got.Source != "p2" {
		t.Fatalf("en şiddetli sağlayıcı kazanmalı: %+v", got)
	}
	// bilinmeyen → Unknown, panik yok
	if u := s.Domain("temiz.example"); u.Reputation != Unknown {
		t.Fatalf("bilinmeyen domain Unknown olmalı: %+v", u)
	}
	// özel IP → hep Unknown, sağlayıcı sorulmaz
	badIP := fakeProvider{name: "x", m: map[string]Indicator{"ip|10.0.0.1": {Reputation: Malicious}}}
	s2 := New(time.Minute, badIP)
	if g := s2.IP("10.0.0.1"); g.Reputation != Unknown {
		t.Fatalf("RFC1918 IP hep Unknown: %+v", g)
	}
	// noktasız / boş domain
	if g := s.Domain("localhost"); g.Reputation != Unknown {
		t.Fatalf("noktasız domain Unknown: %+v", g)
	}
}

func TestServiceCaches(t *testing.T) {
	calls := 0
	counting := countingProvider{fn: func() { calls++ }}
	s := New(time.Minute, counting)
	s.Domain("a.example")
	s.Domain("a.example")
	if calls != 1 {
		t.Fatalf("ikinci çağrı önbellekten olmalı: %d sağlayıcı çağrısı", calls)
	}
}

type countingProvider struct{ fn func() }

func (countingProvider) Name() string { return "c" }
func (c countingProvider) Lookup(string, string) (Indicator, bool) {
	c.fn()
	return Indicator{}, false
}

func TestLocalFileProvider(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ioc.txt")
	os.WriteFile(f, []byte("# yorum\nevil-c2.example\n185.220.101.5\n0.0.0.0 ad-tracker.example\n"), 0o644)
	list, err := ioc.Load(f)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if list.Count() != 2 || list.IPCount() != 1 {
		t.Fatalf("domain=%d ip=%d beklenen 2/1", list.Count(), list.IPCount())
	}
	s := New(time.Hour, NewLocalFile(list))

	if g := s.Domain("sub.evil-c2.example"); g.Reputation != Malicious || g.Source != "localfile" {
		t.Fatalf("alt alan malicious olmalı: %+v", g)
	}
	if g := s.IP("185.220.101.5"); g.Reputation != Malicious {
		t.Fatalf("IP malicious olmalı: %+v", g)
	}
	if g := s.IP("8.8.8.8"); g.Reputation != Unknown {
		t.Fatalf("listede olmayan IP Unknown: %+v", g)
	}
	if !s.Enabled() || len(s.Providers()) != 1 || s.Providers()[0] != "localfile" {
		t.Fatalf("sağlayıcı listesi hatalı: %+v", s.Providers())
	}
}
