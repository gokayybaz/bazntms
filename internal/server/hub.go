package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/store"
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
	alerts  *alert.Manager

	store        store.Store
	onlineWindow time.Duration
	fleetMu      sync.Mutex
	fleet        store.FleetSummary
	fleetAt      time.Time
}

func NewHub(alerts *alert.Manager) *Hub {
	h := &Hub{
		clients: map[*websocket.Conn]struct{}{},
		tick:    time.NewTicker(time.Second),
		alerts:  alerts,
	}
	go h.broadcastLoop()
	return h
}

// setFleetSource, WS tick'inde yayınlanacak filo özetinin kaynağını bağlar
// (server.New'den). store nil ise tick yalnızca alarm olaylarını taşır.
func (h *Hub) setFleetSource(st store.Store, telemetryInterval int) {
	h.store = st
	h.onlineWindow = time.Duration(2*telemetryInterval) * time.Second
}

// fleetSummary, filo özetini ~3 sn önbellekle döndürür: WS 1 sn'de bir tick
// atsa ve N istemci bağlı olsa bile DB'ye saniyede birden fazla gitmez.
func (h *Hub) fleetSummary() *store.FleetSummary {
	if h.store == nil {
		return nil
	}
	h.fleetMu.Lock()
	defer h.fleetMu.Unlock()
	if time.Since(h.fleetAt) < 3*time.Second {
		fs := h.fleet
		return &fs
	}
	fs, err := h.store.FleetSummary(h.onlineWindow)
	if err != nil {
		slog.Debug("fleet özeti alınamadı", "err", err)
		if h.fleetAt.IsZero() {
			return nil
		}
		fs = h.fleet // eski değeri kullan
	}
	h.fleet, h.fleetAt = fs, time.Now()
	f := fs
	return &f
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
	slog.Info("ws istemci baglandi", "toplam", h.count())

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		_ = conn.Close()
		slog.Info("ws istemci ayrildi", "toplam", h.count())
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
	AlertEvents []store.AlertEvent  `json:"alert_events"`
	Fleet       *store.FleetSummary `json:"fleet,omitempty"`
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

		payload := tickPayload{
			Type:        "tick",
			AlertEvents: h.alerts.RecentEvents(20),
			Fleet:       h.fleetSummary(),
		}
		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		for _, c := range clients {
			_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				h.mu.Lock()
				delete(h.clients, c)
				h.mu.Unlock()
				_ = c.Close()
			}
		}
	}
}
