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

// TestIsSanalIface, auto arayuz seciminin sanal/VPN adaptorlerini (Tailscale
// vakasi) ele alip fiziksel NIC adlarina dokunmadigini dogrular.
func TestIsSanalIface(t *testing.T) {
	sanal := []string{
		"Tailscale", "tailscale0", "wg0", "WireGuard Tunnel", "utun4", "tun0",
		"tap0", "ztyugelu6b", "vEthernet (WSL)", "docker0", "br-1a2b3c",
		"veth9f3", "VMware Network Adapter VMnet8", "ppp0", "Npcap Loopback Adapter",
	}
	for _, n := range sanal {
		if !isSanalIface(n) {
			t.Errorf("isSanalIface(%q) = false; sanal olmaliydi", n)
		}
	}
	fiziksel := []string{"eth0", "en0", "Ethernet", "Ethernet 2", "Wi-Fi", "wlan0", "enp3s0", "Local Area Connection"}
	for _, n := range fiziksel {
		if isSanalIface(n) {
			t.Errorf("isSanalIface(%q) = true; fiziksel olmaliydi", n)
		}
	}
}

// TestHubHostPort, hub URL'inden dogru "host:port"un cikarildigini dogrular
// (autoIface varsayilan-rota probu bu hedefe dial eder).
func TestHubHostPort(t *testing.T) {
	cases := map[string]string{
		"https://hub.example.com":      "hub.example.com:443",
		"http://hub.example.com":       "hub.example.com:80",
		"https://hub.example.com:8443": "hub.example.com:8443",
		"http://10.0.0.5:8080":         "10.0.0.5:8080",
		"":                             "",
		"://bozuk":                     "",
		"not a url":                    "",
	}
	for in, want := range cases {
		if got := hubHostPort(in); got != want {
			t.Errorf("hubHostPort(%q) = %q; beklenen %q", in, got, want)
		}
	}
}

// TestKisalt, attr_note kisaltmasinin rune sinirinda calistigini dogrular.
func TestKisalt(t *testing.T) {
	if got := kisalt("kisa", 10); got != "kisa" {
		t.Errorf("kisalt kisa metni degistirdi: %q", got)
	}
	if got := kisalt("abcdefghij", 5); got != "abcde…" {
		t.Errorf("kisalt(_,5) = %q; beklenen %q", got, "abcde…")
	}
}
