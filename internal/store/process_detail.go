package store

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// --- süreç detayı / derin inceleme (Faz 23-A) ---
//
// Bir agent'taki tek bir sürecin (ad bazlı — PID zamanla değişir) tüm ağ
// etkinliğinin sunucu-tarafı toplaması. Yeni tablo yok: `process_traffic` +
// `agent_dns` + `l7_endpoints` üstünde okuma sorguları (canlı bağlantılar
// handler'da `LatestAgentConnections`'tan süzülür).

// ProcessSummary, süreç kimliği + trafik özeti.
type ProcessSummary struct {
	Process   string  `json:"process"`
	PIDs      []int64 `json:"pids"`
	FirstSeen int64   `json:"first_seen"`
	LastSeen  int64   `json:"last_seen"`
	BytesIn   uint64  `json:"bytes_in"`
	BytesOut  uint64  `json:"bytes_out"`
	Total     uint64  `json:"total"`
	RxBps     float64 `json:"rx_bps"`
	TxBps     float64 `json:"tx_bps"`
}

// ProcessSummary, süreç bazlı toplamları döndürür. Süreç `since`'ten beri hiç
// görülmediyse Total=0 ve FirstSeen=0 ile döner (handler bunu 404'e çevirir).
func (s *sqlStore) ProcessSummary(agentID int64, process string, since time.Time) (ProcessSummary, error) {
	cut := since.Unix()
	out := ProcessSummary{Process: process}

	row := s.db.QueryRow(s.q(`SELECT
		COALESCE(SUM(bytes_in),0), COALESCE(SUM(bytes_out),0),
		COALESCE(MIN(ts),0), COALESCE(MAX(ts),0)
		FROM process_traffic WHERE agent_id=? AND process=? AND ts>=?`), agentID, process, cut)
	if err := row.Scan(&out.BytesIn, &out.BytesOut, &out.FirstSeen, &out.LastSeen); err != nil {
		return out, err
	}
	out.Total = out.BytesIn + out.BytesOut
	if out.FirstSeen == 0 {
		return out, nil // hiç görülmedi
	}

	prows, err := s.db.Query(s.q(`SELECT DISTINCT pid FROM process_traffic
		WHERE agent_id=? AND process=? AND ts>=? AND pid>0 ORDER BY pid`), agentID, process, cut)
	if err != nil {
		return out, err
	}
	for prows.Next() {
		var pid int64
		if err := prows.Scan(&pid); err != nil {
			prows.Close()
			return out, err
		}
		out.PIDs = append(out.PIDs, pid)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return out, err
	}

	// anlık hız: son iki farklı ts kovası arasındaki delta (LatestDeviceIfaces
	// first/last deseni) — process_traffic satırları dönemlik bayt (kümülatif
	// sayaç değil), kova aralığını varsaymadan hız = son kova / kova-aralığı.
	brows, err := s.db.Query(s.q(`SELECT ts, SUM(bytes_in), SUM(bytes_out)
		FROM process_traffic WHERE agent_id=? AND process=? AND ts>=?
		GROUP BY ts ORDER BY ts DESC LIMIT 2`), agentID, process, cut)
	if err != nil {
		return out, err
	}
	defer brows.Close()
	type bkt struct {
		ts       int64
		in, outB uint64
	}
	var bks []bkt
	for brows.Next() {
		var b bkt
		if err := brows.Scan(&b.ts, &b.in, &b.outB); err != nil {
			return out, err
		}
		bks = append(bks, b)
	}
	if len(bks) == 2 && bks[0].ts > bks[1].ts {
		dt := float64(bks[0].ts - bks[1].ts)
		out.RxBps = float64(bks[0].in) / dt
		out.TxBps = float64(bks[0].outB) / dt
	}
	return out, brows.Err()
}

