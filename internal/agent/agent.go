// Package agent, uclarda calisan telemetri istemcisini barindirir:
// enrollment (hello), periyodik TelemetryBatch gonderimi ve offline
// disk kuyrugu (baglanti yoksa batch'ler saklanir, geri gelince bosaltilir).
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gokayybaz/bazntms/internal/sysmon"
	"github.com/gokayybaz/bazntms/internal/version"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

const maxQueuedBatches = 100

// Options, agent istemcisi yapilandirmasi.
type Options struct {
	HubURL      string   // tek hub (geriye uyumlu)
	HubURLs     []string // birden cok hub: failover sirasi (Faz 5.4)
	EnrollToken string
	Name        string
	Site        string
	StateFile   string
	IntervalSec int
	HTTPTimeout time.Duration
	PCAPEnabled bool // hub politikasi: agent'ta derin toplama/PCAP izinli
}

// State, diskte tutulan kalici agent kimligi (token kaybolursa yeniden enroll gerekir).
type State struct {
	AgentID int64  `json:"agent_id"`
	Token   string `json:"token"`
}

type Client struct {
	opts   Options
	http   *http.Client
	hubIdx int // aktif hub indeksi (failover icin kayar)
}

func New(opts Options) *Client {
	if opts.IntervalSec <= 0 {
		opts.IntervalSec = 30
	}
	if opts.HTTPTimeout <= 0 {
		opts.HTTPTimeout = 15 * time.Second
	}
	if opts.HubURL != "" && len(opts.HubURLs) == 0 {
		opts.HubURLs = []string{opts.HubURL}
	}
	return &Client{opts: opts, http: &http.Client{Timeout: opts.HTTPTimeout}}
}

// urls, failover havuzunu dondurur; bos ise tek bos URL (hata mesaji icin).
func (c *Client) urls() []string {
	if len(c.opts.HubURLs) > 0 {
		return c.opts.HubURLs
	}
	return []string{""}
}

// withFailover, islemi hub havuzunda calistirir: aktif hub'dan baslar,
// hata halinde siradaki URL'yi dener; basarili hub'a yapisir (Faz 5.4).
func (c *Client) withFailover(fn func(baseURL string) error) error {
	pool := c.urls()
	var lastErr error
	for i := 0; i < len(pool); i++ {
		idx := (c.hubIdx + i) % len(pool)
		if err := fn(pool[idx]); err != nil {
			if len(pool) > 1 {
				slog.Warn("hub yanit vermedi, siradaki deneniyor", "hub", pool[idx], "err", err)
			}
			lastErr = err
			continue
		}
		c.hubIdx = idx
		return nil
	}
	return lastErr
}

// State, varsa diskten agent kimligini yukler.
func (c *Client) LoadState() State {
	var st State
	data, err := os.ReadFile(c.opts.StateFile)
	if err != nil {
		return st
	}
	json.Unmarshal(data, &st)
	return st
}

func (c *Client) saveState(st State) error {
	if err := os.MkdirAll(filepath.Dir(c.opts.StateFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.opts.StateFile, data, 0o600)
}

// Enroll, agent'i hub'a kaydeder ve kalici token alir (zaten kayitliysa atlar).
func (c *Client) Enroll() (State, error) {
	st := c.LoadState()
	if st.Token != "" && st.AgentID != 0 {
		return st, nil
	}
	if c.opts.EnrollToken == "" {
		return st, fmt.Errorf("enrollment token'i yok: hub'dan -enroll-token degerini alip config'e ekleyin")
	}
	hello := telemetry.AgentHello{
		Name:            c.opts.Name,
		Site:            c.opts.Site,
		Version:         version.Version,
		ProtocolVersion: version.ProtocolVersion,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
	}
	body, _ := json.Marshal(hello)

	var reply telemetry.HubReply
	err := c.withFailover(func(baseURL string) error {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/agent/hello", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Enroll-Token", c.opts.EnrollToken)
		return c.doJSON(req, &reply)
	})
	if err != nil {
		return st, err
	}
	if !reply.Accepted {
		return st, fmt.Errorf("enrollment reddedildi: %s", reply.Reason)
	}
	st = State{AgentID: reply.AgentID, Token: reply.AgentToken}
	if err := c.saveState(st); err != nil {
		return st, fmt.Errorf("state kaydi: %w", err)
	}
	slog.Info("enrollment tamamlandi", "agent_id", st.AgentID, "interval", reply.TelemetryIntervalSeconds)
	if reply.TelemetryIntervalSeconds > 0 {
		c.opts.IntervalSec = reply.TelemetryIntervalSeconds
	}
	c.opts.PCAPEnabled = reply.PCAPEnabled
	return st, nil
}

