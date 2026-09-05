package proctraffic

import "testing"

// Non-root'ta lsof yalnizca kendi sureclerini gosterir; test, saglayicinin
// calistigini ve makul esleme dondurdugunu dogrular.
func TestDarwinSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("short mod")
	}
	p := NewProvider()
	snap := p.Snapshot()
	if len(snap) == 0 {
		t.Skip("lsof bos dondu (ortam sinirli)")
	}
	var withPID int
	for _, pi := range snap {
		if pi.PID > 0 && pi.Process != "" {
			withPID++
		}
	}
	if withPID == 0 {
		t.Fatal("hicbir baglanti surec eslesmedi")
	}
	// Regresyon: Key.Proto pcap tarafiyla ayni sozlukte olmali ("tcp"/"udp").
	// lsof'un kucuk 't' alani IPv4/IPv6 dondurur; yanlislikla o kullanilirsa
	// atif tablosu hicbir pakete eslesmez ve surec/L7/DNS gorunurlugu bos kalir.
	for k := range snap {
		if k.Proto != "tcp" && k.Proto != "udp" {
			t.Fatalf("beklenmeyen protokol %q — lsof 'P' alani (TCP/UDP) kullanilmali", k.Proto)
		}
	}
	t.Logf("esleme: %d anahtar, %d surec eslesmis", len(snap), withPID)
}

// TestParseLsofFields, gercek lsof -F pcnPf ciktisinin alan sirasini
// (p/c, ardindan her dosya icin f/P/n) hermetik olarak dogrular.
func TestParseLsofFields(t *testing.T) {
	fixture := []byte(`p624
crapportd
f10
PTCP
n*:55578
f17
PTCP
n[fe80:e::144a:40b2:cd68:328f]:55578->[fe80:e::1cdd:e93c:d5bc:fb67]:49622
f20
PUDP
n*:3722
p901
cmDNSResponder
f31
PUDP
n192.168.1.20:54321->192.168.1.1:53
`)
	got := parseLsof(fixture)

	tcp := Key{Proto: "tcp", LocalIP: "fe80:e::144a:40b2:cd68:328f", LocalPort: 55578,
		RemoteIP: "fe80:e::1cdd:e93c:d5bc:fb67", RemotePort: 49622}
	if pi := got[tcp]; pi.PID != 624 || pi.Process != "rapportd" {
		t.Errorf("TCP baglantisi atfedilemedi: %+v", pi)
	}
	udp := Key{Proto: "udp", LocalIP: "192.168.1.20", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 53}
	if pi := got[udp]; pi.PID != 901 || pi.Process != "mDNSResponder" {
		t.Errorf("UDP/53 baglantisi atfedilemedi: %+v", pi)
	}
	// ters yon de bulunmali (paket src/dst sirasi onemsiz)
	rev := Key{Proto: "udp", LocalIP: "192.168.1.1", LocalPort: 53, RemoteIP: "192.168.1.20", RemotePort: 54321}
	if got[rev].PID != 901 {
		t.Error("ters yonlu anahtar eksik")
	}
	// dinleme soketleri ("*:3722", "->" yok) atfedilemez, tabloya girmemeli
	if len(got) != 4 {
		t.Errorf("beklenen 4 anahtar (2 baglanti x 2 yon), bulunan %d", len(got))
	}
}