// ProcessRemote, sürecin bir uzak uç noktasına (ip:port/proto) toplam trafiği.
type ProcessRemote struct {
	RemoteIP  string `json:"remote_ip"`
	Port      int    `json:"port"`
	Proto     string `json:"proto"`
	BytesIn   uint64 `json:"bytes_in"`
	BytesOut  uint64 `json:"bytes_out"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	// Conns, bu uzak IP'ye açık canlı bağlantı sayısı — handler
	// `agent_conn_latest`'ten doldurur (sorgu penceresi değil, o an).
	Conns int `json:"conns"`
	// Country/ASN, handler `s.geo` ile doldurur (23-E `internal/enrich`
	// gelene kadar opportunistik zenginleştirme).
	Country string `json:"country,omitempty"`
	ASN     string `json:"asn,omitempty"`
}

// ProcessRemotes, sürecin uzak uç noktalarını (remote_ip,port,proto) bayt
// toplamına göre azalan sırada döndürür.
func (s *sqlStore) ProcessRemotes(agentID int64, process string, since time.Time, limit int) ([]ProcessRemote, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(s.q(`SELECT remote_ip, port, proto,
		SUM(bytes_in), SUM(bytes_out), MIN(ts), MAX(ts)
		FROM process_traffic
		WHERE agent_id=? AND process=? AND ts>=?
		GROUP BY remote_ip, port, proto
		ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT ?`), agentID, process, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcessRemote{}
	for rows.Next() {
		var r ProcessRemote
		if err := rows.Scan(&r.RemoteIP, &r.Port, &r.Proto, &r.BytesIn, &r.BytesOut, &r.FirstSeen, &r.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ProcessAppObservation, sürecin bir alan adına uygulama-katmanı görünürlüğü
// (DNS sorgusu / TLS SNI / HTTP Host).
type ProcessAppObservation struct {
	Type         string `json:"type"` // dns | tls | http
	Host         string `json:"host"`
	Observations uint64 `json:"observations"`
	Bytes        uint64 `json:"bytes"`
	FirstSeen    int64  `json:"first_seen"`
	LastSeen     int64  `json:"last_seen"`
}

// ProcessAppVisibility, sürece atfedilen DNS + L7 (SNI/Host) alan adlarını
// birleştirip gözlem sayısına göre azalan döndürür (RecentAgentDomains'in
// UNION deseni, tek agent+süreç kapsamlı).
func (s *sqlStore) ProcessAppVisibility(agentID int64, process string, since time.Time, limit int) ([]ProcessAppObservation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cut := since.Unix()
	q := `SELECT type, host, SUM(obs), SUM(bytes), MIN(ts), MAX(ts) FROM (
		SELECT 'dns' AS type, domain AS host, (queries + responses) AS obs, 0 AS bytes, ts
			FROM agent_dns WHERE agent_id=? AND process=? AND ts>=? AND domain <> ''
		UNION ALL
		SELECT kind AS type, host, hits AS obs, bytes, ts
			FROM l7_endpoints WHERE agent_id=? AND process=? AND ts>=? AND host <> ''
	) x GROUP BY type, host ORDER BY SUM(obs) DESC LIMIT ?`
	rows, err := s.db.Query(s.q(q), agentID, process, cut, agentID, process, cut, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcessAppObservation{}
	for rows.Next() {
		var o ProcessAppObservation
		if err := rows.Scan(&o.Type, &o.Host, &o.Observations, &o.Bytes, &o.FirstSeen, &o.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ProcessTimelineEntry, süreç zaman çizelgesindeki tek bir olay.
type ProcessTimelineEntry struct {
	Ts     int64  `json:"ts"`
	Event  string `json:"event"` // process.first_seen | dns.query | tls.sni | http.host | traffic.spike | alert
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
}

// ProcessTimeline, sürecin `since`'ten beri kronolojik olay akışını birleştirir:
// ilk görülme · domain başına ilk DNS sorgusu · L7 host ilk görülme · trafik
// sıçraması (kova toplamı > ortalama + 3σ) · key'inde süreç adı geçen
// alert_events. Kronolojik (artan) döndürür, en fazla `limit` (en yeniler
// tutulur).
func (s *sqlStore) ProcessTimeline(agentID int64, agentName, process string, since time.Time, limit int) ([]ProcessTimelineEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	cut := since.Unix()
	var out []ProcessTimelineEntry

	// 1) ilk görülme
	var firstTs int64
	if err := s.db.QueryRow(s.q(`SELECT COALESCE(MIN(ts),0) FROM process_traffic
		WHERE agent_id=? AND process=? AND ts>=?`), agentID, process, cut).Scan(&firstTs); err != nil {
		return nil, err
	}
	if firstTs > 0 {
		out = append(out, ProcessTimelineEntry{Ts: firstTs, Event: "process.first_seen", Target: process})
	}

	// 2) DNS sorguları — domain başına ilk görülme
	if err := s.timelineRows(&out, `SELECT domain, MIN(ts), SUM(queries+responses)
		FROM agent_dns WHERE agent_id=? AND process=? AND ts>=? AND domain <> ''
		GROUP BY domain`, []any{agentID, process, cut},
		func(host string, ts, n int64) ProcessTimelineEntry {
			return ProcessTimelineEntry{Ts: ts, Event: "dns.query", Target: host, Detail: fmt.Sprintf("%d gözlem", n)}
		}); err != nil {
		return nil, err
	}

	// 3) L7 host ilk görülme (SNI / HTTP Host)
	lrows, err := s.db.Query(s.q(`SELECT kind, host, MIN(ts)
		FROM l7_endpoints WHERE agent_id=? AND process=? AND ts>=? AND host <> ''
		GROUP BY kind, host`), agentID, process, cut)
	if err != nil {
		return nil, err
	}
	for lrows.Next() {
		var kind, host string
		var ts int64
		if err := lrows.Scan(&kind, &host, &ts); err != nil {
			lrows.Close()
			return nil, err
		}
		ev := "tls.sni"
		if kind == "http" {
			ev = "http.host"
		}
		out = append(out, ProcessTimelineEntry{Ts: ts, Event: ev, Target: host})
	}
	lrows.Close()
	if err := lrows.Err(); err != nil {
		return nil, err
	}

	// 4) trafik sıçraması — kova toplamı > ortalama + 3σ
	trows, err := s.db.Query(s.q(`SELECT ts, SUM(bytes_in + bytes_out)
		FROM process_traffic WHERE agent_id=? AND process=? AND ts>=?
		GROUP BY ts ORDER BY ts`), agentID, process, cut)
	if err != nil {
		return nil, err
	}
	type sample struct {
		ts    int64
		bytes float64
	}
	var series []sample
	for trows.Next() {
		var sm sample
		if err := trows.Scan(&sm.ts, &sm.bytes); err != nil {
			trows.Close()
			return nil, err
		}
		series = append(series, sm)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}
	if len(series) >= 5 {
		var sum float64
		for _, sm := range series {
			sum += sm.bytes
		}
		mean := sum / float64(len(series))
		var varsum float64
		for _, sm := range series {
			d := sm.bytes - mean
			varsum += d * d
		}
		std := math.Sqrt(varsum / float64(len(series)))
		thresh := mean + 3*std
		for _, sm := range series {
			if std > 0 && mean > 0 && sm.bytes > thresh {
				out = append(out, ProcessTimelineEntry{
					Ts: sm.ts, Event: "traffic.spike",
					Detail: fmt.Sprintf("%.1f MB", sm.bytes/1024/1024),
				})
			}
		}
	}

	// 5) ilişkili uyarılar — key'inde süreç adı geçenler (hub-yerel key=process;
	// agent-bazlı key=agentName:process — fireCtx). target/ioc IP/domain
	// key'leri bu süzgeçten geçmez (bilinen sınır).
	arows, err := s.db.Query(s.q(`SELECT ts, kind, message FROM alert_events
		WHERE ts>=? AND kind IN ('proc','anomaly','bw','target','ioc')
		AND (key = ? OR key = ?) ORDER BY ts`),
		cut, process, agentName+":"+process)
	if err != nil {
		return nil, err
	}
	for arows.Next() {
		var ts int64
		var kind, msg string
		if err := arows.Scan(&ts, &kind, &msg); err != nil {
			arows.Close()
			return nil, err
		}
		out = append(out, ProcessTimelineEntry{Ts: ts, Event: "alert", Target: kind, Detail: msg})
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Ts < out[j].Ts })
	if len(out) > limit {
		out = out[len(out)-limit:] // en yeni `limit` olay, kronolojik
	}
	return out, nil
}

// timelineRows, (host, ts, n) üçlüsü döndüren bir sorguyu çalıştırıp her satırı
// `mk` ile ProcessTimelineEntry'ye çevirir ve `out`'a ekler.
func (s *sqlStore) timelineRows(out *[]ProcessTimelineEntry, query string, args []any, mk func(host string, ts, n int64) ProcessTimelineEntry) error {
	rows, err := s.db.Query(s.q(query), args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var host string
		var ts, n int64
		if err := rows.Scan(&host, &ts, &n); err != nil {
			return err
		}
		*out = append(*out, mk(host, ts, n))
	}
	return rows.Err()
}
