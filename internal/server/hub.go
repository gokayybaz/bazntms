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
		conn.Close()
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
	Type        string             `json:"type"`
	AlertEvents []store.AlertEvent `json:"alert_events"`
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
