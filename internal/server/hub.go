package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/sysmon"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // ayni makine uzerinde calisir
	},
}

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	tick    *time.Ticker
	engine  *capture.Engine
	alerts  *alert.Manager
	geo     *geoip.Resolver
}

func NewHub(engine *capture.Engine, alerts *alert.Manager, geo *geoip.Resolver) *Hub {
	h := &Hub{
		clients: map[*websocket.Conn]struct{}{},
		tick:    time.NewTicker(time.Second),
		engine:  engine,
		alerts:  alerts,
		geo:     geo,
	}
	go h.broadcastLoop()
	return h
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
	log.Printf("ws istemci baglandi (toplam: %d)", h.count())

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
		log.Printf("ws istemci ayrildi (toplam: %d)", h.count())
	}()

	// istemciden gelen ping/pong ve kapatma mesajlarini oku
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

type tickPayload struct {
	Type        string              `json:"type"`
	Stats       *capture.Snapshot   `json:"stats"`
	Connections []sysmon.Connection `json:"connections"`
	AlertEvents []store.AlertEvent  `json:"alert_events"`
	Record      capture.RecordInfo  `json:"record"`
}

func (h *Hub) broadcastLoop() {
	for range h.tick.C {
		h.mu.Lock()
		if len(h.clients) == 0 {
			h.mu.Unlock()
			continue
		}
		clients := make([]*websocket.Conn, 0, len(h.clients))
		for c := range h.clients {
			clients = append(clients, c)
		}
		h.mu.Unlock()

		snap := h.engine.Snapshot()
		if h.geo != nil && h.geo.Enabled() {
			for i := range snap.TopEndpoints {
				info := h.geo.Lookup(snap.TopEndpoints[i].IP)
				snap.TopEndpoints[i].Country = info.Country
				snap.TopEndpoints[i].ASN = info.ASN
			}
		}
		cons := sysmon.ListConnections()
		if h.geo != nil && h.geo.Enabled() {
			for i := range cons {
				if cons[i].RemoteAddr == "" {
					continue
				}
				if host, _, err := net.SplitHostPort(cons[i].RemoteAddr); err == nil {
					info := h.geo.Lookup(host)
					cons[i].Country = info.Country
					cons[i].ASN = info.ASN
				}
			}
		}
		payload := tickPayload{
			Type:        "tick",
			Stats:       snap,
			Connections: cons,
			AlertEvents: h.alerts.RecentEvents(20),
			Record:      h.engine.RecordStatus(),
		}
		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		for _, c := range clients {
			c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				h.mu.Lock()
				delete(h.clients, c)
				h.mu.Unlock()
				c.Close()
			}
		}
	}
}
