package store

import (
	"database/sql"
	"time"
)

// --- olay (incident) korelasyonu (Faz 24-B) ---
//
// İlişkili uyarılar/olaylar deterministik kurallarla bir incident'a toplanır
// (internal/incident.Engine, lider-kapılı). Bkz. docs/decisions/0011.

type Incident struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Severity          string `json:"severity"` // info | warn | crit
	Status            string `json:"status"`   // open | investigating | resolved | closed
	Site              string `json:"site"`
	AgentID           int64  `json:"agent_id"`
	CorrelationKey    string `json:"correlation_key"`
	CorrelationReason string `json:"correlation_reason"`
	Summary           string `json:"summary"`
	RiskScore         int    `json:"risk_score"`
	CreatedTs         int64  `json:"created_ts"`
	UpdatedTs         int64  `json:"updated_ts"`
	FirstSeen         int64  `json:"first_seen"`
	LastSeen          int64  `json:"last_seen"`
	AckBy             string `json:"ack_by,omitempty"`
	AckTs             int64  `json:"ack_ts,omitempty"`
	ResolvedTs        int64  `json:"resolved_ts,omitempty"`
	ExtRef            string `json:"ext_ref,omitempty"`
}

type IncidentEvidence struct {
	Kind    string `json:"kind"` // alert | event
	Ref     string `json:"ref"`
	Ts      int64  `json:"ts"`
	Summary string `json:"summary"`
}

type IncidentFilter struct {
	Status   string // "" = tümü; "open" özel: open+investigating
	Severity string
	AgentID  int64
	Site     string
	Limit    int
}

const incidentCols = `id, title, severity, status, site, agent_id, correlation_key,
	correlation_reason, summary, risk_score, created_ts, updated_ts, first_seen,
	last_seen, ack_by, ack_ts, resolved_ts, ext_ref`

func scanIncident(sc interface{ Scan(...any) error }) (Incident, error) {
	var i Incident
	err := sc.Scan(&i.ID, &i.Title, &i.Severity, &i.Status, &i.Site, &i.AgentID,
		&i.CorrelationKey, &i.CorrelationReason, &i.Summary, &i.RiskScore,
		&i.CreatedTs, &i.UpdatedTs, &i.FirstSeen, &i.LastSeen,
		&i.AckBy, &i.AckTs, &i.ResolvedTs, &i.ExtRef)
	return i, err
}

// OpenIncidentByCorrelation, verilen anahtara sahip açık (open|investigating)
// incident'ı döndürür (dedup) — yoksa nil.
func (s *sqlStore) OpenIncidentByCorrelation(key string) (*Incident, error) {
	i, err := scanIncident(s.db.QueryRow(s.q(`SELECT `+incidentCols+`
		FROM incidents WHERE correlation_key = ? AND status IN ('open','investigating')
		ORDER BY id DESC LIMIT 1`), key))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *sqlStore) CreateIncident(in Incident) (int64, error) {
	now := time.Now().Unix()
	if in.CreatedTs == 0 {
		in.CreatedTs = now
	}
	in.UpdatedTs = now
	if in.FirstSeen == 0 {
		in.FirstSeen = now
	}
	if in.LastSeen == 0 {
		in.LastSeen = now
	}
	if in.Severity == "" {
		in.Severity = "warn"
	}
	if in.Status == "" {
		in.Status = "open"
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO incidents
		(title, severity, status, site, agent_id, correlation_key, correlation_reason,
		 summary, risk_score, created_ts, updated_ts, first_seen, last_seen)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		in.Title, in.Severity, in.Status, in.Site, in.AgentID, in.CorrelationKey,
		in.CorrelationReason, in.Summary, in.RiskScore, in.CreatedTs, in.UpdatedTs,
		in.FirstSeen, in.LastSeen).Scan(&id)
	return id, err
}

