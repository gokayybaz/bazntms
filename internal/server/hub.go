package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/store"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	tick    *time.Ticker
	alerts  *alert.Manager

	upgrader websocket.Upgrader
	// allowedOrigins, Cross-Site WebSocket Hijacking'e karsi izin listesi (B5):
	// host (kucuk harf, portsuz) → true. Bos ise tum origin'ler kabul edilir
	// (bugunku davranis) + bir kez uyari loglanir.
	allowedOrigins map[string]bool
	originWarned   atomic.Bool

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
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 64 * 1024,
		CheckOrigin:     h.checkOrigin,
	}
	go h.broadcastLoop()
	return h
}

// setAllowedOrigins, WS origin izin listesini ayarlar (host adlari; port/scheme
// yok sayilir). server.SetWSOrigins → main (-public-url + -tls-hosts + localhost).
func (h *Hub) setAllowedOrigins(hosts []string) {
	set := map[string]bool{}
	for _, x := range hosts {
		if hn := originHost(strings.TrimSpace(x)); hn != "" {
			set[hn] = true
		}
	}
	h.allowedOrigins = set
}

// checkOrigin, WS handshake origin denetimi (B5 — CSWSH savunmasi).
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // tarayici disi istemci / same-origin
	}
	oh := originHost(origin)
	if oh == "" {
		return false
	}
	// same-origin: Origin host == istek Host'u
	if oh == originHost(r.Host) {
		return true
	}
	if len(h.allowedOrigins) == 0 {
		if h.originWarned.CompareAndSwap(false, true) {
			slog.Warn("WS origin izin listesi bos — tum origin'ler kabul ediliyor; -public-url ile sinirlayin", "origin", origin)
		}
		return true
	}
	return h.allowedOrigins[oh]
}

// originHost, bir Origin / Host degerinden kucuk-harf, portsuz host cikarir.
func originHost(v string) string {
	if v == "" {
		return ""
	}
	if !strings.Contains(v, "://") {
		v = "//" + v // "host:port" → parse edilebilir
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
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
	conn, err := h.upgrader.Upgrade(w, r, nil)
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
