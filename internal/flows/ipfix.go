package flows

import (
	"encoding/binary"
	"time"
)

// ParseIPFIX, IPFIX / NetFlow v10 (RFC 7011) mesajini cozer. NetFlow v9'un
// temizlenmis surumudur: header 16 bayt ve TOPLAM mesaj uzunlugunu tasir,
// zaman "export time" (unix sn), gozlem alani obsDomainID. Set'ler:
//
//	setID 2      → template set
//	setID 3      → options template set (atlanir)
//	setID >= 256 → data set (setID == templateID)
//
// Template alanlarinda enterprise-bit (0x8000) varsa 4 bayt enterprise number
// takip eder — parseTemplate(ipfix=true) bunu atlar.
func ParseIPFIX(cache *TemplateCache, payload []byte, device, exporterKey string, receivedAt time.Time) []Row {
	if len(payload) < 16 || binary.BigEndian.Uint16(payload[0:2]) != 10 {
		return nil
	}
	msgLen := int(binary.BigEndian.Uint16(payload[2:4]))
	if msgLen < 16 || msgLen > len(payload) {
		msgLen = len(payload)
	}
	exportTime := binary.BigEndian.Uint32(payload[4:8])
	domain := binary.BigEndian.Uint32(payload[12:16])

	var rows []Row
	off := 16
	for off+4 <= msgLen {
		setID := binary.BigEndian.Uint16(payload[off : off+2])
		setLen := int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		if setLen < 4 || off+setLen > msgLen {
			break
		}
		body := payload[off+4 : off+setLen]
		off += setLen

		switch {
		case setID == 2: // template set
			b := body
			for len(b) >= 4 {
				id, t, consumed, ok := parseTemplate(b, true)
				if !ok {
					break
				}
				cache.put(templateKey{exporter: exporterKey, domain: domain, tmpl: id}, t)
				b = b[consumed:]
			}
		case setID == 3: // options template — atla
		case setID >= 256:
			t, ok := cache.get(templateKey{exporter: exporterKey, domain: domain, tmpl: setID})
			if !ok || t.recLen <= 0 {
				continue
			}
			for r := 0; r+t.recLen <= len(body); r += t.recLen {
				if row, ok := decodeRecord(t, body[r:r+t.recLen], device, exportTime, receivedAt); ok {
					rows = append(rows, row)
				}
			}
		}
	}
	return rows
}
