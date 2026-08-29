//go:build darwin

package proctraffic

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type darwinProvider struct {
	mu     sync.Mutex
	cached map[Key]ProcInfo
	last   time.Time
}

func newPlatformProvider() Provider { return &darwinProvider{} }

// Snapshot, `lsof -i -n -P -F pcnt` ciktisini ayrıştırır. Root çalışmıyorsa
// yalnızca kendi süreçleri görünür (agent root olarak çalışır).
func (p *darwinProvider) Snapshot() map[Key]ProcInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.last) < 5*time.Second && p.cached != nil {
		return p.cached
	}
	out := map[Key]ProcInfo{}

	cmd := exec.Command("lsof", "-i", "-n", "-P", "-w", "-F", "pcnt")
	outBytes, err := cmd.Output()
	if err != nil {
		// lsof bazen erisilemeyen fd'ler icin nonzero doner; cikti yine kullanilir
		if len(outBytes) == 0 {
			p.cached = out
			p.last = time.Now()
			return out
		}
	}

	var pid int32
	var proc, typ, name string
	flush := func() {
		if pid > 0 && typ != "" && name != "" && strings.Contains(name, "->") {
			parts := strings.SplitN(name, "->", 2)
			local, remote := parts[0], parts[1]
			proto := strings.ToLower(typ) // TCP/UDP -> tcp/udp
			lp, lIP := splitAddr(local)
			rp, rIP := splitAddr(remote)
			if lp == 0 || rp == 0 || rIP == "" {
				return
			}
			a := Key{Proto: proto, LocalIP: lIP, LocalPort: lp, RemoteIP: rIP, RemotePort: rp}
			b := Key{Proto: proto, LocalIP: rIP, LocalPort: rp, RemoteIP: lIP, RemotePort: lp}
			info := ProcInfo{PID: pid, Process: proc}
			out[a] = info
			out[b] = info
		}
		typ, name = "", ""
	}

	for _, line := range strings.Split(string(outBytes), "\n") {
		if len(line) < 2 {
			continue
		}
		code, val := line[0], strings.TrimSpace(line[1:])
		switch code {
		case 'p':
			flush()
			n, _ := strconv.Atoi(val)
			pid = int32(n)
			proc = ""
		case 'c':
			proc = val
		case 't':
			typ = val
		case 'n':
			name = val
			flush()
		}
	}
	flush()

	p.cached = out
	p.last = time.Now()
	return out
}

// splitAddr, "[::1]:443" / "1.2.3.4:443" / "[fe80::1%en0]:443" formatlarini cozer.
func splitAddr(s string) (uint16, string) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return 0, ""
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return 0, ""
	}
	ip := strings.Trim(s[:i], "[]")
	ip = strings.TrimSuffix(ip, "%en0")
	if j := strings.Index(ip, "%"); j >= 0 {
		ip = ip[:j]
	}
	return uint16(port), ip
}
