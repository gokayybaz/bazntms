package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- agent registry (Faz 1) ---

type Agent struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Site            string `json:"site"`
	TokenHash       string `json:"-"`
	FirstSeen       int64  `json:"first_seen"`
	LastSeen        int64  `json:"last_seen"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	RemoteIP        string `json:"remote_ip"`
	// MachineID, agent'ın kararlı makine kimliği hash'i (C3, Faz 13). Boş
	// olabilir (eski agent / kimlik alınamadı) — o zaman her hello yeni satır.
	MachineID string `json:"-"`
	// UplinkDeviceID, agent'ın bağlı olduğu erişim katmanı cihazı (switch/AP).
	// Canlı Akış şeması agent'ları bu alana göre gruplar. nil = "Doğrudan"
	// (router'a doğrudan bağlı). Yönetici manuel atar (SNMP keşfi değil).
	UplinkDeviceID *int64 `json:"uplink_device_id,omitempty"`
	// AttrMethod, agent'ın en son bildirdiği aktif süreç-atıf arka ucu:
	// "ebpf" | "pcap" | "etw" | "off". Boş = henüz bildirmedi / eski agent.
	AttrMethod string `json:"attr_method,omitempty"`
	// AttrIface, pcap arka ucunun dinlediği yakalama arayüzü (yalnız
	// method=pcap). UI teşhisi: "motor çalışıyor ama panel boş → yanlış/sanal
	// arayüz mü?".
	AttrIface string `json:"attr_iface,omitempty"`
	// AttrNote, atıf motoru kapalı/başlatılamadıysa insan-okur neden
	// ("collect.method=off" | "hub -agent-pcap=false" | pcap hata ipucu).
	AttrNote string `json:"attr_note,omitempty"`
}

func TokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *sqlStore) RegisterAgent(a Agent) (int64, error) {
	now := time.Now().Unix()
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO agents (name, site, token_hash, first_seen, last_seen, version, protocol_version, remote_ip, machine_id)
		VALUES (?,?,?,?,?,?,?,?,?) RETURNING id`),
		a.Name, a.Site, a.TokenHash, now, now, a.Version, a.ProtocolVersion, a.RemoteIP, a.MachineID).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// RegisterOrReuseAgent, C3 (Faz 13): a.MachineID doluysa ve aynı machine_id +
// site'lı ÇEVRİMDIŞI (last_seen < offlineBefore) bir agent varsa, o satırı yeni
// token/isim/sürümle günceller ve id'sini döndürür (reused=true) — state
// dosyası kaybında `agents` tablosu şişmesin. Aksi halde yeni satır açar.
// machine_id "" veya eşleşen kayıt çevrimiçi ise her zaman yeni satır.
func (s *sqlStore) RegisterOrReuseAgent(a Agent, offlineBefore int64) (id int64, reused bool, err error) {
	if a.MachineID != "" {
		var existing int64
		var lastSeen int64
		e := s.db.QueryRow(s.q(`SELECT id, last_seen FROM agents
			WHERE machine_id = ? AND site = ? ORDER BY last_seen DESC LIMIT 1`),
			a.MachineID, a.Site).Scan(&existing, &lastSeen)
		if e == nil && lastSeen < offlineBefore {
			now := time.Now().Unix()
			if _, e := s.db.Exec(s.q(`UPDATE agents SET
					name = ?, token_hash = ?, version = ?, protocol_version = ?,
					remote_ip = ?, last_seen = ?
				WHERE id = ?`),
				a.Name, a.TokenHash, a.Version, a.ProtocolVersion, a.RemoteIP, now, existing); e != nil {
				return 0, false, e
			}
			return existing, true, nil
		}
		if e != nil && e != sql.ErrNoRows {
			return 0, false, e
		}
	}
	id, err = s.RegisterAgent(a)
	return id, false, err
}

