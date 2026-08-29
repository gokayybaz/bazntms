package server

// Topoloji haritasi ucu (Faz 6.1): cihazlar + agent'lar + kesif kenarlari
// tek grafikte birlestirilir. UI bunu canli ag haritasina cevirir.

import (
	"net/http"
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

type topologyGraph struct {
	GeneratedAt int64                `json:"generated_at"`
	Devices     []topoDevice         `json:"devices"`
	Agents      []topoAgent          `json:"agents"`
	Links       []store.TopologyLink `json:"links"`
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices()
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

	graph := topologyGraph{
		GeneratedAt: time.Now().Unix(),
		Devices:     []topoDevice{},
		Agents:      []topoAgent{},
		Links:       links,
	}
	now := time.Now().Unix()
	for _, d := range devices {
		online := d.LastPoll > 0 && now-d.LastPoll < int64(3*d.PollSeconds)
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
