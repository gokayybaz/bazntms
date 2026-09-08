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
	// Confidence, kenarın güven düzeyi (Faz 23-D): discovered (SNMP/agent keşfi)
	// | inferred (trafik çıkarımı — açıkça işaretli) | manual (operatör). Boş →
	// UpsertTopologyLink "discovered" atar.
	Confidence string `json:"confidence"`
}

func (s *sqlStore) UpsertTopologyLink(l TopologyLink) error {
	if l.Ts == 0 {
		l.Ts = time.Now().Unix()
	}
	if l.Confidence == "" {
		l.Confidence = "discovered"
	}
	// ON CONFLICT confidence'ı KORUR — bir kez 'manual'/'inferred' işaretlenen
	// kenar rediscovery ile 'discovered'a düşmesin.
	_, err := s.db.Exec(s.q(`INSERT INTO topology_links
		(ts, kind, source_type, source_id, source_name, local_port, peer_type, peer_id, peer_name, peer_ip, confidence)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (kind, source_type, source_id, local_port, peer_name, peer_ip) DO UPDATE SET
			ts = excluded.ts, source_name = excluded.source_name,
			peer_type = excluded.peer_type, peer_id = excluded.peer_id`),
		l.Ts, l.Kind, l.SourceType, l.SourceID, l.SourceName, l.LocalPort,
		l.PeerType, l.PeerID, l.PeerName, l.PeerIP, l.Confidence)
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
		peer_type, peer_id, peer_name, peer_ip, confidence
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
			&l.LocalPort, &l.PeerType, &l.PeerID, &l.PeerName, &l.PeerIP, &l.Confidence); err != nil {
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

// --- istatistiksel baseline (Faz 6.2 · S22.2 mevsimsel + S22.3 çok-boyutlu) ---
//
// Baseline alt-toplam sorguları (mevsimsel kova × gün-yaşı, boyut bazlı) artık
// fleet_baseline.go'da (BaselineDayBuckets — dim="local"|"fleet"|"site"|"agent").
// Buradaki iki fonksiyon current-window karşılaştırması içindir.

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
