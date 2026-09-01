package main

import (
	"errors"
	"runtime"
	"testing"
)

// TestPcapErrHint, Npcap eksikligi tespitinin yalniz Windows'ta ve yalniz
// gopacket'in ozel "couldn't load wpcap.dll" hatasinda devreye girdigini
// dogrular. Platform-bagimli oldugu icin beklenti runtime.GOOS'a gore
// ayarlanir — CI matrisindeki her 3 platformda (windows/linux/darwin)
// dogru davranisi ayri ayri sinar.
func TestPcapErrHint(t *testing.T) {
	npcapErr := errors.New("couldn't load wpcap.dll")

	hint := pcapErrHint(npcapErr)
	if runtime.GOOS == "windows" {
		if hint == "" {
			t.Fatal("windows'ta wpcap.dll hatasi icin bir ipucu donmeliydi")
		}
	} else if hint != "" {
		t.Fatalf("windows disinda ipucu donmemeliydi, gelen: %q", hint)
	}

	if got := pcapErrHint(errors.New("izin reddedildi")); got != "" {
		t.Fatalf("ilgisiz bir hata icin ipucu donmemeliydi, gelen: %q", got)
	}

	if got := pcapErrHint(nil); got != "" {
		t.Fatalf("nil hata icin ipucu donmemeliydi, gelen: %q", got)
	}
}
