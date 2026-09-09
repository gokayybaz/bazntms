// Package fortigate, FortiOS REST API v2 istemcisidir (Faz 8.2, Faz 27 yeniden
// yazım).
//
// Özellikler:
//   - Bearer token kimlik doğrulama (System > Admin > REST API Admin token'ı;
//     token düz metin olarak yalnızca burada kullanılır, vault'ta şifreli durur)
//   - Self-signed sertifika için cihaz bazlı TLS verify toggle
//   - VDOM hedefleme: tek vdom veya "all" (tüm vdomlar taranır)
//   - cmdb uçlarında sayfalama, alan filtreleme, istekler arası hız koruması,
//     geçici hatalarda retry/backoff
//   - Toleranslı yanıt ayrıştırma (parse.go): `results` dizi/obje/tek-kayıt
//     olabilir, alan adları ve tipleri sürümler arası oynar — endpoint metotları
//     yalnızca mantıksal alanları sorar.
//   - Sürüm profili (profile.go): toleransla çözülemeyen sürüm farkları +
//     yanıt zarfındaki `version`'dan auto tespit.
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
	// Profile, sürüm profili. Boş/ID="auto" → ilk yanıtın `version` alanından
	// otomatik seçilir. ID doluysa kullanıcı pini kabul edilir (auto override).
	Profile Profile
	// VDOM, yalnızca Probe için hedef vdom ("" / "root" → parametre yok).
	// Poll yolunda vdom metot argümanıyla verilir.
	VDOM string
}

// Client, FortiGate REST API istemcisi.
type Client struct {
	opts Options
	http *http.Client

	mu            sync.Mutex
	lastReq       time.Time
	lastVersion   string
	lastBuild     int
	profile       Profile
	pinnedProfile bool
}

// New, istemciyi hazırlar.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.MinInterval <= 0 {
		opts.MinInterval = 150 * time.Millisecond
	}
	switch {
	case opts.MaxRetries < 0:
		opts.MaxRetries = 0 // açıkça retry'sız (probe)
	case opts.MaxRetries == 0:
		opts.MaxRetries = 2
	}
	transport := &http.Transport{}
	if !opts.VerifyTLS {
		//nolint:gosec // G402: FortiGate cihazlari genelde self-signed; VerifyTLS opt-in ile kapatilir
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	c := &Client{
		opts: opts,
		http: &http.Client{Timeout: opts.Timeout, Transport: transport},
	}
	if _, ok := ProfileByID(opts.Profile.ID); ok {
		c.profile = opts.Profile
		c.pinnedProfile = true
	} else {
		c.profile = defaultProfile()
	}
	return c
}

// Version, son yanıtta görülen FortiOS sürümü ("v7.2.11"); henüz istek
// yapılmadıysa boş.
func (c *Client) Version() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastVersion
}

// Build, son yanıtta görülen FortiOS build numarası.
func (c *Client) Build() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastBuild
}

// ProfileID, etkin profilin ID'si ("7.2", "default", ...).
func (c *Client) ProfileID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.profile.ID
}

// setVersion, yanıt zarfından sürümü kaydeder ve profil pinlenmemişse
// sürümden otomatik seçer.
func (c *Client) setVersion(v string, b int) {
	if v == "" {
		return
	}
	c.mu.Lock()
	c.lastVersion, c.lastBuild = v, b
	if !c.pinnedProfile {
		c.profile = ProfileForVersion(v)
	}
	c.mu.Unlock()
}

func (c *Client) sdwanPath() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.profile.SDWANPath()
}

