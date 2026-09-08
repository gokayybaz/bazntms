// Package ioc, tehdit istihbaratı domain listelerini (IOC — indicator of
// compromise) yükler ve gözlemlenen alan adlarını bunlara karşı eşleştirir.
// İmza tabanlı tam DPI değil: agent'ın zaten çıkardığı TLS SNI / HTTP Host /
// DNS alan adları bir kara listeye bakılır (bir hash-set lookup, ~sıfır maliyet).
package ioc

import (
	"bufio"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// List, domain + IP kara listesi. Dosyadan yüklenir; hosts / AdBlock / düz metin
// formatlarını kabul eder. Domain eşleştirmesi tam domain + üst alan içindir
// (sub.evil.com, listede evil.com varsa eşleşir); IP eşleştirmesi tamdır
// (Faz 24-E — `net.ParseIP` başarılı satırlar IP kümesine gider).
type List struct {
	path string

	mu    sync.RWMutex
	set   map[string]struct{} // domain'ler
	ips   map[string]struct{} // IP'ler
	mtime time.Time
}

// Load, listeyi dosyadan okur.
func Load(path string) (*List, error) {
	l := &List{path: path}
	if err := l.reload(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *List) reload() error {
	fi, err := os.Stat(l.path)
	if err != nil {
		return err
	}
	f, err := os.Open(l.path)
	if err != nil {
		return err
	}
	defer f.Close()

	set := make(map[string]struct{})
	ips := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		d := parseLine(sc.Text())
		if d == "" {
			continue
		}
		if ip := net.ParseIP(d); ip != nil {
			ips[ip.String()] = struct{}{}
		} else {
			set[d] = struct{}{}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	l.set = set
	l.ips = ips
	l.mtime = fi.ModTime()
	l.mu.Unlock()
	return nil
}

// Watch, dosyayı periyodik yoklar ve mtime değişince yeniden yükler.
// stop kapanınca döner.
func (l *List) Watch(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			fi, err := os.Stat(l.path)
			if err != nil {
				continue
			}
			l.mu.RLock()
			stale := fi.ModTime().After(l.mtime)
			l.mu.RUnlock()
			if !stale {
				continue
			}
			if err := l.reload(); err != nil {
				slog.Warn("IOC listesi yeniden yüklenemedi", "err", err)
			} else {
				slog.Info("IOC listesi yeniden yüklendi", "domain", l.Count())
			}
		}
	}
}

// Count, listedeki domain sayısı.
func (l *List) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.set)
}

// IPCount, listedeki IP sayısı (Faz 24-E).
func (l *List) IPCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.ips)
}

// MatchIP, verilen IP listede mi (tam eşleşme).
func (l *List) MatchIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.ips[ip.String()]
	return ok
}

// Match, domain'in kendisi veya bir üst alanı listede mi? Eşleşen giriş +
// bulundu döner. Tek etiketli TLD'ler (com, net…) tek başına eşleşmez.
func (l *List) Match(domain string) (string, bool) {
	d := Normalize(domain)
	if d == "" || !strings.Contains(d, ".") {
		return "", false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.set) == 0 {
		return "", false
	}
	for {
		if _, ok := l.set[d]; ok {
			return d, true
		}
		i := strings.IndexByte(d, '.')
		if i < 0 {
			return "", false
		}
		d = d[i+1:]
		if !strings.Contains(d, ".") { // tek etiket kaldı (TLD) — aşırı geniş, dur
			return "", false
		}
	}
}

// Normalize, bir alan adını karşılaştırma biçimine indirger: küçük harf,
// sondaki nokta / yol / port atılır.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "/\\"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 && !strings.Contains(s, "]") {
		s = s[:i]
	}
	return strings.TrimSuffix(s, ".")
}

// parseLine, çeşitli kara liste formatlarından bir domain çıkarır:
//
//	evil.com
//	0.0.0.0 evil.com            (hosts formatı)
//	127.0.0.1  evil.com  # not  (hosts + yorum)
//	||evil.com^                 (AdBlock)
//	# yorum / ! yorum / boş     → atlanır
func parseLine(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" || strings.HasPrefix(line, "!") {
		return ""
	}

	if strings.HasPrefix(line, "||") { // AdBlock: ||domain^
		line = strings.TrimPrefix(line, "||")
		line = strings.TrimRight(line, "^|")
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	cand := fields[0]
	switch cand { // hosts formatı: yönlendirme IP'si + domain
	case "0.0.0.0", "127.0.0.1", "::1", "::", "255.255.255.255":
		if len(fields) < 2 {
			return ""
		}
		cand = fields[1]
	}

	cand = Normalize(cand)
	if cand == "" || !strings.Contains(cand, ".") || strings.ContainsAny(cand, " \t*") {
		return ""
	}
	return cand
}
