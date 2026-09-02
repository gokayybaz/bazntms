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

// TelemetryReply, /api/v1/agent/telemetry yanitidir. Enrollment yalnizca ilk
// kayitta calistigi icin (kayitli agent hello'yu atlar) hub politikasi —
// telemetri araligi ve PCAP izni — agent'a her gonderimde bu yanitla
// tazelenir; agent bir sonraki dongude uygular. PCAPEnabled pointer'dir:
// nil (alan yok = eski hub) "degistirme" anlamina gelir, boylece agent
// enroll'dan gelen degeri korur.
type TelemetryReply struct {
	OK          bool  `json:"ok"`
	Interval    int   `json:"interval"`
	PCAPEnabled *bool `json:"pcap_enabled,omitempty"`
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

// ProcessTrafficSample, agent'in bir donem icinde surec bazli trafik
// farklaridir (delta). Hub tarafinda process_traffic tablosuna yazilir.
type ProcessTrafficSample struct {
	PID      int32  `json:"pid"`
	Process  string `json:"process"`
	Proto    string `json:"proto"`
	RemoteIP string `json:"remote_ip"`
	Port     uint16 `json:"port"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
}

// TelemetryBatch, agent'in periyodik toplu gonderimi.
type TelemetryBatch struct {
	TS             int64                  `json:"ts"`
	Interfaces     []InterfaceSample      `json:"interfaces"`
	Connections    []ConnectionSample     `json:"connections"`
	ProcessTraffic []ProcessTrafficSample `json:"process_traffic,omitempty"`
	Subnets        []string               `json:"subnets,omitempty"` // yerel aglar (CIDR) — topoloji kesfi (Faz 6.1)
	DroppedPackets uint64                 `json:"dropped_packets,omitempty"`
}
