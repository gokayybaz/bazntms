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

// Snapshot, `lsof -i -n -P -F pcnPf` ciktisini ayrıştırır. Root çalışmıyorsa
// yalnızca kendi süreçleri görünür (agent root olarak çalışır).
//
// Alan kodlari onemli: buyuk 'P' protokol adidir (TCP/UDP); kucuk 't' dosya
// TURUdur (IPv4/IPv6) — protokol degil. 't' kullanildiginda Key.Proto "ipv4"
// olur ve pcap tarafinin urettigi "tcp"/"udp" ile hicbir zaman eslesmez;
// macOS'ta surec atfi, L7 ve DNS gorunurlugu sessizce bos kalir.
func (p *darwinProvider) Snapshot() map[Key]ProcInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.last) < 5*time.Second && p.cached != nil {
		return p.cached
	}
	cmd := exec.Command("lsof", "-i", "-n", "-P", "-w", "-F", "pcnPf")
	outBytes, err := cmd.Output()
	if err != nil && len(outBytes) == 0 {
		// lsof bazen erisilemeyen fd'ler icin nonzero doner; cikti varsa kullanilir
		p.cached = map[Key]ProcInfo{}
		p.last = time.Now()
		return p.cached
	}

	p.cached = parseLsof(outBytes)
	p.last = time.Now()
	return p.cached
}

// parseLsof, `lsof -F pcnPf` alan ciktisini baglanti -> surec haritasina cevirir.
func parseLsof(outBytes []byte) map[Key]ProcInfo {
	out := map[Key]ProcInfo{}

	var pid int32
	var proc, proto, name string
	flush := func() {
		defer func() { proto, name = "", "" }()
		// yalnizca baglantili soketler atfedilebilir ("yerel->uzak")
		if pid <= 0 || proto == "" || !strings.Contains(name, "->") {
			return
		}
		local, remote, _ := strings.Cut(name, "->")
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

	// lsof alan sirasi: p/c (surec seti), ardindan her dosya icin f/P/n.
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
		case 'f': // yeni dosya seti — onceki dosyanin artiklarini bosalt
			flush()
		case 'P':
			proto = strings.ToLower(val) // TCP/UDP -> tcp/udp
		case 'n':
			name = val
			flush()
		}
	}
	flush()

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
