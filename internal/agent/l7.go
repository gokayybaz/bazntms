package agent

import (
	"bytes"
	"encoding/binary"
	"strings"
	"sync"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// l7Tracker, giden TCP payload'larından çıkarılan SNI/Host adlarını süreçlere
// atfeder ve delta üretir. Kendi kilidi vardır; pcapAttrSource (tam yakalama)
// ve ebpfAttrSource (dar-filtreli yardımcı pcap handle — S20.7) ortak kullanır.
type l7Tracker struct {
	mu   sync.Mutex
	agg  map[l7Key]*l7Agg
	sent map[l7Key]uint64
}

func newL7Tracker() *l7Tracker {
	return &l7Tracker{agg: map[l7Key]*l7Agg{}, sent: map[l7Key]uint64{}}
}

// observe, giden bir TCP payload'ında SNI/Host varsa sürece atfeder.
func (t *l7Tracker) observe(pid int32, process, remoteIP string, payload []byte, length uint64) {
	if host, kind := sniffL7(payload); host != "" {
		t.record(pid, process, remoteIP, host, kind, length)
	}
}

// record, önceden çıkarılmış bir (host, kind)'i sürece atfeder — çağıran
// tarafın payload'ı bir kez ayrıştırdığı yerlerde (l7Helper) kullanılır.
func (t *l7Tracker) record(pid int32, process, remoteIP, host, kind string, length uint64) {
	k := l7Key{pid: pid, process: process, kind: kind, host: host, remoteIP: remoteIP}
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.agg[k]
	if a == nil {
		if len(t.agg) >= 4000 {
			return
		}
		a = &l7Agg{}
		t.agg[k] = a
	}
	a.bytes += length
	a.count++
}

// deltas, son çağrıdan bu yana süreç bazlı SNI/Host farklarını döndürür.
func (t *l7Tracker) deltas() []telemetry.L7Sample {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]telemetry.L7Sample, 0, len(t.agg))
	for k, a := range t.agg {
		d := delta(a.count, t.sent[k])
		if d == 0 {
			continue
		}
		t.sent[k] = a.count
		out = append(out, telemetry.L7Sample{
			PID:      k.pid,
			Process:  k.process,
			Kind:     k.kind,
			Host:     k.host,
			RemoteIP: k.remoteIP,
			Bytes:    a.bytes,
			Count:    d,
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

// sniffL7, giden bir TCP payload'inda TLS ClientHello SNI'sini veya HTTP istek
// satirindaki Host'u arar. Bulursa (host, "tls"|"http") doner. Basit ve
// yanilma payi dusuk tutulur — imza tabanli L7 DPI degil, yalnizca acikca
// gorunen alan adi.
func sniffL7(p []byte) (host, kind string) {
	if h := tlsSNI(p); h != "" {
		return h, "tls"
	}
	if h := httpHost(p); h != "" {
		return h, "http"
	}
	return "", ""
}

// tlsSNI, bir TLS kaydinin (record type 22 = handshake, handshake type 1 =
// ClientHello) server_name uzantisini cozer. Yakalama snaplen'i (attrSnapLen)
// ClientHello'yu kesebilir — o durumda bos doner (sorun degil, sonraki
// baglantida yakalanir).
func tlsSNI(p []byte) string {
	// TLS record: type(1)=22, version(2), length(2)
	if len(p) < 43 || p[0] != 0x16 {
		return ""
	}
	rec := p[5:]
	// Handshake: type(1)=1, length(3), version(2), random(32), sessionIDLen(1)...
	if len(rec) < 38 || rec[0] != 0x01 {
		return ""
	}
	hs := rec[4:] // handshake body
	off := 2 + 32 // client_version + random
	if off >= len(hs) {
		return ""
	}
	sidLen := int(hs[off])
	off += 1 + sidLen
	if off+2 > len(hs) {
		return ""
	}
	csLen := int(binary.BigEndian.Uint16(hs[off : off+2]))
	off += 2 + csLen
	if off+1 > len(hs) {
		return ""
	}
	compLen := int(hs[off])
	off += 1 + compLen
	if off+2 > len(hs) {
		return ""
	}
	extTotal := int(binary.BigEndian.Uint16(hs[off : off+2]))
	off += 2
	end := off + extTotal
	if end > len(hs) {
		end = len(hs)
	}
	for off+4 <= end {
		extType := binary.BigEndian.Uint16(hs[off : off+2])
		extLen := int(binary.BigEndian.Uint16(hs[off+2 : off+4]))
		off += 4
		if off+extLen > len(hs) {
			return ""
		}
		if extType == 0x0000 { // server_name
			sn := hs[off : off+extLen]
			// server_name_list: listLen(2), nameType(1)=0, nameLen(2), name
			if len(sn) < 5 || sn[2] != 0x00 {
				return ""
			}
			nameLen := int(binary.BigEndian.Uint16(sn[3:5]))
			if 5+nameLen > len(sn) {
				return ""
			}
			return sanitizeHost(string(sn[5 : 5+nameLen]))
		}
		off += extLen
	}
	return ""
}

var httpMethods = [][]byte{
	[]byte("GET "), []byte("POST "), []byte("PUT "), []byte("HEAD "),
	[]byte("DELETE "), []byte("PATCH "), []byte("OPTIONS "), []byte("CONNECT "),
}

// httpHost, bir HTTP/1.x istek payload'inda "Host:" satirini bulur.
// CONNECT metodunda hedef istek satirindadir (proxy).
func httpHost(p []byte) string {
	isReq := false
	for _, m := range httpMethods {
		if bytes.HasPrefix(p, m) {
			isReq = true
			break
		}
	}
	if !isReq {
		return ""
	}
	head := p
	if i := bytes.Index(p, []byte("\r\n\r\n")); i >= 0 {
		head = p[:i]
	}
	for _, line := range bytes.Split(head, []byte("\r\n")) {
		if len(line) > 5 && (bytes.HasPrefix(bytes.ToLower(line), []byte("host:"))) {
			return sanitizeHost(strings.TrimSpace(string(line[5:])))
		}
	}
	return ""
}

// sanitizeHost, port'u ayirir, kucuk harfe cevirir, kaba dogrulama yapar.
func sanitizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if i := strings.LastIndexByte(h, ':'); i > 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	h = strings.Trim(h, "[]")
	if h == "" || len(h) > 253 || strings.ContainsAny(h, " \t\r\n/\\") {
		return ""
	}
	// en az bir nokta veya ':' (ipv6) — cip lak IP/host de kabul
	if !strings.ContainsAny(h, ".:") {
		return ""
	}
	return h
}

// L7Deltas, son gonderimden bu yana surec bazli uygulama (SNI/Host) farklarini
// dondurur.
func (e *pcapAttrSource) L7Deltas() []telemetry.L7Sample { return e.l7.deltas() }
