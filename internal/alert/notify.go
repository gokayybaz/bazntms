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
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Notifier, olaylari masaustu bildirimi ve webhook'lara dagitir.
// Tum gonderimler asenkron ve hatayi loglayarak yutuyor: bildirim hatasi
// izleme isini durdurmamali.
type Notifier struct{}

func (n *Notifier) Deliver(nf Notifiers, ev store.AlertEvent) {
	title := fmt.Sprintf("bazNTMS [%s]", kindLabel(ev.Kind))
	go func() {
		if nf.Desktop {
			if err := sendDesktop(title, ev.Message); err != nil {
				log.Printf("masaustu bildirimi: %v", err)
			}
		}
		if nf.GenericURL != "" {
			payload := map[string]any{
				"ts": ev.Ts, "kind": ev.Kind, "key": ev.Key, "message": ev.Message, "title": title,
			}
			n.postJSON(nf.GenericURL, payload)
		}
		if nf.DiscordURL != "" {
			n.postJSON(nf.DiscordURL, map[string]string{"content": "**" + title + "**\n" + ev.Message})
		}
		if nf.SlackURL != "" {
			n.postJSON(nf.SlackURL, map[string]string{"text": "*" + title + "*\n" + ev.Message})
		}
		if nf.TelegramToken != "" && nf.TelegramChatID != "" {
			url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", nf.TelegramToken)
			n.postJSON(url, map[string]string{
				"chat_id": nf.TelegramChatID,
				"text":    title + "\n" + ev.Message,
			})
		}
		// Faz 6.3: kurumsal entegrasyonlar
		if nf.TeamsURL != "" {
			n.postJSON(nf.TeamsURL, map[string]any{
				"@type":      "MessageCard",
				"@context":   "http://schema.org/extensions",
				"themeColor": "0E7490",
				"title":      title,
				"text":       ev.Message,
			})
		}
		if nf.WebhookV2URL != "" {
			n.postWebhookV2(nf.WebhookV2URL, nf.WebhookV2Secret, ev, title)
		}
		if nf.EmailHost != "" && len(nf.EmailTo) > 0 {
			if err := sendEmail(nf, title, ev.Message); err != nil {
				log.Printf("e-posta bildirimi: %v", err)
			}
		}
		if nf.SIEM.Enabled {
			deliverSIEM(nf.SIEM, ev)
		}
	}()
}

// postWebhookV2, imzali webhook: govde HMAC-SHA256 ile imzalanir; alici
// tarafinda X-BazNTMS-Signature basligi dogrulanarak replay/spoof onlenir.
func (n *Notifier) postWebhookV2(url, secret string, ev store.AlertEvent, title string) {
	payload := map[string]any{
		"version": 2, "ts": ev.Ts, "kind": ev.Kind, "key": ev.Key,
		"message": ev.Message, "title": title,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BazNTMS-Signature", "sha256="+hmacSHA256Hex(secret, body))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		log.Printf("webhook v2: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("webhook v2 basarisiz (HTTP %d): %s", resp.StatusCode, url)
	}
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
	defer conn.Close()
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

func (n *Notifier) postJSON(url string, payload any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("webhook: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("webhook basarisiz (HTTP %d): %s", resp.StatusCode, url)
	}
}

// sendDesktop, platformun yerel bildirim mekanizmasini dener.
func sendDesktop(title, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		return exec.CommandContext(ctx, "osascript", "-e", script).Run()
	case "linux":
		return exec.CommandContext(ctx, "notify-send", title, message).Run()
	case "windows":
		// BurntToast modulu kuruluysa calisir; kurulu degilse hata loglanir, sorun degil
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
	case "vpn_down":
		return "VPN Düşüşü"
	case "sdwan_sla_breach":
		return "SD-WAN SLA"
	case "high_sessions":
		return "Oturum Eşiği"
	default:
		return strings.ToUpper(kind)
	}
}
