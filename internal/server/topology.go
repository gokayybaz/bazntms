package server

// Topoloji haritasi ucu (Faz 6.1): cihazlar + agent'lar + kesif kenarlari
// tek grafikte birlestirilir. UI bunu canli ag haritasina cevirir.

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type topoDevice struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Kind    string `json:"kind"`
	SysName string `json:"sys_name"`
	Online  bool   `json:"online"`
}

type topoAgent struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Site   string `json:"site"`
	Online bool   `json:"online"`
}

// edgeTelemetry, SNMP arayüz telemetrisi bir topoloji kenarına bağlandığında
// (Faz 23-D) — source_type='device' + local_port bir ifName'e eşleşince.
type edgeTelemetry struct {
	IfName     string  `json:"if_name"`
	OperStatus int     `json:"oper_status"`
	SpeedBps   uint64  `json:"speed_bps"`
	RxBps      float64 `json:"rx_bps"`
	TxBps      float64 `json:"tx_bps"`
	RxUtilPct  float64 `json:"rx_util_pct"`
	TxUtilPct  float64 `json:"tx_util_pct"`
	Class      string  `json:"class"`
	Errors     uint64  `json:"errors"`   // in + out
	Discards   uint64  `json:"discards"` // in + out
}

type topoLink struct {
	store.TopologyLink
	Telemetry *edgeTelemetry `json:"telemetry,omitempty"`
}

type topologyGraph struct {
	GeneratedAt int64        `json:"generated_at"`
	Devices     []topoDevice `json:"devices"`
	Agents      []topoAgent  `json:"agents"`
	Links       []topoLink   `json:"links"`
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices(SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	agents, err := s.store.ListAgents(2*time.Duration(s.telemetryInterval)*time.Second, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	links, err := s.store.RecentTopologyLinks(time.Now().Add(-24 * time.Hour))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// S14.B4: site-kısıtlı kimlik için kenarları görünür düğümlere daralt —
	// RecentTopologyLinks tüm sahaları döndürür (bir agent'ın subnet kenarı
	// aksi halde başka sahaya sızardı). Global kimlikte (scope=="") tümü kalır.
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		vis := map[string]bool{}
		for _, d := range devices {
			vis["device:"+strconv.FormatInt(d.ID, 10)] = true
		}
		for _, a := range agents {
			vis["agent:"+strconv.FormatInt(a.ID, 10)] = true
		}
		nodeVisible := func(typ string, id int64) bool {
			if typ != "agent" && typ != "device" {
				return true // host / harici uç — kaynak zaten görünürse sorun yok
			}
			return vis[typ+":"+strconv.FormatInt(id, 10)]
		}
		scoped := links[:0]
		for _, l := range links {
			if nodeVisible(l.SourceType, l.SourceID) && nodeVisible(l.PeerType, l.PeerID) {
				scoped = append(scoped, l)
			}
		}
		links = scoped
	}

	// Faz 23-D: source_type='device' + local_port bir ifName'e eşleşen kenarlara
	// canlı SNMP arayüz telemetrisi bağla. Eşleşme yoksa kenar telemetrisiz
	// render edilir (graf bozulmaz). LatestDeviceIfaces cihaz başına bir kez.
	ifaceCache := map[int64][]store.DeviceIfaceRate{}
	tLinks := make([]topoLink, 0, len(links))
	for _, l := range links {
		tl := topoLink{TopologyLink: l}
		if l.SourceType == "device" && l.LocalPort != "" && l.SourceID > 0 {
			ifs, ok := ifaceCache[l.SourceID]
			if !ok {
				ifs, _ = s.store.LatestDeviceIfaces(l.SourceID)
				ifaceCache[l.SourceID] = ifs
			}
			for _, r := range ifs {
				if r.Name == l.LocalPort {
					tl.Telemetry = &edgeTelemetry{
						IfName: r.Name, OperStatus: r.OperStatus, SpeedBps: r.SpeedBitsPS,
						RxBps: r.RxBps, TxBps: r.TxBps,
						RxUtilPct: r.RxUtilPct, TxUtilPct: r.TxUtilPct, Class: r.Class,
						Errors:   r.InErrors + r.OutErrors,
						Discards: r.InDiscards + r.OutDiscards,
					}
					break
				}
			}
		}
		tLinks = append(tLinks, tl)
	}

	graph := topologyGraph{
		GeneratedAt: time.Now().Unix(),
		Devices:     []topoDevice{},
		Agents:      []topoAgent{},
		Links:       tLinks,
	}
	now := time.Now().Unix()
	for _, d := range devices {
		// online = yakında ve BAŞARIYLA poll edildi. Son poll hata verdiyse
		// (ör. SNMP sessizce zaman aşımına uğradı, IF-MIB boş döndü) cihaz
		// çevrimdışı sayılır — yoksa "poll edildi" ile "veri geliyor" karışır.
		online := d.LastPoll > 0 && now-d.LastPoll < int64(3*d.PollSeconds) && d.LastError == ""
		graph.Devices = append(graph.Devices, topoDevice{
			ID: d.ID, Name: d.Name, Host: d.Host, Kind: d.Kind,
			SysName: d.SysName, Online: online,
		})
	}
	for _, a := range agents {
		graph.Agents = append(graph.Agents, topoAgent{
			ID: a.ID, Name: a.Name, Site: a.Site, Online: a.Online,
		})
	}
	writeJSON(w, graph)
}
