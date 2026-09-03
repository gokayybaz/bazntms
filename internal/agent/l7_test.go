package agent

import (
	"encoding/binary"
	"testing"
)

// clientHelloWithSNI, verilen SNI ile minimal ama gecerli bir TLS
// ClientHello record'u uretir.
func clientHelloWithSNI(sni string) []byte {
	// server_name extension body
	name := []byte(sni)
	sniExt := []byte{0x00, 0x00} // extension type 0
	body := make([]byte, 0, len(name)+5)
	body = append(body, 0x00, byte(len(name)+3)) // server_name_list length
	body = append(body, 0x00)                    // name type host_name
	body = binary.BigEndian.AppendUint16(body, uint16(len(name)))
	body = append(body, name...)
	sniExt = binary.BigEndian.AppendUint16(sniExt, uint16(len(body)))
	sniExt = append(sniExt, body...)

	// handshake body: version(2)+random(32)+sidLen(1)+csLen(2)+cs(2)+compLen(1)+comp(1)+extLen(2)+exts
	hs := make([]byte, 0, 64+len(sniExt))
	hs = append(hs, 0x03, 0x03)             // client_version TLS1.2
	hs = append(hs, make([]byte, 32)...)    // random
	hs = append(hs, 0x00)                   // session id len 0
	hs = append(hs, 0x00, 0x02, 0x13, 0x01) // cipher suites len 2 + one suite
	hs = append(hs, 0x01, 0x00)             // compression: len 1, null
	hs = binary.BigEndian.AppendUint16(hs, uint16(len(sniExt)))
	hs = append(hs, sniExt...)

	// handshake header: type(1)=1 + length(3)
	hsFull := []byte{0x01, byte(len(hs) >> 16), byte(len(hs) >> 8), byte(len(hs))}
	hsFull = append(hsFull, hs...)

	// TLS record: type(1)=22 + version(2) + length(2)
	rec := []byte{0x16, 0x03, 0x01, byte(len(hsFull) >> 8), byte(len(hsFull))}
	return append(rec, hsFull...)
}

func TestTLSSNI(t *testing.T) {
	host, kind := sniffL7(clientHelloWithSNI("api.github.com"))
	if host != "api.github.com" || kind != "tls" {
		t.Fatalf("SNI cozulemedi: host=%q kind=%q", host, kind)
	}
	// port'lu / buyuk harfli → normalize
	if host, _ := sniffL7(clientHelloWithSNI("CDN.Example.COM")); host != "cdn.example.com" {
		t.Fatalf("SNI normalize edilmedi: %q", host)
	}
	// kesik ClientHello (snaplen) → bos, panik yok
	full := clientHelloWithSNI("truncated.example.org")
	if host, _ := sniffL7(full[:20]); host != "" {
		t.Fatalf("kesik ClientHello'dan host cikmamali: %q", host)
	}
}

func TestHTTPHost(t *testing.T) {
	req := []byte("GET /index.html HTTP/1.1\r\nHost: www.example.net\r\nUser-Agent: x\r\n\r\n")
	host, kind := sniffL7(req)
	if host != "www.example.net" || kind != "http" {
		t.Fatalf("HTTP Host cozulemedi: host=%q kind=%q", host, kind)
	}
	// buyuk/kucuk harf duyarsiz header adi + port
	req2 := []byte("POST /x HTTP/1.1\r\nhost: Shop.Example.com:8080\r\n\r\n")
	if host, _ := sniffL7(req2); host != "shop.example.com" {
		t.Fatalf("Host normalize edilmedi: %q", host)
	}
	// HTTP olmayan payload → bos
	if host, _ := sniffL7([]byte("\x17\x03\x03random tls app data")); host != "" {
		t.Fatalf("rastgele veriden host cikmamali: %q", host)
	}
}

func TestSanitizeHostRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "localhost", "no-dot", "has space.com", "a/b.com", string(make([]byte, 300))} {
		if h := sanitizeHost(bad); h != "" {
			t.Fatalf("gecersiz host kabul edildi: %q → %q", bad, h)
		}
	}
	if sanitizeHost("8.8.8.8") == "" {
		t.Fatal("cip lak IPv4 host reddedilmemeli")
	}
}
