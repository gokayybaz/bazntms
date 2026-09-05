// Package fortigate, FortiOS REST API v2 istemcisidir (Faz 8.2).
//
// Özellikler:
//   - Bearer token kimlik doğrulama (System > Admin > REST API Admin token'ı;
//     token düz metin olarak yalnızca burada kullanılır, vault'ta şifreli durur)
//   - Self-signed sertifika için cihaz bazlı TLS verify toggle
//   - VDOM hedefleme: tek vdom veya "all" (tüm vdomlar taranır)
//   - cmdb uçlarında sayfalama, alan filtreleme, istekler arası hız koruması,
//     geçici hatalarda retry/backoff
//   - Toleranslı yanıt ayrıştırma: `results` dizi veya map olabilir; eksik
//     alanlar sıfır kabul edilir. FortiOS sürümü arasında şema farkı olan
//     uçlarda bilinen şemalar denenir, uymayan sessizce atlanır.
package fortigate

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options, istemci yapılandırması.
type Options struct {
	BaseURL     string        // örn. https://10.0.0.1 (port dahil olabilir)
	Token       string        // REST API token (düz metin; vault'ta şifreli saklanır)
	VerifyTLS   bool          // false → self-signed kabul edilir
	Timeout     time.Duration // istek zaman aşımı (0 → 20 sn)
	MinInterval time.Duration // istekler arası asgari süre (0 → 150 ms, yönetim CPU koruması)
	MaxRetries  int           // 5xx/geçici hatalarda deneme sayısı (0 → 2)
}

// Client, FortiGate REST API istemcisi.
type Client struct {
	opts Options
	http *http.Client

	mu      sync.Mutex
	lastReq time.Time
}

// New, istemciyi hazırlar.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.MinInterval <= 0 {
		opts.MinInterval = 150 * time.Millisecond
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 2
	}
	transport := &http.Transport{}
	if !opts.VerifyTLS {
		//nolint:gosec // G402: FortiGate cihazlari genelde self-signed; VerifyTLS opt-in ile kapatilir
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		opts: opts,
		http: &http.Client{Timeout: opts.Timeout, Transport: transport},
	}
}

// envelope, FortiOS yanıt zarfı: {"http_method":..., "results":..., "status":"success", "version":"v7.4.4", "build":...}
type envelope struct {
	HTTPMethod string          `json:"http_method"`
	Results    json.RawMessage `json:"results"`
	VDOM       json.RawMessage `json:"vdom"`
	Status     string          `json:"status"`
	Version    string          `json:"version"`
	Build      int             `json:"build"`
}

// get, GET isteğini hız koruması + retry ile yürütür ve results'ı çözer.
func (c *Client) get(ctx context.Context, path string, q url.Values, results any) error {
	var lastErr error
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
		}
		if err := c.pace(ctx); err != nil {
			return err
		}
		done, err := c.getOnce(ctx, path, q, results)
		if err == nil {
			return nil
		}
		lastErr = err
		if !done { // kalıcı hata (4xx): retry anlamsız
			return err
		}
	}
	return lastErr
}

// pace, istekler arasındaki asgari süreyi korur (yönetim CPU bütçesi).
func (c *Client) pace(ctx context.Context) error {
	c.mu.Lock()
	wait := time.Duration(0)
	if !c.lastReq.IsZero() {
		if elapsed := time.Since(c.lastReq); elapsed < c.opts.MinInterval {
			wait = c.opts.MinInterval - elapsed
		}
	}
	c.lastReq = time.Now().Add(wait)
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}
	return nil
}

