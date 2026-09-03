package flows

import (
	"encoding/binary"
	"time"
)

// ParseV9, NetFlow v9 (RFC 3954) paketini cozer. Sablonlar cache'e yazilir;
// veri kayitlari icin sablon yoksa (henuz gelmemis) o flowset atlanir —
// exporter sablonu yeniledigi zaman cozulmeye baslar.
//
// Header (20 bayt): version(2)=9, count(2)=flowset sayisi (kayit degil),
// sysUptime(4), unixSecs(4), seqNum(4), sourceID(4). Ardindan flowset'ler:
// flowsetID(2) + length(2) + govde.
//
//	flowsetID 0      → template flowset
//	flowsetID 1      → options template (atlanir)
//	flowsetID >= 256 → data flowset (flowsetID == templateID)
func ParseV9(cache *TemplateCache, payload []byte, device, exporterKey string, receivedAt time.Time) []Row {
	if len(payload) < 20 || binary.BigEndian.Uint16(payload[0:2]) != 9 {
		return nil
	}
	unixSecs := binary.BigEndian.Uint32(payload[8:12])
	sourceID := binary.BigEndian.Uint32(payload[16:20])

	var rows []Row
	off := 20
	for off+4 <= len(payload) {
		fsID := binary.BigEndian.Uint16(payload[off : off+2])
		fsLen := int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		if fsLen < 4 || off+fsLen > len(payload) {
			break
		}
		body := payload[off+4 : off+fsLen]
		off += fsLen

		switch {
		case fsID == 0: // template flowset
			b := body
			for len(b) >= 4 {
				id, t, consumed, ok := parseTemplate(b, false)
				if !ok {
					break
				}
				cache.put(templateKey{exporter: exporterKey, domain: sourceID, tmpl: id}, t)
				b = b[consumed:]
			}
		case fsID == 1: // options template — atla
		case fsID >= 256:
			t, ok := cache.get(templateKey{exporter: exporterKey, domain: sourceID, tmpl: fsID})
			if !ok || t.recLen <= 0 {
				continue
			}
			for r := 0; r+t.recLen <= len(body); r += t.recLen {
				if row, ok := decodeRecord(t, body[r:r+t.recLen], device, unixSecs, receivedAt); ok {
					rows = append(rows, row)
				}
			}
		}
	}
	return rows
}
