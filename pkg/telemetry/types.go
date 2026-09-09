// Package telemetry, agent ↔ hub arasinda paylasilan telgraf tipleridir.
// Faz 1 transportu JSON-over-HTTPS'tir; alan adlari telemetry.proto
// sozlesmesiyle birebir eslesir (protobuf'ya gecis icin hazir).
package telemetry

// ClampTS, agent'in bildirdigi batch zaman damgasini makul araliga sikistirir
// (S21.15 — saat kaymasi). Agent saati ileri/geri kaymissa örnekler yanlis
// zaman kovasina duser (rapor/grafik bozulur, TS retention erken siler). now
// hub'in su anki unix saniyesi. Kabul araligi: [now-7g, now+1g] — offline
// kuyruk replay'i (eski TS'li batch'ler) bu pencerede kalir, ham veri
// retention'i zaten 7g. Disi veya ts <= 0 → now.
func ClampTS(ts, now int64) int64 {
	const day = 86400
	if ts <= 0 || ts < now-7*day || ts > now+day {
		return now
	}
	return ts
}

// AgentHello, enrollment/ilk baglanti handshake'i.
type AgentHello struct {
	Name            string   `json:"name"`
	Site            string   `json:"site"`
	Version         string   `json:"version"`
	ProtocolVersion int      `json:"protocol_version"`
	OS              string   `json:"os"`
	Arch            string   `json:"arch"`
	Capabilities    []string `json:"capabilities,omitempty"`
	// MachineID, kararli makine kimligi hash'i (C3, Faz 13): state dosyasi
	// kaybolursa hub eslesen cevrimdisi kaydi yeniden kullanir, yeni satir
	// acmaz. Bos olabilir (kimlik alinamadi) — o zaman her hello yeni satir.
	MachineID string `json:"machine_id,omitempty"`
	// CSRPEM doluysa ve hub'da mTLS aciksa hub bunu bir istemci sertifikasina
	// donusturup HubReply.ClientCertPEM ile geri verir (agent'in bir sonraki
	// baglantidan itibaren kullandigi karsilikli TLS kimligi).
	CSRPEM string `json:"csr_pem,omitempty"`
}

// HubReply, AgentHello yaniti: kabul + agent kimligi + politika.
type HubReply struct {
	Accepted                 bool   `json:"accepted"`
	Reason                   string `json:"reason,omitempty"`
	AgentID                  int64  `json:"agent_id"`
	AgentToken               string `json:"agent_token,omitempty"`
	TelemetryIntervalSeconds int    `json:"telemetry_interval_seconds"`
	PCAPEnabled              bool   `json:"pcap_enabled"`
	// ProtocolVersion, hub'in konustugu protokol surumu. Agent bundan
	// yeniyse hub eskisini dayatir; agent bu degere gore degrade eder
	// (S21.15 — sert 401 yerine nazik degrade). 0 = eski hub, agent kendi
	// surumunu korur.
	ProtocolVersion int `json:"protocol_version,omitempty"`
	// mTLS: hub CA'si acikken CSR gonderilirse doldurulur.
	ClientCertPEM string `json:"client_cert_pem,omitempty"`
	CACertPEM     string `json:"ca_cert_pem,omitempty"`
}

// CertRequest, POST /api/v1/agent/cert govdesi — mevcut Bearer token ile
// kimliklenip suresi dolmak uzere olan istemci sertifikasini yeniler.
type CertRequest struct {
	CSRPEM string `json:"csr_pem"`
}

// CertReply, CertRequest yaniti.
type CertReply struct {
	ClientCertPEM string `json:"client_cert_pem"`
	CACertPEM     string `json:"ca_cert_pem"`
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

// L7Sample, agent'in bir donemde gozlemledigi surec bazli uygulama
// gorunurlugu: TLS ClientHello SNI'si veya HTTP Host'u. Hub tarafinda
// l7_endpoints tablosuna yazilir.
type L7Sample struct {
	PID      int32  `json:"pid"`
	Process  string `json:"process"`
	Kind     string `json:"kind"` // "tls" | "http"
	Host     string `json:"host"` // alan adi (SNI / Host header)
	RemoteIP string `json:"remote_ip"`
	Bytes    uint64 `json:"bytes"`
	Count    uint64 `json:"count"` // gozlem sayisi (delta)
}

// DNSSample, agent'in bir donemde gozlemledigi surec bazli DNS aktivitesi
// (sorulan domain + sorgu/yanit sayisi delta). Hub'da agent_dns tablosuna
// yazilir; fleet raporundaki "DNS görünürlüğü" bölümünü besler.
type DNSSample struct {
	PID       int32  `json:"pid"`
	Process   string `json:"process"`
	Domain    string `json:"domain"`
	Queries   uint64 `json:"queries"`
	Responses uint64 `json:"responses"`
}

// TelemetryBatch, agent'in periyodik toplu gonderimi.
type TelemetryBatch struct {
	TS int64 `json:"ts"`
	// Version/ProtocolVersion her batch'te gonderilir: enrollment yalnizca ilk
	// kayitta calistigi icin (kayitli agent hello'yu atlar) agent guncellendiginde
	// — MSI reinstall, self-update — hub'in bildigi surum aksi halde donuk kalir.
	// Bos/0 ise (eski agent) hub mevcut degeri korur.
	Version         string `json:"version,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`
	// AttrMethod, agent'in aktif surec-atif arka ucu: "ebpf" | "pcap" | "etw"
	// | "off". Bos = tasimayan eski agent (hub mevcut degeri korur). Yalnizca
	// gosterim/teshis — hub davranisini etkilemez.
	AttrMethod string `json:"attr_method,omitempty"`
	// AttrIface, pcap arka ucunun dinledigi yakalama arayuzu (yalnizca
	// method=pcap; eBPF/ETW soket duzeyinde, tum arayuzler). UI teshisi:
	// "motor calisiyor ama panel bos → yanlis/sanal arayuz mu yakalaniyor?"
	AttrIface string `json:"attr_iface,omitempty"`
	// AttrNote, atif motoru KAPALI/baslatilamadiysa insan-okur neden:
	// "collect.method=off" | "hub PCAP politikasi kapali (-agent-pcap=false)" |
	// pcap acilis hata ipucu. Motor calisirken "". UI panel bos-durum metnine
	// yansir.
	AttrNote       string                 `json:"attr_note,omitempty"`
	Interfaces     []InterfaceSample      `json:"interfaces"`
	Connections    []ConnectionSample     `json:"connections"`
	ProcessTraffic []ProcessTrafficSample `json:"process_traffic,omitempty"`
	L7             []L7Sample             `json:"l7,omitempty"`
	DNS            []DNSSample            `json:"dns,omitempty"`
	Subnets        []string               `json:"subnets,omitempty"` // yerel aglar (CIDR) — topoloji kesfi (Faz 6.1)
	DroppedPackets uint64                 `json:"dropped_packets,omitempty"`
}