// getOnce, tek istek; dönüş: retry anlamlı mı.
func (c *Client) getOnce(ctx context.Context, path string, q url.Values, results any) (bool, error) {
	u := strings.TrimRight(c.opts.BaseURL, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.opts.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return true, err // ağ hatası: retry ok
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return true, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, fmt.Errorf("fortigate yetki hatası: HTTP %d (token kapsamını kontrol edin)", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return true, fmt.Errorf("fortigate HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("fortigate HTTP %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("fortigate yanıt zarfı: %w", err)
	}
	if env.Status != "" && env.Status != "success" {
		return false, fmt.Errorf("fortigate status=%s", env.Status)
	}
	if results != nil && len(env.Results) > 0 {
		if err := json.Unmarshal(env.Results, results); err != nil {
			return false, fmt.Errorf("fortigate results: %w", err)
		}
	}
	return false, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- tipler (toleranslı) ---

// SystemStatus, monitor/system/status çıktısı.
type SystemStatus struct {
	Version  string `json:"version"`
	Build    int    `json:"build"`
	Serial   string `json:"serial"`
	Hostname string `json:"hostname"`
	Model    string `json:"model_name"`
	Uptime   int64  `json:"uptime"` // saniye
}

// ResourceSample, monitor/system/resource/usage örnekleri.
type ResourceSample struct {
	Time    int64   `json:"time"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Disk    float64 `json:"disk"`
	Session float64 `json:"session"`
}

// LinkInfo, arayüzün fiziksel bağlantı bilgisi.
type LinkInfo struct {
	Speed string `json:"speed"` // örn. "1000FDX"
}

// Interface, monitor/system/interface girdisi.
type Interface struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Alias     string    `json:"alias"`
	IP        string    `json:"ip"`
	Mask      string    `json:"mask"`
	Status    string    `json:"status"`
	Link      *LinkInfo `json:"link,omitempty"`
	RxBytes   uint64    `json:"rx_bytes"`
	TxBytes   uint64    `json:"tx_bytes"`
	RxPackets uint64    `json:"rx_packets"`
	TxPackets uint64    `json:"tx_packets"`
	RxErrors  uint64    `json:"rx_errors"`
	TxErrors  uint64    `json:"tx_errors"`
	RxDrops   uint64    `json:"rx_drops"`
	TxDrops   uint64    `json:"tx_drops"`
}

// SpeedBps, link hızını bps'e çevirir ("1000FDX" → 1e9; boş → 0).
func (i Interface) SpeedBps() uint64 {
	s := ""
	if i.Link != nil {
		s = i.Link.Speed
	}
	digits := ""
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		} else {
			break
		}
	}
	mbps, err := strconv.ParseUint(digits, 10, 64)
	if err != nil || mbps == 0 {
		return 0
	}
	return mbps * 1_000_000
}

// VPNTunnel, monitor/vpn/ipsec tünel girdisi (bilinen alanlar).
type VPNTunnel struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
	Peer    string `json:"peer"`
}

// SSLVPNSession, monitor/vpn/ssl bağlantısı.
type SSLVPNSession struct {
	User       string `json:"user"`
	RemoteHost string `json:"remote_host"`
	Status     string `json:"status"`
	Uptime     int64  `json:"uptime"`
	RxBytes    uint64 `json:"rx"`
	TxBytes    uint64 `json:"tx"`
}

// SDWANMember, health-check üyesi.
type SDWANMember struct {
	Member        string  `json:"name"`
	State         string  `json:"state"`
	Status        string  `json:"status"`
	LatencyMs     float64 `json:"latency"`
	JitterMs      float64 `json:"jitter"`
	PacketLossPct float64 `json:"packet_loss"`
}

// Policy, cmdb/firewall/policy girdisi.
type Policy struct {
	PolicyID int64  `json:"policyid"`
	Name     string `json:"name"`
	Action   string `json:"action"`
	Hits     uint64 `json:"hit_count"`
	Bytes    uint64 `json:"bytes"`
}

// --- API metotları ---

// SystemStatus, cihaz kimliği/sürüm/uptime.
func (c *Client) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	var res SystemStatus
	if err := c.get(ctx, "/api/v2/monitor/system/status", url.Values{}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ResourceUsage, pencere başına kaynak örnekleri (interval: 1minute|10minutes|1hour...).
func (c *Client) ResourceUsage(ctx context.Context, vdom, interval string) ([]ResourceSample, error) {
	q := url.Values{"interval": {interval}}
	if vdom != "" && vdom != "root" {
		q.Set("vdom", vdom)
	}
	var res []ResourceSample
	if err := c.get(ctx, "/api/v2/monitor/system/resource/usage", q, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// Interfaces, arayüz durum ve sayaçları (VLAN/aggregate dahil).
func (c *Client) Interfaces(ctx context.Context, vdom string) ([]Interface, error) {
	q := url.Values{"include_vlan": {"true"}, "include_aggregate": {"true"}}
	if vdom != "" && vdom != "root" {
		q.Set("vdom", vdom)
	}
	var res []Interface
	if err := c.get(ctx, "/api/v2/monitor/system/interface", q, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// VDOMs, cihazdaki vdom adları (multi-VDOM taraması için).
func (c *Client) VDOMs(ctx context.Context) ([]string, error) {
	var res []struct {
		Name string `json:"name"`
	}
	q := url.Values{"fields": {"name"}}
	if err := c.get(ctx, "/api/v2/cmdb/system/vdom", q, &res); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res))
	for _, v := range res {
		if v.Name != "" {
			out = append(out, v.Name)
		}
	}
	return out, nil
}

// IPsecTunnels, IPsec tünel durumu. results dizi veya map olabilir → toleranslı.
func (c *Client) IPsecTunnels(ctx context.Context, vdom string) ([]VPNTunnel, error) {
	q := url.Values{}
	if vdom != "" && vdom != "root" {
		q.Set("vdom", vdom)
	}
	var raw json.RawMessage
	if err := c.get(ctx, "/api/v2/monitor/vpn/ipsec", q, &raw); err != nil {
		return nil, err
	}
	return parseTunnels(raw), nil
}

func parseTunnels(raw json.RawMessage) []VPNTunnel {
	out := []VPNTunnel{}
	if len(raw) == 0 {
		return out
	}
	var arr []VPNTunnel
	if err := json.Unmarshal(raw, &arr); err == nil {
		return append(out, arr...)
	}
	var m map[string]VPNTunnel
	if err := json.Unmarshal(raw, &m); err == nil {
		for name, t := range m {
			if t.Name == "" {
				t.Name = name
			}
			out = append(out, t)
		}
	}
	return out
}

// SSLVPNSessions, bağlı SSL-VPN kullanıcıları.
func (c *Client) SSLVPNSessions(ctx context.Context, vdom string) ([]SSLVPNSession, error) {
	q := url.Values{}
	if vdom != "" && vdom != "root" {
		q.Set("vdom", vdom)
	}
	var raw json.RawMessage
	if err := c.get(ctx, "/api/v2/monitor/vpn/ssl", q, &raw); err != nil {
		return nil, err
	}
	out := []SSLVPNSession{}
	if len(raw) == 0 {
		return out, nil
	}
	// results dizi veya {users: [...]} olabilir
	var arr []SSLVPNSession
	if err := json.Unmarshal(raw, &arr); err == nil {
		return append(out, arr...), nil
	}
	var wrap struct {
		Users []SSLVPNSession `json:"users"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil {
		return append(out, wrap.Users...), nil
	}
	return out, nil
}

// SDWANHealth, health-check üyesi metrikleri. results: map[hcName]{members:[...]} veya dizi.
func (c *Client) SDWANHealth(ctx context.Context, vdom string) (map[string][]SDWANMember, error) {
	q := url.Values{}
	if vdom != "" && vdom != "root" {
		q.Set("vdom", vdom)
	}
	var raw json.RawMessage
	if err := c.get(ctx, "/api/v2/monitor/virtual-wan/health-check", q, &raw); err != nil {
		return nil, err
	}
	out := map[string][]SDWANMember{}
	if len(raw) == 0 {
		return out, nil
	}
	var m map[string]struct {
		Members []SDWANMember `json:"members"`
	}
	if err := json.Unmarshal(raw, &m); err == nil && len(m) > 0 {
		for hc, v := range m {
			out[hc] = v.Members
		}
		return out, nil
	}
	var arr []struct {
		Name    string        `json:"name"`
		Members []SDWANMember `json:"members"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, h := range arr {
			out[h.Name] = h.Members
		}
	}
	return out, nil
}

// Policies, firewall policy listesi (hit sayaçlarıyla) — sayfalı.
func (c *Client) Policies(ctx context.Context, vdom string) ([]Policy, error) {
	const pageSize = 500
	var out []Policy
	for start := 0; ; start += pageSize {
		q := url.Values{
			"start":  {strconv.Itoa(start)},
			"count":  {strconv.Itoa(pageSize)},
			"fields": {"policyid,name,action,hit_count,bytes"},
		}
		if vdom != "" && vdom != "root" {
			q.Set("vdom", vdom)
		}
		var page []Policy
		if err := c.get(ctx, "/api/v2/cmdb/firewall/policy", q, &page); err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < pageSize {
			return out, nil
		}
	}
}
