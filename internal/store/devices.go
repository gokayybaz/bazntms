package store

import (
	"database/sql"
	"time"
)

// --- cihaz envanteri, SNMP ornekleri, NetFlow, syslog (Faz 3) ---

type Device struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Kind        string `json:"kind"`                // router | switch | firewall | ap | other
	SNMPVersion int    `json:"snmp_version"`        // 2 (v2c) veya 3 (v3)
	Community   string `json:"community,omitempty"` // sifreli (v2c)
	V3User      string `json:"v3_user,omitempty"`
	V3AuthProto string `json:"v3_auth_proto,omitempty"` // MD5|SHA|SHA256|SHA384|SHA512
	V3AuthPass  string `json:"v3_auth_pass,omitempty"`  // sifreli
	V3PrivProto string `json:"v3_priv_proto,omitempty"` // DES|AES|AES192|AES256
	V3PrivPass  string `json:"v3_priv_pass,omitempty"`  // sifreli
	PollSeconds int    `json:"poll_seconds"`
	Enabled     bool   `json:"enabled"`
	SysName     string `json:"sys_name"`
	SysDescr    string `json:"sys_descr"`
	AddedAt     int64  `json:"added_at"`
	LastPoll    int64  `json:"last_poll"`
	LastError   string `json:"last_error,omitempty"`
}

type DeviceIface struct {
	IfIndex     int64  `json:"if_index"`
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	Speed       uint64 `json:"speed"`
	OperStatus  int    `json:"oper_status"` // 1=up
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	InErrors    uint64 `json:"in_errors"`
	OutErrors   uint64 `json:"out_errors"`
	InDiscards  uint64 `json:"in_discards"`
	OutDiscards uint64 `json:"out_discards"`
}

type DeviceIfaceRate struct {
	DeviceIface
	RxBps float64 `json:"rx_bps"`
	TxBps float64 `json:"tx_bps"`
}

type DeviceWithStatus struct {
	Device
	Online  bool `json:"online"`
	IfaceUp int  `json:"iface_up"`
	IfaceN  int  `json:"iface_n"`
}