// APIError, FortiOS'un 2xx-dışı yanıtı. Probe HTTP kodunu bununla okur.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	switch {
	case e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden:
		return fmt.Sprintf("fortigate yetki hatası: HTTP %d (token kapsamını/trusthost'u kontrol edin)", e.Status)
	case e.Body != "":
		return fmt.Sprintf("fortigate HTTP %d: %s", e.Status, truncateStr(e.Body, 200))
	default:
		return fmt.Sprintf("fortigate HTTP %d", e.Status)
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
	Serial     string          `json:"serial"`
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
		return false, &APIError{Status: resp.StatusCode}
	}
	if resp.StatusCode >= 500 {
		return true, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	if resp.StatusCode >= 300 {
		return false, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("fortigate yanıt zarfı: %w", err)
	}
	c.setVersion(env.Version, env.Build)
	if env.Status != "" && env.Status != "success" {
		return false, fmt.Errorf("fortigate status=%s", env.Status)
	}
	switch out := results.(type) {
	case *json.RawMessage:
		*out = env.Results
	case nil:
		// sonuç istenmiyor
	default:
		if len(env.Results) > 0 {
			if err := json.Unmarshal(env.Results, results); err != nil {
				return false, fmt.Errorf("fortigate results: %w", err)
			}
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

// getRaw, ham `results` gövdesini döndürür (toleranslı ayrıştırma için).
func (c *Client) getRaw(ctx context.Context, path string, q url.Values) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.get(ctx, path, q, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// getList, ham `results`'ı kayıt listesine indirger.
func (c *Client) getList(ctx context.Context, path string, q url.Values) ([]item, error) {
	raw, err := c.getRaw(ctx, path, q)
	if err != nil {
		return nil, err
	}
	return resultItems(raw), nil
}

// vdomQuery, vdom parametresini hazırlar (root/boş → parametre yok).
func vdomQuery(base url.Values, vdom string) url.Values {
	if base == nil {
		base = url.Values{}
	}
	if vdom != "" && vdom != "root" {
		base.Set("vdom", vdom)
	}
	return base
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

// ResourceSample, monitor/system/resource/usage anlık örneği.
type ResourceSample struct {
	Time    int64   `json:"time"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Disk    float64 `json:"disk"`
	Session float64 `json:"session"`
}

// Interface, monitor/system/interface girdisi (hesaplanmış hız dahil).
type Interface struct {
	ID        int64
	Name      string
	Alias     string
	IP        string
	Mask      string
	Status    string // "up" | "down"
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  uint64
	TxErrors  uint64
	RxDrops   uint64
	TxDrops   uint64
	speedBps  uint64
}

// SpeedBps, link hızını bit/sn olarak döndürür (0 = bilinmiyor).
func (i Interface) SpeedBps() uint64 { return i.speedBps }

// VPNTunnel, monitor/vpn/ipsec tünel girdisi.
type VPNTunnel struct {
	Name    string
	Status  string
	RxBytes uint64
	TxBytes uint64
	Peer    string
}

// SSLVPNSession, monitor/vpn/ssl bağlantısı.
type SSLVPNSession struct {
	User       string
	RemoteHost string
	Status     string
	Uptime     int64
	RxBytes    uint64
	TxBytes    uint64
}

// SDWANMember, health-check üyesi.
type SDWANMember struct {
	Member        string
	State         string
	Status        string
	LatencyMs     float64
	JitterMs      float64
	PacketLossPct float64
}

// Policy, firewall politikası (metadata + hit sayaçları).
type Policy struct {
	PolicyID int64
	Name     string
	Action   string
	Hits     uint64
	Bytes    uint64
}

// --- API metotları ---

// SystemStatus, cihaz kimliği/sürüm/uptime.
func (c *Client) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	raw, err := c.getRaw(ctx, "/api/v2/monitor/system/status", url.Values{})
	if err != nil {
		return nil, err
	}
	// results tek bir obje (dizi/anahtarlı-map değil) — doğrudan haritaya çöz.
	it := asItem(raw)
	if it == nil {
		if items := resultItems(raw); len(items) > 0 {
			it = items[0]
		}
	}
	if it == nil {
		return nil, fmt.Errorf("fortigate system/status boş")
	}
	return &SystemStatus{
		Version:  it.str("version"),
		Build:    int(it.u64("build")),
		Serial:   it.str("serial"),
		Hostname: it.str("hostname", "host_name"),
		Model:    it.str("model_name", "model"),
		Uptime:   int64(it.u64("uptime")),
	}, nil
}

// ResourceUsage, anlık kaynak kullanımı. `interval` parametresi bilinçli olarak
// gönderilmez — parametresiz çağrı FortiOS'ta güncel değerleri döndürür ve
// bazNTMS zaten yalnız son örneği kullanır. Yanıt biçimi sürüme göre
// {cpu:[{current:N}],...} map-of-arrays, {cpu:N,...} düz veya eski
// [{time,cpu,...}] dizisi olabilir — hepsi tolere edilir.
func (c *Client) ResourceUsage(ctx context.Context, vdom string) ([]ResourceSample, error) {
	raw, err := c.getRaw(ctx, "/api/v2/monitor/system/resource/usage", vdomQuery(nil, vdom))
	if err != nil {
		return nil, err
	}
	return parseResource(raw), nil
}

func parseResource(raw json.RawMessage) []ResourceSample {
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return nil
	}
	if t[0] == '[' {
		var arr []ResourceSample
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
			return arr
		}
		// dizi ama {cpu:{current}} öğeli olabilir → son öğe
		if items := resultItems(raw); len(items) > 0 {
			return []ResourceSample{sampleFromItem(items[len(items)-1])}
		}
		return nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	pick := func(keys ...string) float64 {
		for _, k := range keys {
			v, ok := obj[k]
			if !ok {
				continue
			}
			var f float64
			if json.Unmarshal(v, &f) == nil {
				return f
			}
			var arr []item
			if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
				return arr[len(arr)-1].f64("current", "value", "usage", "used")
			}
			if it := asItem(v); it != nil {
				return it.f64("current", "value", "usage", "used")
			}
		}
		return 0
	}
	s := ResourceSample{
		CPU:     pick("cpu", "cpu_usage"),
		Mem:     pick("mem", "memory", "mem_usage"),
		Disk:    pick("disk", "disk_usage"),
		Session: pick("session", "sessions", "session_count", "setuprate"),
	}
	if s == (ResourceSample{}) {
		return nil
	}
	return []ResourceSample{s}
}

func sampleFromItem(it item) ResourceSample {
	return ResourceSample{
		Time:    int64(it.u64("time")),
		CPU:     it.f64("cpu", "current"),
		Mem:     it.f64("mem", "memory"),
		Disk:    it.f64("disk"),
		Session: it.f64("session", "sessions"),
	}
}

// Interfaces, arayüz durum ve sayaçları (VLAN/aggregate dahil).
func (c *Client) Interfaces(ctx context.Context, vdom string) ([]Interface, error) {
	q := vdomQuery(url.Values{"include_vlan": {"true"}, "include_aggregate": {"true"}}, vdom)
	items, err := c.getList(ctx, "/api/v2/monitor/system/interface", q)
	if err != nil {
		return nil, err
	}
	out := make([]Interface, 0, len(items))
	for idx, it := range items {
		iface := Interface{
			ID:        stableIndex(it, idx+1),
			Name:      it.str("_key", "name"),
			Alias:     it.str("alias"),
			IP:        firstField(it.str("ip")),
			RxBytes:   it.u64("rx_bytes"),
			TxBytes:   it.u64("tx_bytes"),
			RxPackets: it.u64("rx_packets"),
			TxPackets: it.u64("tx_packets"),
			RxErrors:  it.u64("rx_errors"),
			TxErrors:  it.u64("tx_errors"),
			RxDrops:   it.u64("rx_drops", "rx_dropped", "rx_drop"),
			TxDrops:   it.u64("tx_drops", "tx_dropped", "tx_drop"),
		}
		// oper durum: link bool | status string | link.status.
		// Çözülemezse "down" (asla sahte "up" üretme).
		up, found := it.boolOr("link", "status", "line_status", "state")
		if !found {
			if sub := it.sub("link"); sub != nil {
				up, _ = sub.boolOr("status", "state", "up")
			}
		}
		if up {
			iface.Status = "up"
		} else {
			iface.Status = "down"
		}
		// hız: top-level speed (Mbps sayı) | link.speed ("1000FDX")
		if mbps := it.f64("speed"); mbps > 0 {
			iface.speedBps = uint64(mbps) * 1_000_000
		} else if sub := it.sub("link"); sub != nil {
			iface.speedBps = linkSpeedBps(sub.str("speed"))
		}
		out = append(out, iface)
	}
	return out, nil
}

// firstField, "1.2.3.4 255.255.255.0" gibi boşluk-ayrılmış değerin ilkini alır.
func firstField(s string) string {
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}

// VDOMs, cihazdaki vdom adları (multi-VDOM taraması için).
func (c *Client) VDOMs(ctx context.Context) ([]string, error) {
	items, err := c.getList(ctx, "/api/v2/cmdb/system/vdom", url.Values{"fields": {"name"}})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if n := it.str("name", "_key", "q_origin_key"); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// IPsecTunnels, IPsec tünel durumu.
func (c *Client) IPsecTunnels(ctx context.Context, vdom string) ([]VPNTunnel, error) {
	items, err := c.getList(ctx, "/api/v2/monitor/vpn/ipsec", vdomQuery(nil, vdom))
	if err != nil {
		return nil, err
	}
	out := make([]VPNTunnel, 0, len(items))
	for _, it := range items {
		t := VPNTunnel{
			Name:    it.str("name", "_key", "p1name"),
			Peer:    it.str("rgwy", "peer", "remote_gw", "rem_gw"),
			Status:  strings.ToLower(it.str("status")),
			RxBytes: it.u64("rx_bytes", "incoming_bytes"),
			TxBytes: it.u64("tx_bytes", "outgoing_bytes"),
		}
		// tünel-seviye status/bayt yoksa proxyid[] alt kayıtlarından türet
		if px := it.arr("proxyid"); len(px) > 0 {
			anyUp := false
			var prx, ptx uint64
			for _, p := range px {
				if strings.EqualFold(p.str("status"), "up") {
					anyUp = true
				}
				prx += p.u64("incoming_bytes", "rx_bytes")
				ptx += p.u64("outgoing_bytes", "tx_bytes")
			}
			if t.Status == "" {
				if anyUp {
					t.Status = "up"
				} else {
					t.Status = "down"
				}
			}
			if t.RxBytes == 0 {
				t.RxBytes = prx
			}
			if t.TxBytes == 0 {
				t.TxBytes = ptx
			}
		}
		if t.Status == "" {
			if it.u64("connection_count") > 0 {
				t.Status = "up"
			} else {
				t.Status = "down"
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// SSLVPNSessions, bağlı SSL-VPN kullanıcıları.
func (c *Client) SSLVPNSessions(ctx context.Context, vdom string) ([]SSLVPNSession, error) {
	raw, err := c.getRaw(ctx, "/api/v2/monitor/vpn/ssl", vdomQuery(nil, vdom))
	if err != nil {
		return nil, err
	}
	items := resultItems(unwrapKey(raw, "users"))
	out := make([]SSLVPNSession, 0, len(items))
	for _, it := range items {
		out = append(out, SSLVPNSession{
			User:       it.str("user_name", "user", "_key", "name"),
			RemoteHost: it.str("remote_host", "source_ip", "remote_ip"),
			Status:     it.str("status"),
			Uptime:     int64(it.u64("uptime", "duration")),
			RxBytes:    it.u64("rx", "in_bytes", "rx_bytes", "bytes_in"),
			TxBytes:    it.u64("tx", "out_bytes", "tx_bytes", "bytes_out"),
		})
	}
	return out, nil
}

// SDWANHealth, health-check üyesi metrikleri.
// results: {hcName: {members:[...]}} veya {hcName: {ifname: {...}}}.
func (c *Client) SDWANHealth(ctx context.Context, vdom string) (map[string][]SDWANMember, error) {
	raw, err := c.getRaw(ctx, c.sdwanPath(), vdomQuery(nil, vdom))
	if err != nil {
		return nil, err
	}
	out := map[string][]SDWANMember{}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return out, nil
	}
	for hc, v := range top {
		hcItem := asItem(v)
		if hcItem == nil {
			continue
		}
		if members := hcItem.arr("members"); len(members) > 0 {
			for _, m := range members {
				out[hc] = append(out[hc], sdwanMember(m, m.str("name", "_key", "interface")))
			}
			continue
		}
		// üyeler ifname-anahtarlı
		for ifname, mv := range hcItem {
			mi := asItem(mv)
			if mi == nil {
				continue
			}
			out[hc] = append(out[hc], sdwanMember(mi, ifname))
		}
	}
	return out, nil
}

func sdwanMember(it item, name string) SDWANMember {
	return SDWANMember{
		Member:        name,
		State:         it.str("status", "state"),
		Status:        it.str("status"),
		LatencyMs:     it.f64("latency"),
		JitterMs:      it.f64("jitter"),
		PacketLossPct: it.f64("packet_loss", "packetloss", "packet_loss_pct"),
	}
}

// Policies, firewall politikaları — metadata (cmdb) + hit sayaçları (monitor),
// policyid ile birleştirilir. Bir kaynak başarısız olursa diğerinden gelen
// kısmi veri yine döner.
func (c *Client) Policies(ctx context.Context, vdom string) ([]Policy, error) {
	const pageSize = 500
	byID := map[int64]*Policy{}
	var order []int64
	track := func(id int64) *Policy {
		p, ok := byID[id]
		if !ok {
			p = &Policy{PolicyID: id}
			byID[id] = p
			order = append(order, id)
		}
		return p
	}

	var cmdbErr, monErr error

	// 1. cmdb: ad + aksiyon (yalnız geçerli alanlar; sayfalı)
	for start := 0; ; start += pageSize {
		q := vdomQuery(url.Values{
			"start":  {strconv.Itoa(start)},
			"count":  {strconv.Itoa(pageSize)},
			"fields": {"policyid,name,action"},
		}, vdom)
		items, err := c.getList(ctx, "/api/v2/cmdb/firewall/policy", q)
		if err != nil {
			cmdbErr = err
			break
		}
		for _, it := range items {
			id := int64(it.u64("policyid", "q_origin_key"))
			if id == 0 {
				continue
			}
			p := track(id)
			p.Name = it.str("name")
			p.Action = it.str("action")
		}
		if len(items) < pageSize {
			break
		}
	}

	// 2. monitor: hit/bayt sayaçları
	if items, err := c.getList(ctx, "/api/v2/monitor/firewall/policy", vdomQuery(nil, vdom)); err != nil {
		monErr = err
	} else {
		for _, it := range items {
			id := int64(it.u64("policyid", "_key"))
			if id == 0 {
				continue
			}
			p := track(id)
			p.Hits = it.u64("hit_count", "session_count", "active_sessions", "packets")
			p.Bytes = it.u64("bytes")
		}
	}

	if len(order) == 0 {
		if cmdbErr != nil {
			return nil, cmdbErr
		}
		if monErr != nil {
			return nil, monErr
		}
		return nil, nil
	}
	out := make([]Policy, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}
