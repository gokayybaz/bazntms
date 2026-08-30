package server

// Agent filo uclari (Faz 1): enrollment, telemetri, liste.
// UI auth'undan ayrı olarak agent token'lari (Bearer) ile korunur.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

const defaultTelemetryInterval = 30

// agentFromCtx, agentAuth middleware'inin yerlestirdigi kaydi dondurur.
type ctxKey string

const agentCtxKey ctxKey = "agent"

func agentFromCtx(r *http.Request) *store.Agent {
	if v, ok := r.Context().Value(agentCtxKey).(*store.Agent); ok {
		return v
	}
	return nil
}

// agentAuth, Bearer agent token'ini dogrular ve kaydi context'e koyar.
func (s *Server) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		token := ""
		if ah := r.Header.Get("Authorization"); len(ah) > len(prefix) && ah[:len(prefix)] == prefix {
			token = ah[len(prefix):]
		}
		if token == "" {
			unauthorized(w, "agent token gerekli")
			return
		}
		agent, err := s.store.AgentByTokenHash(store.TokenHash(token))
		if err != nil {
			unauthorized(w, "gecersiz agent token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), agentCtxKey, agent)))
	})
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// handleAgentHello, enrollment: X-Enroll-Token dogrulamasi ile agent kaydeder
// ve kalici agent token'i dondurur.
func (s *Server) handleAgentHello(w http.ResponseWriter, r *http.Request) {
	if s.enrollToken == "" {
		unauthorized(w, "enrollment kapalı: hub'i -enroll-token ile başlatın")
		return
	}
	if r.Header.Get("X-Enroll-Token") != s.enrollToken {
		unauthorized(w, "geçersiz enrollment token")
		return
	}
	var hello telemetry.AgentHello
	if err := json.NewDecoder(r.Body).Decode(&hello); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if hello.Name == "" {
		http.Error(w, "name zorunlu", http.StatusBadRequest)
		return
	}
	if hello.ProtocolVersion > maxProtocolVersion {
		unauthorized(w, "protokol surumu hub'dan yeni")
		return
	}

	buf := make([]byte, 32)
	rand.Read(buf)
	agentToken := hex.EncodeToString(buf)
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	id, err := s.store.RegisterAgent(store.Agent{
		Name:            hello.Name,
		Site:            hello.Site,
		TokenHash:       store.TokenHash(agentToken),
		Version:         hello.Version,
		ProtocolVersion: hello.ProtocolVersion,
		RemoteIP:        ip,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("agent enroll edildi", "agent_id", id, "name", hello.Name, "site", hello.Site, "ip", ip)
	writeJSON(w, telemetry.HubReply{
		Accepted:                 true,
		AgentID:                  id,
		AgentToken:               agentToken,
		TelemetryIntervalSeconds: s.telemetryInterval,
		PCAPEnabled:              s.agentPCAP,
	})
}

// handleAgentTelemetry, batch veriyi kaydeder ve last_seen gunceller.
// Kuyruk (ingest sink) yapilandirilmis ise batch JetStream'e yayinlanir ve
// yazim arka planda store-writer processor'u tarafindan yapilir (Faz 4.2).
func (s *Server) handleAgentTelemetry(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r)
	if agent == nil {
		unauthorized(w, "agent kimliği yok")
		return
	}
	var batch telemetry.TelemetryBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ts := batch.TS
	if ts == 0 {
		ts = time.Now().Unix()
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	if s.ingest != nil {
		if err := s.ingest.PublishTelemetry(agent.ID, agent.Version, ip, ts, &batch); err != nil {
			slog.Error("telemetri kuyrugu yayinlama hatasi", "agent_id", agent.ID, "err", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		slog.Debug("telemetri kuyruga alindi", "agent_id", agent.ID, "ifaces", len(batch.Interfaces))
		writeJSON(w, map[string]any{"ok": true, "interval": s.telemetryInterval})
		return
	}

	if err := s.store.SaveIfaceSamples(agent.ID, ts, batch.Interfaces); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.ReplaceConnLatest(agent.ID, batch.Connections); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.TouchAgent(agent.ID, agent.Version, ip); err != nil {
		slog.Error("agent touch hatasi", "agent_id", agent.ID, "err", err)
	}
	if len(batch.ProcessTraffic) > 0 {
		if err := s.store.SaveProcessTraffic(agent.ID, ts, batch.ProcessTraffic); err != nil {
			slog.Error("surec trafik kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	if len(batch.Subnets) > 0 {
		// topoloji kesfi (Faz 6.1): agent'in yerel aglari
		if err := s.store.SaveAgentSubnets(agent.ID, agent.Name, batch.Subnets); err != nil {
			slog.Error("subnet kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	slog.Debug("telemetri alindi", "agent_id", agent.ID, "ifaces", len(batch.Interfaces), "conns", len(batch.Connections))
	writeJSON(w, map[string]any{"ok": true, "interval": s.telemetryInterval})
}

// handleProcesses, surec bazli trafik top-listesi (UI auth ile korunur).
func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	agentID, _ := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.store.TopProcessTraffic(time.Now().Add(-time.Duration(minutes)*time.Minute), agentID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleAgentsList, UI icin filo gorunumu (UI auth'u ile korunur).
// Site-sinirli kimlikte (RBAC site scope) yalnizca kendi sitesi doner.
func (s *Server) handleAgentsList(w http.ResponseWriter, r *http.Request) {
	window := time.Duration(2*s.telemetryInterval) * time.Second
	agents, err := s.store.ListAgents(window, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, agents)
}

func (s *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	agent, err := s.store.AgentByID(id)
	if err != nil {
		http.Error(w, "agent bulunamadi", http.StatusNotFound)
		return
	}
	window := time.Duration(2*s.telemetryInterval) * time.Second
	agents, _ := s.store.ListAgents(window, SiteScope(identityFromCtx(r)))
	var withRates *store.AgentWithRates
	for i := range agents {
		if agents[i].ID == id {
			withRates = &agents[i]
			break
		}
	}
	if withRates == nil {
		withRates = &store.AgentWithRates{Agent: *agent, Online: false}
	}
	withRates.Conns = len(s.store.LatestAgentConnections(id))
	writeJSON(w, map[string]any{
		"agent":       withRates,
		"connections": s.store.LatestAgentConnections(id),
	})
}

// handleAgentHistory, Agent Detay sayfasindaki throughput grafigi icin
// zaman serisi dondurur (ThroughputChart ile ayni Bucket semasi).
func (s *Server) handleAgentHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	buckets, err := s.store.AgentHistory(id, time.Now().Add(-time.Duration(minutes)*time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, buckets)
}

func (s *Server) handleAgentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteAgent(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("agent silindi", "agent_id", id)
	s.audit(r, identityFromCtx(r), "agent.delete", fmt.Sprintf("agent:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

// version1, desteklenen en yuksek agent protokol surumu.
const maxProtocolVersion = 1

// version1 geriye uyumluluk icin korunuyor.
const version1 = maxProtocolVersion

func parseID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}
