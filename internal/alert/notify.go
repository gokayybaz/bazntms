package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Bildirim kanalı kimlikleri (durum haritası + metrik etiketi).
const (
	ChDesktop   = "desktop"
	ChGeneric   = "generic"
	ChDiscord   = "discord"
	ChSlack     = "slack"
	ChTelegram  = "telegram"
	ChTeams     = "teams"
	ChWebhookV2 = "webhook_v2"
	ChEmail     = "email"
	ChSIEM      = "siem"
)

// ChannelStatus, bir bildirim kanalının son teslim denemesinin sonucu (D3).
type ChannelStatus struct {
	LastAttempt int64  `json:"last_attempt"` // unix; 0 = hiç denenmedi
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
}

// Notifier, olaylari masaustu bildirimi ve webhook'lara dagitir. Tum
// gonderimler asenkron; her denemenin sonucu kanal bazinda saklanir
// (Status) ve hata olursa onFail metrik kancasi cagrilir.
type Notifier struct {
	mu     sync.Mutex
	status map[string]ChannelStatus
	onFail func(channel string)
}

func NewNotifier() *Notifier {
	return &Notifier{status: map[string]ChannelStatus{}}
}

// SetFailHook, kanal başına teslim hatasında çağrılacak metrik kancasını
// ayarlar (server Prometheus counter'ına bağlanır).
func (n *Notifier) SetFailHook(fn func(channel string)) {
	n.mu.Lock()
	n.onFail = fn
	n.mu.Unlock()
}

// Status, tüm kanalların son durumlarının kopyasını döndürür.
func (n *Notifier) Status() map[string]ChannelStatus {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make(map[string]ChannelStatus, len(n.status))
	for k, v := range n.status {
		out[k] = v
	}
	return out
}

func (n *Notifier) record(channel string, err error) {
	n.mu.Lock()
	st := ChannelStatus{LastAttempt: time.Now().Unix(), OK: err == nil}
	if err != nil {
		st.Error = err.Error()
	}
	n.status[channel] = st
	hook := n.onFail
	n.mu.Unlock()
	if err != nil {
		log.Printf("bildirim [%s]: %v", channel, err)
		if hook != nil {
			hook(channel)
		}
	}
}

func (n *Notifier) deliver(channel string, fn func() error) {
	n.record(channel, fn())
}

func (n *Notifier) Deliver(nf Notifiers, ev store.AlertEvent) {
	go n.dispatch(nf, ev)
}

// Test, tüm etkin kanallara sentetik bir uyarı gönderir (senkron) ve güncel
// durumları döndürür — panelden "Test Et" için.
func (n *Notifier) Test(nf Notifiers) map[string]ChannelStatus {
	ev := store.AlertEvent{
		Ts:      time.Now().Unix(),
		Kind:    "test",
		Key:     "manual",
		Message: "bazNTMS test bildirimi — kanal yapılandırmasını doğrulamak için gönderildi.",
	}
	n.dispatch(nf, ev)
	return n.Status()
}

