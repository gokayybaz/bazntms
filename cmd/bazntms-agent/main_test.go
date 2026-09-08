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

// TestDeepCollectMethod, v1.3.0 varsayilanini kilitler: derin toplama
// config/flag olmadan da acik; yalnizca method: off kapatir.
func TestDeepCollectMethod(t *testing.T) {
	cases := []struct {
		name, flag, cfg, wantMethod string
		wantWant                    bool
	}{
		{"config yok, flag yok -> auto+acik", "", "", "auto", true},
		{"config auto", "", "auto", "auto", true},
		{"config pcap", "", "pcap", "pcap", true},
		{"config ebpf", "", "ebpf", "ebpf", true},
		{"config off -> kapali", "", "off", "off", false},
		{"flag off config auto -> flag kazanir", "off", "auto", "off", false},
		{"flag pcap config off -> flag kazanir", "pcap", "off", "pcap", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, w := deepCollectMethod(tc.flag, tc.cfg)
			if m != tc.wantMethod || w != tc.wantWant {
				t.Fatalf("deepCollectMethod(%q,%q) = (%q,%v); beklenen (%q,%v)",
					tc.flag, tc.cfg, m, w, tc.wantMethod, tc.wantWant)
			}
		})
	}
}
