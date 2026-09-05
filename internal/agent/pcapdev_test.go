//go:build !windows

package agent

import "testing"

// TestResolvePcapDeviceUnixNoop, Linux/macOS'ta net.Interface.Name zaten pcap
// cihaz adi oldugu icin ResolvePcapDevice'in adi degistirmeden dondurdugunu
// dogrular (Windows'a ozel \Device\NPF_ cevirisi burada devreye girmemeli).
func TestResolvePcapDeviceUnixNoop(t *testing.T) {
	for _, name := range []string{"", "eth0", "en0", "auto", `\Device\NPF_x`} {
		got, err := ResolvePcapDevice(name)
		if err != nil || got != name {
			t.Fatalf("ResolvePcapDevice(%q) = %q, %v; degismeden donmeliydi", name, got, err)
		}
	}
}
