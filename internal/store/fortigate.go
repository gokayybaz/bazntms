package store

// Faz 8.3/8.4: FortiGate REST toplama verileri — kaynak kullanımı, VPN
// tünel durumu, SD-WAN sağlık örnekleri ve politika hit sayaçları.

import (
	"time"
)

// DeviceResource, cihazın anlık kaynak kullanımı (per poll bir kayıt).
type DeviceResource struct {
	Ts       int64   `json:"ts"`
	DeviceID int64   `json:"device_id"`
	CPUPct   float64 `json:"cpu_pct"`
	MemPct   float64 `json:"mem_pct"`
	DiskPct  float64 `json:"disk_pct"`
	Sessions int64   `json:"sessions"`
}

// FortiVPNStatus, güncel VPN tünel/kullanıcı durumu (upsert tablosu).
type FortiVPNStatus struct {
	DeviceID int64  `json:"device_id"`
	VDOM     string `json:"vdom"`
	Kind     string `json:"kind"`   // ipsec | ssl
	Name     string `json:"name"`   // tünel adı / kullanıcı adı
	Peer     string `json:"peer"`   // uzak gw / kullanıcı IP
	Status   string `json:"status"` // up | down | connecting | ...
	Uptime   int64  `json:"uptime"`
	RxBytes  uint64 `json:"rx_bytes"`
	TxBytes  uint64 `json:"tx_bytes"`
	Ts       int64  `json:"ts"` // son görülme
}

// FortiSDWANSample, SD-WAN health-check örneği (zaman serisi).
type FortiSDWANSample struct {
	Ts            int64   `json:"ts"`
	DeviceID      int64   `json:"device_id"`
	VDOM          string  `json:"vdom"`
	Member        string  `json:"member"`
	HealthCheck   string  `json:"health_check"`
	LatencyMs     float64 `json:"latency_ms"`
	JitterMs      float64 `json:"jitter_ms"`
	PacketLossPct float64 `json:"packet_loss_pct"`
	State         string  `json:"state"` // up | down | ...
}

// FortiPolicyHit, firewall policy hit sayacı örneği (kümülatif; delta sorguda).
type FortiPolicyHit struct {
	Ts       int64  `json:"ts"`
	DeviceID int64  `json:"device_id"`
	VDOM     string `json:"vdom"`
	PolicyID int64  `json:"policy_id"`
	Name     string `json:"name"`
	Action   string `json:"action"`
	Hits     uint64 `json:"hits"`  // kümülatif
	Bytes    uint64 `json:"bytes"` // kümülatif
}

// --- yazımlar ---

func (s *sqlStore) SaveDeviceResources(r DeviceResource) error {
	_, err := s.db.Exec(s.q(`INSERT INTO device_resources
		(ts, device_id, cpu_pct, mem_pct, disk_pct, sessions) VALUES (?,?,?,?,?,?)`),
		r.Ts, r.DeviceID, r.CPUPct, r.MemPct, r.DiskPct, r.Sessions)
	return err
}