// BumpIncident, açık bir incident'ı tazeler: last_seen ilerler, severity yalnız
// YÜKSELİR (monoton), risk skoru en yükseği tutar, sebep/özet güncellenir.
func (s *sqlStore) BumpIncident(id int64, lastSeen int64, severity string, riskScore int, reason, summary string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(s.q(`UPDATE incidents SET
		last_seen = ?, updated_ts = ?,
		severity = CASE
			WHEN ? = 'crit' THEN 'crit'
			WHEN ? = 'warn' AND severity = 'info' THEN 'warn'
			ELSE severity END,
		risk_score = CASE WHEN ? > risk_score THEN ? ELSE risk_score END,
		correlation_reason = ?, summary = ?
		WHERE id = ?`),
		lastSeen, now, severity, severity, riskScore, riskScore, reason, summary, id)
	return err
}

func (s *sqlStore) AddIncidentEvidence(incidentID int64, ev IncidentEvidence) error {
	_, err := s.db.Exec(s.q(`INSERT INTO incident_evidence (incident_id, kind, ref, ts, summary)
		VALUES (?,?,?,?,?) ON CONFLICT (incident_id, kind, ref) DO NOTHING`),
		incidentID, ev.Kind, ev.Ref, ev.Ts, ev.Summary)
	return err
}

func (s *sqlStore) ListIncidents(f IncidentFilter) ([]Incident, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + incidentCols + ` FROM incidents WHERE 1=1`
	var args []any
	switch f.Status {
	case "":
		// tümü
	case "open":
		q += ` AND status IN ('open','investigating')`
	default:
		q += ` AND status = ?`
		args = append(args, f.Status)
	}
	if f.Severity != "" {
		q += ` AND severity = ?`
		args = append(args, f.Severity)
	}
	if f.AgentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, f.AgentID)
	}
	if f.Site != "" {
		q += ` AND site = ?`
		args = append(args, f.Site)
	}
	q += ` ORDER BY last_seen DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		in, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *sqlStore) IncidentByID(id int64) (*Incident, []IncidentEvidence, error) {
	in, err := scanIncident(s.db.QueryRow(s.q(`SELECT `+incidentCols+` FROM incidents WHERE id = ?`), id))
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.Query(s.q(`SELECT kind, ref, ts, summary FROM incident_evidence
		WHERE incident_id = ? ORDER BY ts ASC`), id)
	if err != nil {
		return &in, nil, err
	}
	defer rows.Close()
	var evs []IncidentEvidence
	for rows.Next() {
		var e IncidentEvidence
		if err := rows.Scan(&e.Kind, &e.Ref, &e.Ts, &e.Summary); err != nil {
			return &in, evs, err
		}
		evs = append(evs, e)
	}
	return &in, evs, rows.Err()
}

// SetIncidentStatus, durum geçişi (ack/investigate/resolve/close). ack "by"
// alanını da doldurur; resolve/close resolved_ts'i.
func (s *sqlStore) SetIncidentStatus(id int64, status, by string, ts int64) error {
	now := time.Now().Unix()
	q := `UPDATE incidents SET status = ?, updated_ts = ?`
	args := []any{status, now}
	if by != "" {
		q += `, ack_by = ?, ack_ts = ?`
		args = append(args, by, ts)
	}
	if status == "resolved" || status == "closed" {
		q += `, resolved_ts = ?`
		args = append(args, ts)
	}
	q += ` WHERE id = ?`
	args = append(args, id)
	_, err := s.db.Exec(s.q(q), args...)
	return err
}

func (s *sqlStore) SetIncidentExtRef(id int64, ref string) error {
	_, err := s.db.Exec(s.q(`UPDATE incidents SET ext_ref = ? WHERE id = ?`), ref, id)
	return err
}

// RecentOpenIncidents, `since`'ten beri aktivitesi olan açık incident'lar
// (sağlık skoru / rapor için).
func (s *sqlStore) RecentOpenIncidents(since time.Time) ([]Incident, error) {
	rows, err := s.db.Query(s.q(`SELECT `+incidentCols+` FROM incidents
		WHERE status IN ('open','investigating') AND last_seen >= ?
		ORDER BY last_seen DESC`), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		in, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}