func (s *sqlStore) AgentByTokenHash(hash string) (*Agent, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, site, first_seen, last_seen, version, protocol_version, remote_ip
		FROM agents WHERE token_hash = ?`), hash)
	var a Agent
	a.TokenHash = hash
	err := row.Scan(&a.ID, &a.Name, &a.Site, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// TouchAgent, telemetri/heartbeat'te cagrılır: son gorulme + meta gunceller.
// version bos / protoVersion 0 ise (surum tasimayan eski agent) o alan
// degistirilmez — mevcut deger korunur.
func (s *sqlStore) TouchAgent(id int64, version string, protoVersion int, remoteIP string) error {
	_, err := s.db.Exec(s.q(`UPDATE agents SET
			last_seen = ?,
			remote_ip = ?,
			version = COALESCE(NULLIF(?, ''), version),
			protocol_version = COALESCE(NULLIF(?, 0), protocol_version)
		WHERE id = ?`),
		time.Now().Unix(), remoteIP, version, protoVersion, id)
	return err
}

// SetAgentAttrInfo, agent'ın her telemetri batch'inde bildirdiği süreç-atıf
// teşhis bilgisini kaydeder: aktif arka uç (method) + pcap yakalama arayüzü
// (iface) + kapalı/başlatılamadı nedeni (note). method boş = değiştirme (alan
// taşımayan eski agent — çağıran zaten bu durumda çağırmaz); iface/note ise
// o batch'te ne bildirildiyse odur (boş dönebilir → motor sağlıklı çalışıyor).
func (s *sqlStore) SetAgentAttrInfo(id int64, method, iface, note string) error {
	_, err := s.db.Exec(
		s.q(`UPDATE agents SET
			attr_method = COALESCE(NULLIF(?, ''), attr_method),
			attr_iface  = ?,
			attr_note   = ?
			WHERE id = ?`),
		method, iface, note, id)
	return err
}

func (s *sqlStore) SaveIfaceSamples(agentID int64, ts int64, samples []telemetry.InterfaceSample) error {
	defer metrics.ObserveStoreWrite("agent_iface_samples", len(samples), time.Now())
	rows := make([][]any, len(samples))
	for i, sm := range samples {
		rows[i] = []any{agentID, ts, sm.Name, sm.RxBytes, sm.TxBytes, sm.RxPackets, sm.TxPackets}
	}
	return s.bulkInsert("agent_iface_samples",
		[]string{"agent_id", "ts", "name", "rx_bytes", "tx_bytes", "rx_packets", "tx_packets"}, rows)
}

func (s *sqlStore) ReplaceConnLatest(agentID int64, conns []telemetry.ConnectionSample) error {
	defer metrics.ObserveStoreWrite("agent_conn_latest", len(conns), time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(s.q(`DELETE FROM agent_conn_latest WHERE agent_id = ?`), agentID); err != nil {
		return err
	}
	rows := make([][]any, len(conns))
	for i, c := range conns {
		rows[i] = []any{agentID, c.Proto, c.LocalAddr, c.RemoteAddr, c.Status, c.PID, c.Process}
	}
	if err := s.insertRows(tx, "agent_conn_latest",
		[]string{"agent_id", "proto", "local_addr", "remote_addr", "status", "pid", "process"}, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// AgentRate, son iki ornekten hesaplanan arayuz verim bilgisidir.
type AgentRate struct {
	Name      string  `json:"name"`
	RxBps     float64 `json:"rx_bps"`
	TxBps     float64 `json:"tx_bps"`
	RxBytes   uint64  `json:"rx_bytes"`
	TxBytes   uint64  `json:"tx_bytes"`
	Pps       float64 `json:"pps"`
	RxPackets uint64  `json:"rx_packets"`
	TxPackets uint64  `json:"tx_packets"`
	LastSeen  int64   `json:"last_seen"`
}

type AgentWithRates struct {
	Agent
	Online bool        `json:"online"`
	Rates  []AgentRate `json:"rates"`
	Conns  int         `json:"conns"`
}

// ListAgents, filo gorunumu: online durumu + son orneklerden hesaplanmis
// verimler. site bos degilse yalnizca o sitenin agent'lari doner (Faz 5.1
// site scope).
func (s *sqlStore) ListAgents(onlineWindow time.Duration, site string) ([]AgentWithRates, error) {
	q := `SELECT id, name, site, first_seen, last_seen, version, protocol_version, remote_ip, uplink_device_id, attr_method, attr_iface, attr_note
		FROM agents`
	args := []any{}
	if site != "" {
		q += ` WHERE site = ?`
		args = append(args, site)
	}
	q += ` ORDER BY last_seen DESC`
	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentWithRates
	now := time.Now().Unix()
	for rows.Next() {
		var a AgentWithRates
		var uplink sql.NullInt64
		var attrMethod, attrIface, attrNote sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &a.Site, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP, &uplink, &attrMethod, &attrIface, &attrNote); err != nil {
			return nil, err
		}
		a.UplinkDeviceID = i64ptr(uplink)
		a.AttrMethod = attrMethod.String
		a.AttrIface = attrIface.String
		a.AttrNote = attrNote.String
		a.Online = now-a.LastSeen <= int64(onlineWindow.Seconds())
		out = append(out, a)
	}
	if out == nil {
		out = []AgentWithRates{}
	}

	if len(out) == 0 {
		return out, nil
	}

	// Arayuz verimleri: her agent icin YALNIZCA en son iki telemetri batch'i
	// (ayni ts'e sahip tum arayuzler tek batch). Onceden agent basina ayri
	// sorgu (N+1 — 5000 agent'ta ~1 sn) yapiliyordu; artik tek pencereli
	// sorgu tum filoyu getirir. Sayac gerilemesi (arayuz/agent reset) → o
	// tur icin oran 0 (safeDeltaU64), bir sonraki tutarli iki ornekte duzelir.
	idx := make(map[int64]int, len(out))
	for i := range out {
		idx[out[i].ID] = i
	}
	type sample struct {
		rx, tx, rxPkts, txPkts uint64
		ts                     int64
	}
	// agent_id → arayuz adi → [ilk, son]
	byAgent := make(map[int64]map[string][2]sample, len(out))
	rows2, err := s.db.Query(s.q(`SELECT s.agent_id, s.name, s.rx_bytes, s.tx_bytes, s.rx_packets, s.tx_packets, s.ts
		FROM agent_iface_samples s
		JOIN (
			SELECT agent_id, ts, ROW_NUMBER() OVER (PARTITION BY agent_id ORDER BY ts DESC) AS rn
			FROM (SELECT DISTINCT agent_id, ts FROM agent_iface_samples) d
		) t2 ON t2.agent_id = s.agent_id AND t2.ts = s.ts
		WHERE t2.rn <= 2
		ORDER BY s.agent_id, s.ts ASC`))
	if err == nil {
		for rows2.Next() {
			var aid int64
			var name string
			var sm sample
			if err := rows2.Scan(&aid, &name, &sm.rx, &sm.tx, &sm.rxPkts, &sm.txPkts, &sm.ts); err != nil {
				break
			}
			m := byAgent[aid]
			if m == nil {
				m = map[string][2]sample{}
				byAgent[aid] = m
			}
			pair := m[name]
			if pair[0].ts == 0 {
				pair[0] = sm
			}
			pair[1] = sm
			m[name] = pair
		}
		rows2.Close()
	}
	for aid, m := range byAgent {
		i, ok := idx[aid]
		if !ok {
			continue
		}
		for name, pair := range m {
			f, l := pair[0], pair[1]
			if l.ts <= f.ts {
				continue
			}
			dt := float64(l.ts - f.ts)
			out[i].Rates = append(out[i].Rates, AgentRate{
				Name:      name,
				RxBps:     float64(safeDeltaU64(l.rx, f.rx)) / dt,
				TxBps:     float64(safeDeltaU64(l.tx, f.tx)) / dt,
				RxBytes:   l.rx,
				TxBytes:   l.tx,
				Pps:       float64(safeDeltaU64(l.rxPkts, f.rxPkts)+safeDeltaU64(l.txPkts, f.txPkts)) / dt,
				RxPackets: l.rxPkts,
				TxPackets: l.txPkts,
				LastSeen:  l.ts,
			})
		}
	}

	// Baglanti sayilari: tek GROUP BY (yine N+1'di).
	if crows, err := s.db.Query(s.q(`SELECT agent_id, COUNT(*) FROM agent_conn_latest GROUP BY agent_id`)); err == nil {
		for crows.Next() {
			var aid int64
			var n int
			if err := crows.Scan(&aid, &n); err != nil {
				break
			}
			if i, ok := idx[aid]; ok {
				out[i].Conns = n
			}
		}
		crows.Close()
	}
	return out, nil
}

// safeDeltaU64, iki kumulatif sayac arasindaki farki dondurur; sayac
// gerilediyse (arayuz/agent resetlendi) uint64 alt tasmasi yerine 0 doner.
func safeDeltaU64(cur, prev uint64) uint64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}

func (s *sqlStore) LatestAgentConnections(agentID int64) []telemetry.ConnectionSample {
	rows, err := s.db.Query(s.q(`SELECT proto, local_addr, remote_addr, status, pid, process
		FROM agent_conn_latest WHERE agent_id = ? ORDER BY process, local_addr`), agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []telemetry.ConnectionSample
	for rows.Next() {
		var c telemetry.ConnectionSample
		if err := rows.Scan(&c.Proto, &c.LocalAddr, &c.RemoteAddr, &c.Status, &c.PID, &c.Process); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

// AgentHistory, verilen agent'in ham arayuz orneklerinden zaman serisi
// hesaplar (tum arayuzlerin toplami, bayt/sn + paket/sn) — Agent Detay
// sayfasindaki throughput grafiginde kullanilir (ThroughputChart ile
// ayni Bucket semasi: ts/in/out/local/pps).
func (s *sqlStore) AgentHistory(agentID int64, since time.Time) ([]Bucket, error) {
	rows, err := s.db.Query(s.q(`SELECT name, ts, rx_bytes, tx_bytes, rx_packets, tx_packets
		FROM agent_iface_samples WHERE agent_id = ? AND ts >= ? ORDER BY name, ts ASC`), agentID, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type point struct {
		ts             int64
		rx, tx         uint64
		rxPkts, txPkts uint64
	}
	byIface := map[string][]point{}
	for rows.Next() {
		var name string
		var p point
		if err := rows.Scan(&name, &p.ts, &p.rx, &p.tx, &p.rxPkts, &p.txPkts); err != nil {
			return nil, err
		}
		byIface[name] = append(byIface[name], p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// her arayuz icin ardisik ornekler arasi bps/pps hesaplanir; ayni ts'e
	// denk gelen tum arayuzlerin degerleri toplanir (fleet/agent geneli)
	agg := map[int64]*Bucket{}
	for _, pts := range byIface {
		for i := 1; i < len(pts); i++ {
			prev, cur := pts[i-1], pts[i]
			dt := float64(cur.ts - prev.ts)
			if dt <= 0 {
				continue
			}
			b, ok := agg[cur.ts]
			if !ok {
				b = &Bucket{Ts: cur.ts}
				agg[cur.ts] = b
			}
			// sayac gerilediyse (arayuz/agent resetlendi) bu nokta icin
			// uint64 alt tasmasi yerine 0 katkisi verilir
			b.In += float64(safeDeltaU64(cur.rx, prev.rx)) / dt
			b.Out += float64(safeDeltaU64(cur.tx, prev.tx)) / dt
			b.Pps += float64(safeDeltaU64(cur.rxPkts, prev.rxPkts)+safeDeltaU64(cur.txPkts, prev.txPkts)) / dt
		}
	}
	out := make([]Bucket, 0, len(agg))
	for _, b := range agg {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ts < out[j].Ts })
	return out, nil
}

func (s *sqlStore) AgentByID(id int64) (*Agent, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, site, token_hash, first_seen, last_seen, version, protocol_version, remote_ip, uplink_device_id, attr_method, attr_iface, attr_note
		FROM agents WHERE id = ?`), id)
	var a Agent
	var uplink sql.NullInt64
	var attrMethod, attrIface, attrNote sql.NullString
	err := row.Scan(&a.ID, &a.Name, &a.Site, &a.TokenHash, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP, &uplink, &attrMethod, &attrIface, &attrNote)
	if err != nil {
		return nil, err
	}
	a.UplinkDeviceID = i64ptr(uplink)
	a.AttrMethod = attrMethod.String
	a.AttrIface = attrIface.String
	a.AttrNote = attrNote.String
	return &a, nil
}

