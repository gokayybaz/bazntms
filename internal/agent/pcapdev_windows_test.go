//go:build windows

package agent

import "testing"

// TestResolvePcapDeviceWindows, Windows'a ozel arayuz-adi cozumlemesinin
// koruma yollarini dogrular (gercek Npcap gerektirmeden: NPF adi ve bos ad
// kisa devre eder, bilinmeyen friendly ad ham hali + hata ile doner).
func TestResolvePcapDeviceWindows(t *testing.T) {
	npf := `\Device\NPF_{00000000-0000-0000-0000-000000000000}`
	if got, err := ResolvePcapDevice(npf); got != npf || err != nil {
		t.Fatalf("zaten NPF cihaz adi degismeden donmeliydi: %q %v", got, err)
	}
	if got, err := ResolvePcapDevice(""); got != "" || err != nil {
		t.Fatalf("bos ad bos donmeliydi: %q %v", got, err)
	}
	if got, err := ResolvePcapDevice("BoyleBirArayuzYok"); err == nil || got != "BoyleBirArayuzYok" {
		t.Fatalf("bilinmeyen arayuz ham ad + hata donmeliydi: %q %v", got, err)
	}
}
