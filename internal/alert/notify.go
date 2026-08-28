package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	}()
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
	default:
		return strings.ToUpper(kind)
	}
}
