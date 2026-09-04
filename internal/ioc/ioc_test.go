package ioc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeList(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ioc.txt")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseLineFormats(t *testing.T) {
	cases := map[string]string{
		"evil.com":                    "evil.com",
		"  EVIL.com.  ":               "evil.com",
		"0.0.0.0 ads.evil.com":        "ads.evil.com",
		"127.0.0.1\ttracker.evil.com": "tracker.evil.com",
		"||banner.evil.com^":          "banner.evil.com",
		"malware.com # bilinen c2":    "malware.com",
		"# yorum satırı":              "",
		"! adblock yorum":             "",
		"":                            "",
		"   ":                         "",
		"localhost":                   "", // nokta yok
		"*.evil.com":                  "", // joker — reddet
	}
	for in, want := range cases {
		if got := parseLine(in); got != want {
			t.Errorf("parseLine(%q) = %q, %q bekleniyordu", in, got, want)
		}
	}
}

func TestMatchExactAndParent(t *testing.T) {
	p := writeList(t, "evil.com\nc2.malware.net\n0.0.0.0 ads.tracker.io\n")
	l, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if l.Count() != 3 {
		t.Fatalf("3 domain bekleniyordu: %d", l.Count())
	}

	hit := []string{"evil.com", "www.evil.com", "a.b.evil.com", "c2.malware.net", "ADS.TRACKER.IO", "x.ads.tracker.io"}
	for _, d := range hit {
		if rule, ok := l.Match(d); !ok {
			t.Errorf("%s eşleşmeliydi", d)
		} else if rule == "" {
			t.Errorf("%s: kural boş döndü", d)
		}
	}

	miss := []string{"evil.com.tr", "notevil.com", "malware.net", "net", "com", "tracker.io", "good.example"}
	for _, d := range miss {
		if rule, ok := l.Match(d); ok {
			t.Errorf("%s eşleşmemeliydi (kural: %s)", d, rule)
		}
	}
}

func TestMatchParentRuleReported(t *testing.T) {
	l, _ := Load(writeList(t, "evil.com\n"))
	rule, ok := l.Match("deep.sub.evil.com")
	if !ok || rule != "evil.com" {
		t.Fatalf("üst alan kuralı 'evil.com' bekleniyordu: %q ok=%v", rule, ok)
	}
}

func TestReloadOnChange(t *testing.T) {
	p := writeList(t, "one.evil.com\n")
	l, _ := Load(p)
	if _, ok := l.Match("two.evil.com"); ok {
		t.Fatal("two henüz listede değil")
	}

	// mtime'ı ileri al ki reload tetiklensin
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(p, []byte("one.evil.com\ntwo.evil.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	os.Chtimes(p, future, future)

	stop := make(chan struct{})
	go l.Watch(20*time.Millisecond, stop)
	defer close(stop)

	deadline := time.After(2 * time.Second)
	for {
		if _, ok := l.Match("two.evil.com"); ok {
			return // başarı
		}
		select {
		case <-deadline:
			t.Fatal("Watch listeyi yeniden yüklemedi")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "yok.txt")); err == nil {
		t.Fatal("olmayan dosya hata döndürmeliydi")
	}
}
