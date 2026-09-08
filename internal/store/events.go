package store

import (
	"strings"
	"time"
)

// --- normalleştirilmiş olay akışı (Faz 24-A) ---
//
// Bkz. docs/decisions/0010-event-model.md: YENİ TABLO YOK. Mevcut kaynak
// tablolar (agent_dns / l7_endpoints / flows / syslog_events / connection_events)
// UNION ALL ile normalize edilmiş tek bir okuma modeline sunulur. İstenmeyen
// türlerin dalı hiç eklenmez → tarama sınırlı.

// Event, normalleştirilmiş tek bir gözlem.
type Event struct {
	Type     string `json:"type"`   // dns.query | tls.sni_observed | http.host_observed | netflow.flow | syslog.received | connection.seen
	Source   string `json:"source"` // agent | device | hub
	Ts       int64  `json:"ts"`
	AgentID  int64  `json:"agent_id,omitempty"`
	Device   string `json:"device,omitempty"`
	Process  string `json:"process,omitempty"`
	PID      int64  `json:"pid,omitempty"`
	SrcIP    string `json:"src_ip,omitempty"`
	SrcPort  int    `json:"src_port,omitempty"`
	DstIP    string `json:"dst_ip,omitempty"`
	DstPort  int    `json:"dst_port,omitempty"`
	Proto    string `json:"proto,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Severity string `json:"severity,omitempty"`
	Count    int64  `json:"count,omitempty"`
}

// EventFilter, QueryEvents parametreleri.
type EventFilter struct {
	Types   []string  // boş = tüm türler
	AgentID int64     // >0 → yalnız o agent (agent-kaynaklı türler)
	Device  string    // "" = tümü (device-kaynaklı türler)
	Since   time.Time // alt sınır
	Before  int64     // ts üst sınırı (imleç; 0 = şimdi)
	Limit   int
}

// eventSources, her olay türünün UNION ALL dalı. 15 kolon aynı sırada:
// type, source, ts, agent_id, device, process, pid, src_ip, src_port, dst_ip,
// dst_port, proto, domain, severity, cnt.
var eventSources = map[string]struct {
	branch      string // "?ts?" ve "?ex?" yer tutucuları: ts penceresi + ekstra filtre
	agentBased  bool
	deviceBased bool
}{
	"dns.query": {branch: `SELECT 'dns.query','agent',ts,agent_id,'',process,pid,'',0,'',0,'',domain,'',queries
		FROM agent_dns WHERE ts>=? AND ts<? AND domain<>'' ?ex?`, agentBased: true},
	"tls.sni_observed": {branch: `SELECT 'tls.sni_observed','agent',ts,agent_id,'',process,pid,'',0,remote_ip,0,'',host,'',hits
		FROM l7_endpoints WHERE ts>=? AND ts<? AND host<>'' AND kind='tls' ?ex?`, agentBased: true},
	"http.host_observed": {branch: `SELECT 'http.host_observed','agent',ts,agent_id,'',process,pid,'',0,remote_ip,0,'',host,'',hits
		FROM l7_endpoints WHERE ts>=? AND ts<? AND host<>'' AND kind='http' ?ex?`, agentBased: true},
	"netflow.flow": {branch: `SELECT 'netflow.flow','device',ts,0,device,'',0,src,src_port,dst,dst_port,proto,'','',packets
		FROM flows WHERE ts>=? AND ts<? ?ex?`, deviceBased: true},
	"syslog.received": {branch: `SELECT 'syslog.received','device',ts,0,host,'',0,source_ip,0,'',0,'','',CAST(severity AS TEXT),0
		FROM syslog_events WHERE ts>=? AND ts<? ?ex?`, deviceBased: true},
	"connection.seen": {branch: `SELECT 'connection.seen','hub',ts,0,'',process,pid,local_addr,0,remote_addr,0,proto,'',status,count
		FROM connection_events WHERE ts>=? AND ts<? ?ex?`},
}

// QueryEvents, filtreye uyan olayları ts azalan döndürür + bir sonraki sayfa
// imleci (son olayın ts'i; boş → daha fazla yok).
func (s *sqlStore) QueryEvents(f EventFilter) ([]Event, int64, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	before := f.Before
	if before <= 0 {
		before = time.Now().Unix() + 1
	}
	since := f.Since.Unix()

	want := f.Types
	if len(want) == 0 {
		want = make([]string, 0, len(eventSources))
		for k := range eventSources {
			want = append(want, k)
		}
	}

	var branches []string
	var args []any
	for _, typ := range want {
		src, ok := eventSources[typ]
		if !ok {
			continue
		}
		// agent/device filtresi türe uygunsa uygula
		ex := ""
		var exArgs []any
		if f.AgentID > 0 {
			if !src.agentBased {
				continue // agent filtresi verildi ama tür device-kaynaklı → dahil etme
			}
			ex = " AND agent_id=?"
			exArgs = append(exArgs, f.AgentID)
		}
		if f.Device != "" {
			if !src.deviceBased {
				continue
			}
			ex += " AND device=?"
			exArgs = append(exArgs, f.Device)
		}
		b := strings.Replace(src.branch, "?ex?", ex, 1)
		branches = append(branches, b)
		args = append(args, since, before)
		args = append(args, exArgs...)
	}
	if len(branches) == 0 {
		return []Event{}, 0, nil
	}

	// ORDER BY 3 = ts kolonu (pozisyonel — UNION dal adları belirsiz).
	q := strings.Join(branches, "\nUNION ALL\n") + "\nORDER BY 3 DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Type, &e.Source, &e.Ts, &e.AgentID, &e.Device, &e.Process, &e.PID,
			&e.SrcIP, &e.SrcPort, &e.DstIP, &e.DstPort, &e.Proto, &e.Domain, &e.Severity, &e.Count); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var next int64
	if len(out) == limit {
		next = out[len(out)-1].Ts
	}
	return out, next, nil
}
