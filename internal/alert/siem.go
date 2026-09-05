package alert

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// SIEMConfig, uyarı olaylarını bir SIEM/ITSM sistemine standart formatta
// iletir. Format: CEF (ArcSight), LEEF (QRadar) veya düz JSON. Taşıma: RFC3164
// syslog (UDP/TCP) ya da HTTP POST (Splunk HEC, ServiceNow, jenerik toplayıcı).
type SIEMConfig struct {
	Enabled   bool   `json:"enabled"`
	Format    string `json:"format"`    // cef | leef | json | text  (boş → cef)
	Transport string `json:"transport"` // syslog-udp | syslog-tcp | http  (boş → syslog-udp)
	Target    string `json:"target"`    // syslog: host:port · http: tam URL
	Token     string `json:"token"`     // http: Authorization başlığı (ör. "Splunk <hec-token>"); boş → gönderilmez
	Insecure  bool   `json:"insecure"`  // http: TLS sunucu doğrulamasını atla (self-signed toplayıcı)
}

const (
	siemVendor  = "bazNTMS"
	siemProduct = "bazNTMS"
	siemVersion = "1.0"
)

// deliverSIEM, tek bir olayı yapılandırılmış SIEM hedefine iletir. Dönen
// hata çağırana (Notifier) kanal durumu için verilir; ayrıca loglanır.
func deliverSIEM(c SIEMConfig, ev store.AlertEvent) error {
	if !c.Enabled {
		return nil
	}
	if c.Target == "" {
		return fmt.Errorf("SIEM hedefi (target) boş")
	}

	var payload, ctype string
	switch strings.ToLower(c.Format) {
	case "leef":
		payload, ctype = formatLEEF(ev), "text/plain; charset=utf-8"
	case "json":
		b, _ := json.Marshal(siemJSON(ev))
		payload, ctype = string(b), "application/json"
	case "text":
		// düz syslog satırı: yalnızca insan-okunur mesaj (klasik SOC syslog toplama)
		payload, ctype = fmt.Sprintf("bazNTMS [%s] %s", kindLabel(ev.Kind), ev.Message), "text/plain; charset=utf-8"
	default: // cef
		payload, ctype = formatCEF(ev), "text/plain; charset=utf-8"
	}

	var err error
	switch strings.ToLower(c.Transport) {
	case "http", "https":
		err = siemHTTP(c, payload, ctype)
	case "syslog-tcp":
		err = siemSyslog("tcp", c.Target, ev, payload)
	default: // syslog-udp
		err = siemSyslog("udp", c.Target, ev, payload)
	}
	if err != nil {
		log.Printf("SIEM bildirimi (%s/%s): %v", c.Format, c.Transport, err)
		return fmt.Errorf("%s/%s: %w", c.Format, c.Transport, err)
	}
	return nil
}

// --- formatlar ---

// formatCEF, ArcSight Common Event Format 0.
//
//	CEF:0|Vendor|Product|Version|SignatureID|Name|Severity|Extension
func formatCEF(ev store.AlertEvent) string {
	ext := fmt.Sprintf("rt=%d cat=%s cs1Label=key cs1=%s msg=%s",
		ev.Ts*1000, ev.Kind, cefExt(ev.Key), cefExt(ev.Message))
	return fmt.Sprintf("CEF:0|%s|%s|%s|%s|%s|%d|%s",
		cefHdr(siemVendor), cefHdr(siemProduct), cefHdr(siemVersion),
		cefHdr(ev.Kind), cefHdr(kindLabel(ev.Kind)), severityCEF(ev.Kind), ext)
}

// formatLEEF, IBM QRadar Log Event Extended Format 1.0 (sekme ayraçlı).
func formatLEEF(ev store.AlertEvent) string {
	attrs := strings.Join([]string{
		fmt.Sprintf("devTime=%d", ev.Ts*1000),
		fmt.Sprintf("sev=%d", severityCEF(ev.Kind)),
		"cat=" + ev.Kind,
		"key=" + leefVal(ev.Key),
		"msg=" + leefVal(ev.Message),
	}, "\t")
	return fmt.Sprintf("LEEF:1.0|%s|%s|%s|%s|%s",
		siemVendor, siemProduct, siemVersion, ev.Kind, attrs)
}

func siemJSON(ev store.AlertEvent) map[string]any {
	return map[string]any{
		"vendor": siemVendor, "product": siemProduct,
		"ts": ev.Ts, "kind": ev.Kind, "kind_label": kindLabel(ev.Kind),
		"key": ev.Key, "message": ev.Message, "severity": severityCEF(ev.Kind),
	}
}

// cefHdr, CEF başlık alanı kaçışı: `\` ve `|`.
func cefHdr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "|", `\|`)
}

// cefExt, CEF uzantı değeri kaçışı: `\`, `=` ve satır sonu.
func cefExt(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "=", `\=`)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\r", " ")
}

// leefVal, LEEF öznitelik değerinden ayraç/satır sonu temizler.
func leefVal(s string) string {
	r := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	return r.Replace(s)
}

// severityCEF, uyarı türünü 0-10 önem skalasına oturtur (CEF ve LEEF ortak).
func severityCEF(kind string) int {
	switch kind {
	case "ioc":
		return 10 // bilinen kötü domain'e temas
	case "port":
		return 9
	case "vpn_down":
		return 8
	case "anomaly", "sdwan_sla_breach":
		return 6
	case "bw", "high_sessions":
		return 5
	case "proc", "target":
		return 4
	default:
		return 5
	}
}

// severitySyslog, aynı türü RFC3164 önem koduna (0-7) indirir.
func severitySyslog(kind string) int {
	switch s := severityCEF(kind); {
	case s >= 9:
		return 2 // critical
	case s >= 7:
		return 3 // error
	case s >= 5:
		return 4 // warning
	default:
		return 5 // notice
	}
}

// --- taşımalar ---

// siemSyslog, RFC3164 çerçevesinde CEF/LEEF/JSON satırını gönderir.
func siemSyslog(network, target string, ev store.AlertEvent, msg string) error {
	host, _ := os.Hostname()
	if host == "" {
		host = "bazntms"
	}
	pri := 16*8 + severitySyslog(ev.Kind) // facility local0 (16)
	line := fmt.Sprintf("<%d>%s %s bazntms: %s",
		pri, time.Now().Format("Jan _2 15:04:05"), host, msg)
	if network == "tcp" {
		line += "\n"
	}

	conn, err := net.DialTimeout(network, target, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(line))
	return err
}

// siemHTTP, formatlanmış olayı hedef URL'ye POST eder.
func siemHTTP(c SIEMConfig, payload, ctype string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Target, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ctype)
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}

	client := http.DefaultClient
	if c.Insecure {
		client = &http.Client{
			Timeout: 10 * time.Second,
			//nolint:gosec // G402: SIEM kolektorleri genelde ic self-signed; Insecure opt-in
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