func (s *sqlStore) AddDevice(d Device) (int64, error) {
	if d.PollSeconds <= 0 {
		d.PollSeconds = 60
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO devices
		(name, host, kind, snmp_version, community, v3_user, v3_auth_proto, v3_auth_pass, v3_priv_proto, v3_priv_pass,
		 poll_seconds, enabled, added_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		d.Name, d.Host, d.Kind, d.SNMPVersion, d.Community, d.V3User, d.V3AuthProto, d.V3AuthPass,
		d.V3PrivProto, d.V3PrivPass, d.PollSeconds, btoi(d.Enabled), time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(s.q(`SELECT id, name, host, kind, snmp_version, community, v3_user, v3_auth_proto,
		v3_auth_pass, v3_priv_proto, v3_priv_pass, poll_seconds, enabled, sys_name, sys_descr, added_at, last_poll, last_error
		FROM devices ORDER BY id`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Host, &d.Kind, &d.SNMPVersion, &d.Community, &d.V3User,
			&d.V3AuthProto, &d.V3AuthPass, &d.V3PrivProto, &d.V3PrivPass, &d.PollSeconds, &d.Enabled,
			&d.SysName, &d.SysDescr, &d.AddedAt, &d.LastPoll, &d.LastError); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *sqlStore) DeviceByID(id int64) (*Device, error) {
	list, err := s.ListDevices()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *sqlStore) DeleteDevice(id int64) error {
	for _, q := range []string{
		`DELETE FROM devices WHERE id = ?`,
		`DELETE FROM device_iface_samples WHERE device_id = ?`,
	} {
		if _, err := s.db.Exec(s.q(q), id); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqlStore) UpdateDevicePoll(id int64, sysName, sysDescr string, lastErr string) error {
	_, err := s.db.Exec(s.q(`UPDATE devices SET last_poll = ?, sys_name = ?, sys_descr = ?, last_error = ? WHERE id = ?`),
		time.Now().Unix(), sysName, sysDescr, lastErr, id)
	return err
}

func (s *sqlStore) SaveDeviceIfaceSamples(deviceID int64, ts int64, ifaces []DeviceIface) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO device_iface_samples
		(device_id, ts, if_index, name, alias, speed, oper_status, rx_bytes, tx_bytes, in_errors, out_errors, in_discards, out_discards)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, i := range ifaces {
		if _, err := stmt.Exec(deviceID, ts, i.IfIndex, i.Name, i.Alias, i.Speed, i.OperStatus,
			i.RxBytes, i.TxBytes, i.InErrors, i.OutErrors, i.InDiscards, i.OutDiscards); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LatestDeviceIfaces, son orneklerden arayuz verimlerini hesaplar.
func (s *sqlStore) LatestDeviceIfaces(deviceID int64) ([]DeviceIfaceRate, error) {
	rows, err := s.db.Query(s.q(`SELECT if_index, name, alias, speed, oper_status, rx_bytes, tx_bytes,
		in_errors, out_errors, in_discards, out_discards, ts FROM (
		SELECT if_index, name, alias, speed, oper_status, rx_bytes, tx_bytes,
		       in_errors, out_errors, in_discards, out_discards, ts
		FROM device_iface_samples WHERE device_id = ? ORDER BY ts DESC LIMIT 400) sq ORDER BY ts ASC`), deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type sample struct {
		iface DeviceIface
		ts    int64
	}
	first := map[int64]sample{}
	last := map[int64]sample{}
	var order []int64
	for rows.Next() {
		var i DeviceIface
		var ts int64
		if err := rows.Scan(&i.IfIndex, &i.Name, &i.Alias, &i.Speed, &i.OperStatus, &i.RxBytes, &i.TxBytes,
			&i.InErrors, &i.OutErrors, &i.InDiscards, &i.OutDiscards, &ts); err != nil {
			break
		}
		if _, ok := first[i.IfIndex]; !ok {
			first[i.IfIndex] = sample{i, ts}
			order = append(order, i.IfIndex)
		}
		last[i.IfIndex] = sample{i, ts}
	}

	out := []DeviceIfaceRate{}
	for _, idx := range order {
		f, l := first[idx], last[idx]
		rate := DeviceIfaceRate{DeviceIface: l.iface}
		if l.ts > f.ts {
			dt := float64(l.ts - f.ts)
			rate.RxBps = float64(l.iface.RxBytes-f.iface.RxBytes) / dt
			rate.TxBps = float64(l.iface.TxBytes-f.iface.TxBytes) / dt
		}
		out = append(out, rate)
	}
	return out, nil
}

// --- NetFlow v5 ---

type FlowRow struct {
	Ts      int64  `json:"ts"`
	Device  string `json:"device"`
	Src     string `json:"src"`
	Dst     string `json:"dst"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
	Proto   string `json:"proto"`
	Packets uint64 `json:"packets"`
	Octets  uint64 `json:"octets"`
}

func (s *sqlStore) SaveFlows(rows []FlowRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO flows
		(ts, device, src, dst, src_port, dst_port, proto, packets, octets) VALUES (?,?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, f := range rows {
		if _, err := stmt.Exec(f.Ts, f.Device, f.Src, f.Dst, f.SrcPort, f.DstPort, f.Proto, f.Packets, f.Octets); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) TopFlows(since time.Time, limit int) ([]FlowRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(s.q(`SELECT ts, device, src, dst, src_port, dst_port, proto, packets, octets
		FROM flows WHERE ts >= ? ORDER BY octets DESC LIMIT ?`), since.Unix(), limit)
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

// --- Syslog ---

type SyslogEvent struct {
	ID       int64  `json:"id"`
	Ts       int64  `json:"ts"`
	Host     string `json:"host"`
	Severity int    `json:"severity"`
	Tag      string `json:"tag"`
	Message  string `json:"message"`
}

func (s *sqlStore) SaveSyslogEvent(e SyslogEvent) error {
	_, err := s.db.Exec(s.q(`INSERT INTO syslog_events (ts, host, severity, tag, message) VALUES (?,?,?,?,?)`),
		e.Ts, e.Host, e.Severity, e.Tag, e.Message)
	return err
}

func (s *sqlStore) RecentSyslog(limit int) ([]SyslogEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(s.q(`SELECT id, ts, host, severity, tag, message FROM syslog_events ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SyslogEvent{}
	for rows.Next() {
		var e SyslogEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Host, &e.Severity, &e.Tag, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