// SetAgentUplink, agent'ın erişim katmanı cihazını (switch/AP) atar; deviceID
// nil ise bağı kaldırır ("Doğrudan"). Cihazın varlığı/türü/site'ı çağıran
// katmanda (handler) doğrulanır.
func (s *sqlStore) SetAgentUplink(agentID int64, deviceID *int64) error {
	_, err := s.db.Exec(s.q(`UPDATE agents SET uplink_device_id = ? WHERE id = ?`), nullI64(deviceID), agentID)
	return err
}

func (s *sqlStore) RenameAgent(id int64, name string) error {
	_, err := s.db.Exec(s.q(`UPDATE agents SET name = ? WHERE id = ?`), name, id)
	return err
}

// DeleteAgent, agent'i ve ona bagli TUM satirlari siler (S13.7 — tam cascade).
// Bir tablo unutulursa agent silindikten sonra o satirlar oksuz kalir
// (agent_id artik agents'te yok) ve retention suresi dolana dek "hayalet" filo
// verisi uretir.
func (s *sqlStore) DeleteAgent(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// agent-adi bazli alert_seen anahtarlari (alert.checkAgentNewProc:
	// kind = "agent-proc:"+name). Ad, agent silindikten sonra bir baskasina
	// verilirse eski "gorulmus surec" tabani miras alinip yeni surecler icin
	// alarm kacirilmasin.
	var name string
	if err := tx.QueryRow(s.q(`SELECT name FROM agents WHERE id = ?`), id).Scan(&name); err == nil && name != "" {
		if _, err := tx.Exec(s.q(`DELETE FROM alert_seen WHERE kind = ?`), "agent-proc:"+name); err != nil {
			return err
		}
	}

	for _, q := range []string{
		`DELETE FROM agents WHERE id = ?`,
		`DELETE FROM agent_iface_samples WHERE agent_id = ?`,
		`DELETE FROM agent_conn_latest WHERE agent_id = ?`,
		`DELETE FROM process_traffic WHERE agent_id = ?`,
		`DELETE FROM l7_endpoints WHERE agent_id = ?`,
		`DELETE FROM agent_dns WHERE agent_id = ?`,
	} {
		if _, err := tx.Exec(s.q(q), id); err != nil {
			return err
		}
	}
	// topoloji: agent'in bildirdigi yerel aglar (subnet) + olasi agent↔agent
	// kenarlari — hem kaynak hem peer tarafi.
	if _, err := tx.Exec(s.q(`DELETE FROM topology_links
		WHERE (source_type = 'agent' AND source_id = ?) OR (peer_type = 'agent' AND peer_id = ?)`), id, id); err != nil {
		return err
	}
	return tx.Commit()
}

// PruneOfflineAgents, offlineFor suresinden uzun sure gorulmemis agent'lari
// tam cascade ile siler (S13.7 — "N gundur offline" otomatik arsivleme).
// Donen deger silinen agent sayisidir. offlineFor <= 0 → no-op.
func (s *sqlStore) PruneOfflineAgents(offlineFor time.Duration) (int, error) {
	if offlineFor <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-offlineFor).Unix()
	rows, err := s.db.Query(s.q(`SELECT id, name, last_seen FROM agents WHERE last_seen < ?`), cutoff)
	if err != nil {
		return 0, err
	}
	type victim struct {
		id       int64
		name     string
		lastSeen int64
	}
	var victims []victim
	for rows.Next() {
		var v victim
		if err := rows.Scan(&v.id, &v.name, &v.lastSeen); err != nil {
			rows.Close()
			return 0, err
		}
		victims = append(victims, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	n := 0
	for _, v := range victims {
		if err := s.DeleteAgent(v.id); err != nil {
			return n, err
		}
		slog.Info("uzun sure cevrimdisi agent arsivlendi",
			"agent_id", v.id, "name", v.name,
			"son_gorulme", time.Unix(v.lastSeen, 0).Format(time.RFC3339))
		n++
	}
	return n, nil
}
