package store

import (
	"fmt"
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

// --- istatistiksel baseline (Faz 6.2 · S22.2 mevsimsel + EWMA) ---
//
// Anomali motorunun baseline'ı: (mevsimsel kova × gün-yaşı) alt-toplamları.
// std, alert katmanında sqrt(Σw·x² / Σw − mean²) olarak hesaplanır (SQL sqrt
// yerine — SQLite uyumluluğu). Kaynak: hub yerel yakalaması (`samples`);
// çoklu-hub'da boş → FleetBaselineDayBuckets devreye girer.

// BaselineDayBuckets, son `days` günün hub-yerel baseline'ını (mevsimsel kova ×
// gün-yaşı) alt-toplamları olarak döndürür.
func (s *sqlStore) BaselineDayBuckets(days int, seasonality string) ([]BaselineDayBucket, error) {
	if days <= 0 {
		days = 21
	}
	_, offset := time.Now().Zone()
	now := time.Now().Unix()
	since := now - int64(days)*86400
	q := fmt.Sprintf(`SELECT %s AS bucket, (%d - ts) / 86400 AS day_age, COUNT(*),
			COALESCE(SUM(bps_in + bps_out), 0),
			COALESCE(SUM((bps_in + bps_out) * (bps_in + bps_out)), 0)
		FROM samples WHERE ts >= ?
		GROUP BY bucket, day_age`, seasonalBucketExpr(seasonality, fmt.Sprintf("(ts + %d)", offset)), now)
	rows, err := s.db.Query(s.q(q), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaselineDayBucket{}
	for rows.Next() {
		var b BaselineDayBucket
		if err := rows.Scan(&b.Bucket, &b.DayAge, &b.N, &b.Sum, &b.SumSq); err != nil {
			return nil, err
		}
		out = append(out, b)
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