// PCAPEnabled, hub politikasina gore derin toplama/PCAP iznini dondurur.
func (c *Client) PCAPEnabled() bool { return c.opts.PCAPEnabled }

// Collect, bu anki telemetriyi toplar.
func (c *Client) Collect() telemetry.TelemetryBatch {
	batch := telemetry.TelemetryBatch{TS: time.Now().Unix()}
	for _, i := range sysmon.ListInterfaces() {
		batch.Interfaces = append(batch.Interfaces, telemetry.InterfaceSample{
			Name: i.Name, RxBytes: i.RxBytes, TxBytes: i.TxBytes,
			RxPackets: i.RxPackets, TxPackets: i.TxPackets,
		})
	}
	for _, cn := range sysmon.ListConnections() {
		batch.Connections = append(batch.Connections, telemetry.ConnectionSample{
			Proto: cn.Proto, LocalAddr: cn.LocalAddr, RemoteAddr: cn.RemoteAddr,
			Status: cn.Status, PID: cn.PID, Process: cn.Process,
		})
	}
	batch.Subnets = localSubnets()
	return batch
}

// localSubnets, yerel aglari CIDR olarak cikarir (loopback/link-local haric)
// — hub tarafinda topoloji haritasina islenir (Faz 6.1).
func localSubnets() []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range sysmon.ListInterfaces() {
		for _, a := range i.Addresses {
			ip, ipnet, err := net.ParseCIDR(a)
			if err != nil || ipnet == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			c := ipnet.String()
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// Send, batch'i hub'a gonderir; basarisizsa offline kuyruga yazip hatayi dondurur.
// Kuyrukta bekleyen batch'ler varsa once onlar bosaltilir.
func (c *Client) Send(st State, batch telemetry.TelemetryBatch) error {
	queue := c.loadQueue()
	if len(queue) > 0 {
		drained := queue[:0]
		for _, q := range queue {
			if err := c.postBatch(st, q.TS, q.Data); err != nil {
				drained = append(drained, q)
			}
		}
		queue = drained
		c.saveQueue(queue)
	}

	if err := c.postBatch(st, batch.TS, batch); err != nil {
		queue = append(queue, queuedBatch{TS: batch.TS, Data: batch})
		if len(queue) > maxQueuedBatches {
			queue = queue[len(queue)-maxQueuedBatches:]
		}
		c.saveQueue(queue)
		return err
	}
	return nil
}

type queuedBatch struct {
	TS   int64                    `json:"ts"`
	Data telemetry.TelemetryBatch `json:"data"`
}

func (c *Client) queuePath() string { return c.opts.StateFile + ".queue.jsonl" }

func (c *Client) loadQueue() []queuedBatch {
	data, err := os.ReadFile(c.queuePath())
	if err != nil {
		return nil
	}
	var out []queuedBatch
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var q queuedBatch
		if json.Unmarshal(line, &q) == nil {
			out = append(out, q)
		}
	}
	return out
}

func (c *Client) saveQueue(queue []queuedBatch) {
	var buf bytes.Buffer
	for _, q := range queue {
		line, _ := json.Marshal(q)
		buf.Write(line)
		buf.WriteByte('\n')
	}
	os.MkdirAll(filepath.Dir(c.queuePath()), 0o755)
	os.WriteFile(c.queuePath(), buf.Bytes(), 0o600)
}

func (c *Client) postBatch(st State, ts int64, batch telemetry.TelemetryBatch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	return c.withFailover(func(baseURL string) error {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/agent/telemetry", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.Token)
		var reply struct {
			OK       bool `json:"ok"`
			Interval int  `json:"interval"`
		}
		if err := c.doJSON(req, &reply); err != nil {
			return err
		}
		if reply.Interval > 0 && reply.Interval != c.opts.IntervalSec {
			c.opts.IntervalSec = reply.Interval
			slog.Debug("interval guncellendi", "interval", reply.Interval)
		}
		return nil
	})
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("hub %s: HTTP %d", req.URL.Path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Interval, guncel telemetri araligini dondurur (hub politikasiyla degisebilir).
func (c *Client) Interval() int { return c.opts.IntervalSec }