// dispatch, yapılandırmadaki etkin kanalların her birine ev'i iletir ve
// sonucu kaydeder. Deliver bunu goroutine'de, Test senkron çağırır.
func (n *Notifier) dispatch(nf Notifiers, ev store.AlertEvent) {
	title := fmt.Sprintf("bazNTMS [%s]", kindLabel(ev.Kind))
	if nf.Desktop {
		n.deliver(ChDesktop, func() error { return sendDesktop(title, ev.Message) })
	}
	if nf.GenericURL != "" {
		n.deliver(ChGeneric, func() error {
			return postJSON(nf.GenericURL, map[string]any{
				"ts": ev.Ts, "kind": ev.Kind, "key": ev.Key, "message": ev.Message, "title": title,
			})
		})
	}
	if nf.DiscordURL != "" {
		n.deliver(ChDiscord, func() error {
			return postJSON(nf.DiscordURL, map[string]string{"content": "**" + title + "**\n" + ev.Message})
		})
	}
	if nf.SlackURL != "" {
		n.deliver(ChSlack, func() error {
			return postJSON(nf.SlackURL, map[string]string{"text": "*" + title + "*\n" + ev.Message})
		})
	}
	if nf.TelegramToken != "" && nf.TelegramChatID != "" {
		n.deliver(ChTelegram, func() error {
			url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", nf.TelegramToken)
			return postJSON(url, map[string]string{"chat_id": nf.TelegramChatID, "text": title + "\n" + ev.Message})
		})
	}
	if nf.TeamsURL != "" {
		n.deliver(ChTeams, func() error {
			return postJSON(nf.TeamsURL, map[string]any{
				"@type": "MessageCard", "@context": "http://schema.org/extensions",
				"themeColor": "0E7490", "title": title, "text": ev.Message,
			})
		})
	}
	if nf.WebhookV2URL != "" {
		n.deliver(ChWebhookV2, func() error { return postWebhookV2(nf.WebhookV2URL, nf.WebhookV2Secret, ev, title) })
	}
	if nf.EmailHost != "" && len(nf.EmailTo) > 0 {
		n.deliver(ChEmail, func() error { return sendEmail(nf, title, ev.Message) })
	}
	if nf.SIEM.Enabled {
		n.deliver(ChSIEM, func() error { return deliverSIEM(nf.SIEM, ev) })
	}
}

// postWebhookV2, imzali webhook: govde HMAC-SHA256 ile imzalanir; alici
// tarafinda X-BazNTMS-Signature basligi dogrulanarak replay/spoof onlenir.
func postWebhookV2(url, secret string, ev store.AlertEvent, title string) error {
	payload := map[string]any{
		"version": 2, "ts": ev.Ts, "kind": ev.Kind, "key": ev.Key,
		"message": ev.Message, "title": title,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BazNTMS-Signature", "sha256="+hmacSHA256Hex(secret, body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func hmacSHA256Hex(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// sendEmail, STARTTLS destekli basit SMTP gonderimi (net/smtp).
func sendEmail(nf Notifiers, subject, body string) error {
	port := nf.EmailPort
	if port == 0 {
		port = 587
	}
	host := nf.EmailHost
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if ok, _ := conn.Extension("STARTTLS"); ok {
		if err := conn.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if nf.EmailUser != "" {
		auth := smtp.PlainAuth("", nf.EmailUser, nf.EmailPass, host)
		if err := conn.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := conn.Mail(nf.EmailFrom); err != nil {
		return err
	}
	for _, to := range nf.EmailTo {
		if err := conn.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := conn.Data()
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		nf.EmailFrom, strings.Join(nf.EmailTo, ", "), subject, body)
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return conn.Quit()
}

func postJSON(url string, payload any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// sendDesktop, platformun yerel bildirim mekanizmasini dener.
func sendDesktop(title, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// G204: title/message uyari icerigi — sabit argv slotlarina gecer (kabuk
	// yok). Yalnizca 'desktop' bildirim kanali secilince, operatorun kendi
	// makinesinde calisir.
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		return exec.CommandContext(ctx, "osascript", "-e", script).Run() //nolint:gosec // G204
	case "linux":
		return exec.CommandContext(ctx, "notify-send", title, message).Run() //nolint:gosec // G204
	case "windows":
		// BurntToast modulu kuruluysa calisir; kurulu degilse hata loglanir, sorun degil
		//nolint:gosec // G204
		cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command",
			fmt.Sprintf("New-BurntToastNotification -Text %q, %q", title, message))
		return cmd.Run()
	default:
		return fmt.Errorf("bu platform masaustu bildirimini desteklemiyor")
	}
}

func kindLabel(kind string) string {
	switch kind {
	case "bw":
		return "Bant Genişliği"
	case "port":
		return "Şüpheli Port"
	case "proc":
		return "Yeni Süreç"
	case "target":
		return "Yeni Hedef"
	case "anomaly":
		return "Anomali"
	case "ioc":
		return "IOC / Tehdit"
	case "vpn_down":
		return "VPN Düşüşü"
	case "sdwan_sla_breach":
		return "SD-WAN SLA"
	case "high_sessions":
		return "Oturum Eşiği"
	case "test":
		return "Test"
	default:
		return strings.ToUpper(kind)
	}
}
