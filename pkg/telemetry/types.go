// Package telemetry, agent ↔ hub arasinda paylasilan telgraf tipleridir.
// Faz 1 transportu JSON-over-HTTPS'tir; alan adlari telemetry.proto
// sozlesmesiyle birebir eslesir (protobuf'ya gecis icin hazir).
package telemetry

// AgentHello, enrollment/ilk baglanti handshake'i.
type AgentHello struct {
	Name            string   `json:"name"`
	Site            string   `json:"site"`
	Version         string   `json:"version"`
	ProtocolVersion int      `json:"protocol_version"`
	OS              string   `json:"os"`
	Arch            string   `json:"arch"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

// HubReply, AgentHello yaniti: kabul + agent kimligi + politika.
type HubReply struct {
	Accepted                 bool   `json:"accepted"`
	Reason                   string `json:"reason,omitempty"`
	AgentID                  int64  `json:"agent_id"`
	AgentToken               string `json:"agent_token,omitempty"`
	TelemetryIntervalSeconds int    `json:"telemetry_interval_seconds"`
	PCAPEnabled              bool   `json:"pcap_enabled"`
}

// InterfaceSample, arayuz bazli ham sayac degerleri (rate hub'da hesaplanir).
type InterfaceSample struct {
	Name      string `json:"name"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
}

// ConnectionSample, aktif soket envanteri.
type ConnectionSample struct {
	Proto      string `json:"proto"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Status     string `json:"status,omitempty"`
	PID        int32  `json:"pid"`
	Process    string `json:"process,omitempty"`
}

// TelemetryBatch, agent'in periyodik toplu gonderimi.
type TelemetryBatch struct {
	TS             int64              `json:"ts"`
	Interfaces     []InterfaceSample  `json:"interfaces"`
	Connections    []ConnectionSample `json:"connections"`
	DroppedPackets uint64             `json:"dropped_packets,omitempty"`
}
