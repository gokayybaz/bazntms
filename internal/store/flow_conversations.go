package store

import (
	"sort"
	"time"
)

// --- NetFlow konuşma toplama (Faz 23-B) ---
//
// Ham `flows` kayıtları (tek yönlü NetFlow/IPFIX/sFlow) bir zaman penceresinde
// konuşmalara toplanır: 5'li (src,dst,src_port,dst_port,proto) ya da uç-çifti
// (src ↔ dst, yön birleştirilmiş). Sunucu-tarafı GROUP BY — tarayıcıya ham veri
// gitmez. Yeni cagg YOK (src/dst yüksek kardinalite — pg.go'daki flows_1h notu):
// konuşma görünümü ham `flows`'un retention penceresiyle (vars. 7g) sınırlı.

// FlowConversation, toplanmış tek bir konuşma.
type FlowConversation struct {
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	SrcPort   uint16 `json:"src_port,omitempty"` // yalnız by=5tuple
	DstPort   uint16 `json:"dst_port,omitempty"`
	Proto     string `json:"proto,omitempty"`
	Flows     uint64 `json:"flows"`
	Packets   uint64 `json:"packets"`
	Octets    uint64 `json:"octets"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
}

var flowConvoSort = map[string]string{
	"octets": "octets", "packets": "packets", "flows": "flows", "last_seen": "last_seen",
}

// flowConvoPairCap, uç-çifti modunda yön birleştirmesi öncesi SQL'den çekilen
// azami (src,dst) grubu. Birleştirme Go tarafında; bu sınır 50k flow/sn
// ölçeğinde GROUP BY sonucunu sınırlar (ikisi de sınır dışında kalıp birleşince
// üste çıkacak bir çift pratikte yok — büyük toplam → en az bir yön büyük).
const flowConvoPairCap = 2000

// FlowConversations, `since`'ten beri akışları toplar. by="5tuple" →
// (src,dst,src_port,dst_port,proto); aksi → uç-çifti (A→B ve B→A tek satır,
// Src = sözlüksel küçük uç). sortBy ∈ octets|packets|flows|last_seen (vars.
// octets). site boş değilse yalnız o siteye kayıtlı exporter'ın akışları
// (TopFlows ile aynı kapsam).
func (s *sqlStore) FlowConversations(since time.Time, by, sortBy string, limit int, site string) ([]FlowConversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	orderCol := flowConvoSort[sortBy]
	if orderCol == "" {
		orderCol = "octets"
	}
	fiveTuple := by == "5tuple"
	groupCols := "src, dst"
	if fiveTuple {
		groupCols = "src, dst, src_port, dst_port, proto"
	}

	q := `SELECT ` + groupCols + `,
		COUNT(*) AS flows, SUM(packets) AS packets, SUM(octets) AS octets,
		MIN(ts) AS first_seen, MAX(ts) AS last_seen
		FROM flows WHERE ts >= ? AND src <> '' AND dst <> ''`
	args := []any{since.Unix()}
	if site != "" {
		q += ` AND device IN (SELECT host FROM devices WHERE site = ?)`
		args = append(args, site)
	}
	q += ` GROUP BY ` + groupCols + ` ORDER BY ` + orderCol + ` DESC LIMIT ?`
	if fiveTuple {
		args = append(args, limit)
	} else {
		args = append(args, flowConvoPairCap)
	}

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var raw []FlowConversation
	for rows.Next() {
		var c FlowConversation
		if fiveTuple {
			if err := rows.Scan(&c.Src, &c.Dst, &c.SrcPort, &c.DstPort, &c.Proto,
				&c.Flows, &c.Packets, &c.Octets, &c.FirstSeen, &c.LastSeen); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&c.Src, &c.Dst,
				&c.Flows, &c.Packets, &c.Octets, &c.FirstSeen, &c.LastSeen); err != nil {
				return nil, err
			}
		}
		raw = append(raw, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if fiveTuple {
		return raw, nil
	}

	// uç-çifti: A→B ve B→A birleştir (kanonik anahtar = sıralı uçlar)
	merged := map[string]*FlowConversation{}
	for _, c := range raw {
		a, b := c.Src, c.Dst
		if a > b {
			a, b = b, a
		}
		key := a + "\x00" + b
		m := merged[key]
		if m == nil {
			m = &FlowConversation{Src: a, Dst: b, FirstSeen: c.FirstSeen, LastSeen: c.LastSeen}
			merged[key] = m
		}
		m.Flows += c.Flows
		m.Packets += c.Packets
		m.Octets += c.Octets
		if c.FirstSeen < m.FirstSeen {
			m.FirstSeen = c.FirstSeen
		}
		if c.LastSeen > m.LastSeen {
			m.LastSeen = c.LastSeen
		}
	}
	out := make([]FlowConversation, 0, len(merged))
	for _, m := range merged {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		switch orderCol {
		case "packets":
			return out[i].Packets > out[j].Packets
		case "flows":
			return out[i].Flows > out[j].Flows
		case "last_seen":
			return out[i].LastSeen > out[j].LastSeen
		default:
			return out[i].Octets > out[j].Octets
		}
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// FlowActor, bir NetFlow uç noktasıyla konuşan agent süreci (process_traffic
// korelasyonu — akış exporter'ı bunu bilmez, agent süreç atfından gelir).
type FlowActor struct {
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Process   string `json:"process"`
	IP        string `json:"ip"` // eşleşen uç (src ya da dst)
}

// FlowActorsForConversation, `since`'ten beri ipA veya ipB'ye trafik gönderen
// (agent, süreç) çiftlerini döndürür — konuşma drill-down'ında "ilişkili
// agent/süreç" için. En fazla 20 satır.
func (s *sqlStore) FlowActorsForConversation(since time.Time, ipA, ipB string) ([]FlowActor, error) {
	rows, err := s.db.Query(s.q(`SELECT pt.agent_id, a.name, pt.process, pt.remote_ip
		FROM process_traffic pt JOIN agents a ON a.id = pt.agent_id
		WHERE pt.ts >= ? AND pt.process <> '' AND (pt.remote_ip = ? OR pt.remote_ip = ?)
		GROUP BY pt.agent_id, a.name, pt.process, pt.remote_ip
		ORDER BY SUM(pt.bytes_in + pt.bytes_out) DESC LIMIT 20`), since.Unix(), ipA, ipB)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FlowActor{}
	for rows.Next() {
		var fa FlowActor
		if err := rows.Scan(&fa.AgentID, &fa.AgentName, &fa.Process, &fa.IP); err != nil {
			return nil, err
		}
		out = append(out, fa)
	}
	return out, rows.Err()
}

// FlowConversationDetail, belirli bir src↔dst konuşmasının ham akış kayıtları
// (her iki yön). proto boş değilse süzülür. idx_flows_pair kullanır.
func (s *sqlStore) FlowConversationDetail(since time.Time, src, dst, proto string, limit int, site string) ([]FlowRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ts, device, src, dst, src_port, dst_port, proto, packets, octets
		FROM flows WHERE ts >= ? AND ((src = ? AND dst = ?) OR (src = ? AND dst = ?))`
	args := []any{since.Unix(), src, dst, dst, src}
	if proto != "" {
		q += ` AND proto = ?`
		args = append(args, proto)
	}
	if site != "" {
		q += ` AND device IN (SELECT host FROM devices WHERE site = ?)`
		args = append(args, site)
	}
	q += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FlowRow{}
	for rows.Next() {
		var f FlowRow
		if err := rows.Scan(&f.Ts, &f.Device, &f.Src, &f.Dst, &f.SrcPort, &f.DstPort, &f.Proto, &f.Packets, &f.Octets); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