func (s *sqlStore) SaveFortiVPNStatus(deviceID int64, ts int64, rows []FortiVPNStatus) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO fortigate_vpn_status
		(device_id, vdom, kind, name, peer, status, uptime, rx_bytes, tx_bytes, ts)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (device_id, vdom, kind, name) DO UPDATE SET
			peer = excluded.peer, status = excluded.status, uptime = excluded.uptime,
			rx_bytes = excluded.rx_bytes, tx_bytes = excluded.tx_bytes, ts = excluded.ts`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, v := range rows {
		if _, err := stmt.Exec(deviceID, v.VDOM, v.Kind, v.Name, v.Peer, v.Status, v.Uptime, v.RxBytes, v.TxBytes, ts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) SaveFortiSDWAN(deviceID int64, ts int64, rows []FortiSDWANSample) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO fortigate_sdwan
		(ts, device_id, vdom, member, health_check, latency_ms, jitter_ms, packet_loss_pct, state)
		VALUES (?,?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, w := range rows {
		if _, err := stmt.Exec(ts, deviceID, w.VDOM, w.Member, w.HealthCheck,
			w.LatencyMs, w.JitterMs, w.PacketLossPct, w.State); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) SaveFortiPolicyHits(deviceID int64, ts int64, rows []FortiPolicyHit) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO fortigate_policy_hits
		(ts, device_id, vdom, policy_id, name, action, hits, bytes) VALUES (?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range rows {
		if _, err := stmt.Exec(ts, deviceID, p.VDOM, p.PolicyID, p.Name, p.Action, p.Hits, p.Bytes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- sorgular (UI) ---

func (s *sqlStore) LatestDeviceResources(deviceID int64, minutes int) ([]DeviceResource, error) {
	rows, err := s.db.Query(s.q(`SELECT ts, device_id, cpu_pct, mem_pct, disk_pct, sessions
		FROM device_resources WHERE device_id = ? AND ts >= ?
		ORDER BY ts ASC`), deviceID, time.Now().Add(-time.Duration(minutes)*time.Minute).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceResource{}
	for rows.Next() {
		var r DeviceResource
		if err := rows.Scan(&r.Ts, &r.DeviceID, &r.CPUPct, &r.MemPct, &r.DiskPct, &r.Sessions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sqlStore) LatestFortiVPN(deviceID int64) ([]FortiVPNStatus, error) {
	rows, err := s.db.Query(s.q(`SELECT device_id, vdom, kind, name, peer, status, uptime, rx_bytes, tx_bytes, ts
		FROM fortigate_vpn_status WHERE device_id = ?
		ORDER BY kind, name`), deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FortiVPNStatus{}
	for rows.Next() {
		var v FortiVPNStatus
		if err := rows.Scan(&v.DeviceID, &v.VDOM, &v.Kind, &v.Name, &v.Peer, &v.Status,
			&v.Uptime, &v.RxBytes, &v.TxBytes, &v.Ts); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TopFortiPolicies, penceredeki en aktif politikalari (hit delta) dondurur.
// Ilk/son sayac karsilastirmasi LatestDeviceIfaces deseniyle yapilir.
func (s *sqlStore) TopFortiPolicies(deviceID int64, since time.Time, limit int) ([]FortiPolicyHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(s.q(`SELECT ts, device_id, vdom, policy_id, name, action, hits, bytes
		FROM fortigate_policy_hits WHERE device_id = ? AND ts >= ?
		ORDER BY ts ASC`), deviceID, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type acc struct {
		p           FortiPolicyHit
		first, last FortiPolicyHit
	}
	byID := map[int64]*acc{}
	var order []int64
	for rows.Next() {
		var p FortiPolicyHit
		if err := rows.Scan(&p.Ts, &p.DeviceID, &p.VDOM, &p.PolicyID, &p.Name, &p.Action, &p.Hits, &p.Bytes); err != nil {
			return nil, err
		}
		a, ok := byID[p.PolicyID]
		if !ok {
			a = &acc{first: p}
			a.p = p
			byID[p.PolicyID] = a
			order = append(order, p.PolicyID)
		}
		a.last = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []FortiPolicyHit{}
	for _, id := range order {
		a := byID[id]
		if a.last.Ts <= a.first.Ts {
			continue
		}
		if a.last.Hits < a.first.Hits {
			continue // sayaç sıfırlanmış (cihaz yeniden başlatma): yok say
		}
		out = append(out, FortiPolicyHit{
			Ts: a.last.Ts, DeviceID: a.p.DeviceID, VDOM: a.p.VDOM, PolicyID: id,
			Name: a.p.Name, Action: a.p.Action,
			Hits: a.last.Hits - a.first.Hits, Bytes: a.last.Bytes - a.first.Bytes,
		})
	}
	// delta'ya göre sırala (en aktif üstte)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Hits+out[j].Bytes/1024 > out[j-1].Hits+out[j-1].Bytes/1024; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// --- sorgular (uyarı motoru, cihaz adı ile) ---

type VPNDownRow struct {
	DeviceName string `json:"device_name"`
	FortiVPNStatus
}

// FortiVPNsDown, aktif cihazlarda down durumda (ve güncel) tünelleri dondurur.
func (s *sqlStore) FortiVPNsDown(freshWithin time.Duration) ([]VPNDownRow, error) {
	rows, err := s.db.Query(s.q(`SELECT d.name, v.device_id, v.vdom, v.kind, v.name, v.peer, v.status, v.uptime, v.rx_bytes, v.tx_bytes, v.ts
		FROM fortigate_vpn_status v JOIN devices d ON d.id = v.device_id
		WHERE v.status = 'down' AND v.ts >= ? AND d.enabled = 1
		ORDER BY d.name, v.name`),
		time.Now().Add(-freshWithin).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VPNDownRow{}
	for rows.Next() {
		var r VPNDownRow
		if err := rows.Scan(&r.DeviceName, &r.DeviceID, &r.VDOM, &r.Kind, &r.Name, &r.Peer,
			&r.Status, &r.Uptime, &r.RxBytes, &r.TxBytes, &r.Ts); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SDWANRow struct {
	DeviceName string `json:"device_name"`
	FortiSDWANSample
}

// RecentFortiSDWANAll, aktif cihazların penceredeki SD-WAN örnekleri.
func (s *sqlStore) RecentFortiSDWANAll(since time.Time) ([]SDWANRow, error) {
	rows, err := s.db.Query(s.q(`SELECT d.name, w.ts, w.device_id, w.vdom, w.member, w.health_check,
		w.latency_ms, w.jitter_ms, w.packet_loss_pct, w.state
		FROM fortigate_sdwan w JOIN devices d ON d.id = w.device_id
		WHERE w.ts >= ? AND d.enabled = 1 ORDER BY d.name, w.member`), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SDWANRow{}
	for rows.Next() {
		var r SDWANRow
		if err := rows.Scan(&r.DeviceName, &r.Ts, &r.DeviceID, &r.VDOM, &r.Member, &r.HealthCheck,
			&r.LatencyMs, &r.JitterMs, &r.PacketLossPct, &r.State); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ResourceRow struct {
	DeviceName string `json:"device_name"`
	DeviceResource
}

// RecentDeviceResourcesAll, aktif cihazların penceredeki kaynak örnekleri.
func (s *sqlStore) RecentDeviceResourcesAll(since time.Time) ([]ResourceRow, error) {
	rows, err := s.db.Query(s.q(`SELECT d.name, r.ts, r.device_id, r.cpu_pct, r.mem_pct, r.disk_pct, r.sessions
		FROM device_resources r JOIN devices d ON d.id = r.device_id
		WHERE r.ts >= ? AND d.enabled = 1 ORDER BY d.name, r.ts`), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ResourceRow{}
	for rows.Next() {
		var r ResourceRow
		if err := rows.Scan(&r.DeviceName, &r.Ts, &r.DeviceID, &r.CPUPct, &r.MemPct, &r.DiskPct, &r.Sessions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
