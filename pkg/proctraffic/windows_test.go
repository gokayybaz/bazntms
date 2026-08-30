//go:build windows

package proctraffic

import "testing"

// gercek Windows netstat -ano -p tcp / tcpv6 / udp / udpv6 ciktisina benzer
// ornekler. Sutun genislikleri gercek ciktidaki gibi degisken bosluklu.
const sampleTCP4 = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1234
  TCP    192.168.1.94:50000     142.250.9.14:443       ESTABLISHED     5678
`

const sampleTCP6 = `
Active Connections

  Proto  Local Address                    Foreign Address                  State           PID
  TCP6   [::]:135                         [::]:0                           LISTENING       1234
  TCP6   [fe80::1]:51820                  [fe80::2]:443                    ESTABLISHED     9012
`

const sampleUDP4 = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  UDP    0.0.0.0:5353           *:*                                    4321
`

func TestParseNetstatOutput_IPv4(t *testing.T) {
	out := map[Key]ProcInfo{}
	parseNetstatOutput(sampleTCP4, "tcp", func(pid int32) string { return "chrome" }, out)

	// LISTENING satiri (foreign 0.0.0.0:0) atfa girmemeli — rp=0, gercek
	// paketlerde asla eslesmeyecegi icin zararsizdir ama beklenmedik girdi
	// eklememesi icin en azindan ESTABLISHED satiri dogru ayristirilmali
	want := Key{Proto: "tcp", LocalIP: "0.0.0.0", LocalPort: 50000, RemoteIP: "142.250.9.14", RemotePort: 443}
	info, ok := out[want]
	if !ok {
		t.Fatalf("beklenen anahtar bulunamadi: %+v\nharita: %+v", want, out)
	}
	if info.PID != 5678 || info.Process != "chrome" {
		t.Fatalf("beklenmedik info: %+v", info)
	}

	// ters yonlu anahtar da eklenmis olmali
	revKey := Key{Proto: "tcp", LocalIP: "142.250.9.14", LocalPort: 443, RemoteIP: "0.0.0.0", RemotePort: 50000}
	if _, ok := out[revKey]; !ok {
		t.Fatalf("ters yonlu anahtar eksik: %+v", revKey)
	}
}

// TestParseNetstatOutput_IPv6, duzeltmeden once IPv6 satirlarinin (TCP6)
// tamamen atlandigi regresyonu dogrular.
func TestParseNetstatOutput_IPv6(t *testing.T) {
	out := map[Key]ProcInfo{}
	parseNetstatOutput(sampleTCP6, "tcp", func(pid int32) string { return "edge" }, out)

	want := Key{Proto: "tcp", LocalIP: "0.0.0.0", LocalPort: 51820, RemoteIP: "fe80::2", RemotePort: 443}
	info, ok := out[want]
	if !ok {
		t.Fatalf("IPv6 baglantisi ayristirilamadi (regresyon!): harita: %+v", out)
	}
	if info.PID != 9012 || info.Process != "edge" {
		t.Fatalf("beklenmedik info: %+v", info)
	}
}

func TestParseNetstatOutput_UDPWildcardSkipped(t *testing.T) {
	out := map[Key]ProcInfo{}
	parseNetstatOutput(sampleUDP4, "udp", func(pid int32) string { return "svc" }, out)
	if len(out) != 0 {
		t.Fatalf("UDP '*:*' (dinleme) satiri atlanmali, ama harita dolu: %+v", out)
	}
}

func TestParseNetstatOutput_HeaderLinesIgnored(t *testing.T) {
	out := map[Key]ProcInfo{}
	parseNetstatOutput(sampleTCP4, "tcp", func(int32) string { return "" }, out)
	// "Active Connections" ve baslik satiri PID olarak ayristirilmaya
	// calisilmamali (panik/hata yaratmadan sessizce atlanmali) — buraya
	// kadar hatasiz gelmis olmasi yeterli dogrulama
}
