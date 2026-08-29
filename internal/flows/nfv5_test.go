package flows

import (
	"encoding/binary"
	"testing"
	"time"
)

// NetFlow v5 paketini el ile uretip parser'i dogrular (router simulasyonu).
func TestParseV5(t *testing.T) {
	hdr := make([]byte, 24)
	binary.BigEndian.PutUint16(hdr[0:2], 5)
	binary.BigEndian.PutUint16(hdr[2:4], 1) // 1 akis
	binary.BigEndian.PutUint32(hdr[4:8], 100000)
	binary.BigEndian.PutUint32(hdr[8:12], uint32(time.Now().Unix()))

	rec := make([]byte, 48)
	copy(rec[0:4], []byte{192, 168, 1, 50})   // src 192.168.1.50
	copy(rec[4:8], []byte{142, 250, 74, 100}) // dst 142.250.74.100
	binary.BigEndian.PutUint32(rec[16:20], 120)
	binary.BigEndian.PutUint32(rec[20:24], 95000)
	binary.BigEndian.PutUint16(rec[32:34], 51000)
	binary.BigEndian.PutUint16(rec[34:36], 443)
	rec[38] = 6 // tcp

	payload := append(hdr, rec...)
	rows := ParseV5(payload, "10.0.0.1", time.Now())
	if len(rows) != 1 {
		t.Fatalf("1 akis beklenirdi: %d", len(rows))
	}
	r := rows[0]
	if r.Src != "192.168.1.50" || r.Dst != "142.250.74.100" {
		t.Fatalf("adresler hatali: %+v", r)
	}
	if r.SrcPort != 51000 || r.DstPort != 443 || r.Proto != "tcp" {
		t.Fatalf("port/protokol hatali: %+v", r)
	}
	if r.Packets != 120 || r.Octets != 95000 {
		t.Fatalf("sayac hatali: %+v", r)
	}
	if r.Device != "10.0.0.1" {
		t.Fatalf("kaynak cihaz hatali: %s", r.Device)
	}
}

func TestParseV5RejectsBadInput(t *testing.T) {
	if rows := ParseV5([]byte{1, 2, 3}, "x", time.Now()); rows != nil {
		t.Fatal("kisa paket reddedilmeliydi")
	}
	bad := make([]byte, 60)
	binary.BigEndian.PutUint16(bad[0:2], 9) // v9: v5 parser'i reddetmeli
	if rows := ParseV5(bad, "x", time.Now()); rows != nil {
		t.Fatal("v9 paket v5 parser'da red edilmeliydi")
	}
}
