package flows

import (
	"encoding/binary"
	"net"
	"sync"
	"time"
)

// NetFlow v9 (RFC 3954) ve IPFIX / NetFlow v10 (RFC 7011) sablon tabanli
// akis protokolleridir: exporter once bir "template" (alan tipi + uzunluk
// listesi) gonderir, sonraki veri kayitlari o sablona gore paketlenir. Sablon
// periyodik yenilenir; alici kaybederse veri kayitlarini cozemez. Bu yuzden
// sablonlar (exporter, gozlem alani, template ID) uzerinden onbelleklenir.

// IANA IPFIX Information Element numaralari (v9 alan tipleriyle ortak alt kume).
const (
	ieOctetDeltaCount        = 1
	iePacketDeltaCount       = 2
	ieProtocolIdentifier     = 4
	ieSourceTransportPort    = 7
	ieSourceIPv4Address      = 8
	ieDestinationTransport   = 11
	ieDestinationIPv4Address = 12
	ieSourceIPv6Address      = 27
	ieDestinationIPv6Address = 28
	ieFlowEndSysUpTime       = 21
	ieFlowStartSysUpTime     = 22
	ieFlowEndSeconds         = 151
	ieFlowEndMilliseconds    = 153
	ieOctetTotalCount        = 85
	iePacketTotalCount       = 86
)

type templateKey struct {
	exporter string
	domain   uint32
	tmpl     uint16
}

type tmplField struct {
	typ    uint16
	length uint16 // 0xFFFF = degisken uzunluk (desteklenmez; kayit atlanir)
}

type template struct {
	fields  []tmplField
	recLen  int // sabit kayit uzunlugu (degisken uzunluklu alan varsa -1)
	created time.Time
}

// TemplateCache, exporter'lardan gelen NetFlow v9 / IPFIX sablonlarini tutar.
// Es zamanli erisim guvenli. Bayat sablonlar (24 saat gorulmeyen) periyodik
// temizlenmez — exporter yeniden gonderdiginde uzerine yazilir, sayilari
// sinirlidir (exporter × template ID).
type TemplateCache struct {
	mu sync.Mutex
	m  map[templateKey]template
}

// NewTemplateCache, bos bir sablon onbellegi olusturur.
func NewTemplateCache() *TemplateCache { return &TemplateCache{m: map[templateKey]template{}} }

func (c *TemplateCache) put(k templateKey, t template) {
	c.mu.Lock()
	c.m[k] = t
	c.mu.Unlock()
}

func (c *TemplateCache) get(k templateKey) (template, bool) {
	c.mu.Lock()
	t, ok := c.m[k]
	c.mu.Unlock()
	return t, ok
}

// parseTemplate, bir template set/flowset govdesindeki sablonlari cozer.
// ipfix true ise enterprise-bit'li alanlarda 4 bayt enterprise number atlanir.
func parseTemplate(body []byte, ipfix bool) (id uint16, t template, consumed int, ok bool) {
	if len(body) < 4 {
		return 0, template{}, 0, false
	}
	id = binary.BigEndian.Uint16(body[0:2])
	fieldCount := int(binary.BigEndian.Uint16(body[2:4]))
	off := 4
	t.recLen = 0
	for i := 0; i < fieldCount; i++ {
		if off+4 > len(body) {
			return 0, template{}, 0, false
		}
		typ := binary.BigEndian.Uint16(body[off : off+2])
		length := binary.BigEndian.Uint16(body[off+2 : off+4])
		off += 4
		if ipfix && typ&0x8000 != 0 {
			typ &= 0x7FFF
			off += 4 // enterprise number
			if off > len(body) {
				return 0, template{}, 0, false
			}
		}
		t.fields = append(t.fields, tmplField{typ: typ, length: length})
		if length == 0xFFFF {
			t.recLen = -1
		} else if t.recLen >= 0 {
			t.recLen += int(length)
		}
	}
	t.created = time.Now()
	return id, t, off, true
}

// decodeRecord, bir veri kaydini sablona gore Row'a cozer. baseUnix, exporter
// paket zamani (unix sn); v9 sysUptime tabanli zaman alanlari icin
// receivedAt'e duser (dt hesabi ParseV5'teki kadar hassas degil — v9/IPFIX
// exporter'lari cogunlukla milisaniye/saniye mutlak zaman da gonderir).
func decodeRecord(t template, rec []byte, device string, baseUnix uint32, receivedAt time.Time) (Row, bool) {
	r := Row{Device: device, Ts: receivedAt.Unix()}
	off := 0
	var haveEndpoints bool
	for _, f := range t.fields {
		if f.length == 0xFFFF || off+int(f.length) > len(rec) {
			return Row{}, false
		}
		v := rec[off : off+int(f.length)]
		off += int(f.length)
		switch f.typ {
		case ieSourceIPv4Address:
			if len(v) == 4 {
				r.Src = net.IP(v).String()
				haveEndpoints = true
			}
		case ieDestinationIPv4Address:
			if len(v) == 4 {
				r.Dst = net.IP(v).String()
			}
		case ieSourceIPv6Address:
			if len(v) == 16 {
				r.Src = net.IP(v).String()
				haveEndpoints = true
			}
		case ieDestinationIPv6Address:
			if len(v) == 16 {
				r.Dst = net.IP(v).String()
			}
		case ieSourceTransportPort:
			r.SrcPort = uint16(beUint(v))
		case ieDestinationTransport:
			r.DstPort = uint16(beUint(v))
		case ieProtocolIdentifier:
			if len(v) >= 1 {
				r.Proto = protoName(v[len(v)-1])
			}
		case ieOctetDeltaCount, ieOctetTotalCount:
			r.Octets = beUint(v)
		case iePacketDeltaCount, iePacketTotalCount:
			r.Packets = beUint(v)
		case ieFlowEndSeconds:
			if s := beUint(v); s > 0 {
				r.Ts = int64(s)
			}
		case ieFlowEndMilliseconds:
			if ms := beUint(v); ms > 0 {
				r.Ts = int64(ms / 1000)
			}
		}
	}
	_ = baseUnix
	if !haveEndpoints || r.Src == "" || r.Dst == "" {
		return Row{}, false
	}
	// akla yatkin olmayan zaman → alim zamani
	if r.Ts <= 0 || r.Ts > receivedAt.Unix()+300 || r.Ts < receivedAt.Add(-7*24*time.Hour).Unix() {
		r.Ts = receivedAt.Unix()
	}
	if r.Proto == "" {
		r.Proto = "proto-0"
	}
	return r, true
}

// beUint, 1-8 baytlik big-endian isaretsiz tamsayiyi cozer.
func beUint(b []byte) uint64 {
	var n uint64
	for _, x := range b {
		n = n<<8 | uint64(x)
	}
	return n
}
