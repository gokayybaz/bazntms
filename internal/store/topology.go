package store

import (
	"net"
	"time"
)

// --- topoloji kesfi (Faz 6.1) ---
//
// Kenarlar dedupe edilir (upsert): ayni komsu tekrar gorulurse yalnizca
// ts (son gorulme) guncellenir. Keşif kaynaklari:
//   - lldp/cdp/arp : SNMP poller (kaynak = cihaz)
//   - subnet       : agent telemetrisi (kaynak = agent, peer_ip = CIDR)

type TopologyLink struct {
	ID         int64  `json:"id"`
	Ts         int64  `json:"ts"`          // son gorulme
	Kind       string `json:"kind"`        // lldp | cdp | arp | subnet
	SourceType string `json:"source_type"` // device | agent
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	LocalPort  string `json:"local_port"`
	PeerType   string `json:"peer_type"` // device | agent | host
	PeerID     int64  `json:"peer_id"`
	PeerName   string `json:"peer_name"`
	PeerIP     string `json:"peer_ip"`
}

func (s *sqlStore) UpsertTopologyLink(l TopologyLink) error {
	if l.Ts == 0 {
		l.Ts = time.Now().Unix()
	}
	_, err := s.db.Exec(s.q(`INSERT INTO topology_links
		(ts, kind, source_type, source_id, source_name, local_port, peer_type, peer_id, peer_name, peer_ip)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (kind, source_type, source_id, local_port, peer_name, peer_ip) DO UPDATE SET
			ts = excluded.ts, source_name = excluded.source_name,
			peer_type = excluded.peer_type, peer_id = excluded.peer_id`),
		l.Ts, l.Kind, l.SourceType, l.SourceID, l.SourceName, l.LocalPort,
		l.PeerType, l.PeerID, l.PeerName, l.PeerIP)
	return err
}

// SaveAgentSubnets, agent'in bildirdigi yerel aglari (CIDR) topolojiye isler.
func (s *sqlStore) SaveAgentSubnets(agentID int64, name string, subnets []string) error {
	now := time.Now().Unix()
	for _, cidr := range subnets {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			continue
		}
		if err := s.UpsertTopologyLink(TopologyLink{
			Ts: now, Kind: "subnet",
			SourceType: "agent", SourceID: agentID, SourceName: name,
			PeerType: "host", PeerIP: cidr,
		}); err != nil {
			return err
		}
	}
	return nil
}

// RecentTopologyLinks, son gorulme zamanı penceresindeki kenarları dondurur.
func (s *sqlStore) RecentTopologyLinks(since time.Time) ([]TopologyLink, error) {
	rows, err := s.db.Query(s.q(`SELECT id, ts, kind, source_type, source_id, source_name, local_port,
		peer_type, peer_id, peer_name, peer_ip
		FROM topology_links WHERE ts >= ? ORDER BY source_type, source_id, kind, local_port`),
		since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TopologyLink{}
	for rows.Next() {
		var l TopologyLink
		if err := rows.Scan(&l.ID, &l.Ts, &l.Kind, &l.SourceType, &l.SourceID, &l.SourceName,
			&l.LocalPort, &l.PeerType, &l.PeerID, &l.PeerName, &l.PeerIP); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// PruneTopology, pencerenin disinda kalan (artik gorulmeyen) kenarlari siler.
func (s *sqlStore) PruneTopology(retention time.Duration) error {
	_, err := s.db.Exec(s.q(`DELETE FROM topology_links WHERE ts < ?`),
		time.Now().Add(-retention).Unix())
	return err
}

// --- istatistiksel baseline (Faz 6.2) ---
//
// Anomali motoru icin saatlik mevcut istatistikleri: std, avg(x^2)-avg(x)^2
// olarak Go tarafinda hesaplanir (SQL sqrt yerine — SQLite uyumlulugu).

type HourStat struct {
	Hour   int // 0-23, yerel saat dilimine gore
	Count  int64
	Mean   float64 // avg(bps_in + bps_out)
	MeanSq float64 // avg((bps_in + bps_out)^2)
}

// HourlyBpsStats, son 7 gunun saat-of-day baseline istatistiklerini dondurur.
func (s *sqlStore) HourlyBpsStats() ([]HourStat, error) {
	_, offset := time.Now().Zone()
	since := time.Now().Add(-7 * 24 * time.Hour).Unix()
	rows, err := s.db.Query(s.q(`SELECT ((ts + ?) % 86400)/3600 AS hour, COUNT(*),
			AVG(bps_in + bps_out), AVG((bps_in + bps_out) * (bps_in + bps_out))
		FROM samples WHERE ts >= ?
		GROUP BY hour ORDER BY hour`), offset, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HourStat{}
	for rows.Next() {
		var h HourStat
		if err := rows.Scan(&h.Hour, &h.Count, &h.Mean, &h.MeanSq); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// AvgBpsSince, penceredeki ortalama toplam verim (bps_in + bps_out).
func (s *sqlStore) AvgBpsSince(since time.Time) (float64, error) {
	var avg float64
	err := s.db.QueryRow(s.q(`SELECT COALESCE(AVG(bps_in + bps_out), 0) FROM samples WHERE ts >= ?`),
		since.Unix()).Scan(&avg)
	return avg, err
}

// DropStats, penceredeki dusen paket ve toplam paket sayisi (SLA raporu icin).
func (s *sqlStore) DropStats(since time.Time) (dropped uint64, pps uint64, err error) {
	row := s.db.QueryRow(s.q(`SELECT COALESCE(SUM(dropped), 0), COALESCE(SUM(pps), 0) FROM samples WHERE ts >= ?`),
		since.Unix())
	err = row.Scan(&dropped, &pps)
	return dropped, pps, err
}
