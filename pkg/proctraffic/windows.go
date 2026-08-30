//go:build windows

package proctraffic

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type windowsProvider struct {
	mu     sync.Mutex
	cached map[Key]ProcInfo
	last   time.Time
	names  map[int32]string
}

func newPlatformProvider() Provider { return &windowsProvider{names: map[int32]string{}} }

// Snapshot, `netstat -ano` tablolarini surec adlariyla esler.
func (p *windowsProvider) Snapshot() map[Key]ProcInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.last) < 5*time.Second && p.cached != nil {
		return p.cached
	}
	out := map[Key]ProcInfo{}

	for _, proto := range []string{"tcp", "udp"} {
		cmd := exec.Command("netstat", "-ano", "-p", proto)
		outBytes, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(outBytes), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) < 4 || !strings.HasPrefix(strings.ToLower(fields[0]), proto) {
				continue
			}
			// Proto  Local  Foreign  [State]  PID
			pidIdx := len(fields) - 1
			pid64, err := strconv.ParseUint(fields[pidIdx], 10, 32)
			if err != nil || pid64 == 0 {
				continue
			}
			local := fields[1]
			remote := fields[2]
			if strings.HasPrefix(remote, "*:") {
				continue
			}
			lp, _ := splitAddr(local)
			rp, rIP := splitAddr(remote)
			if lp == 0 || rIP == "" {
				continue
			}
			pid := int32(pid64)
			proc := p.procName(pid)
			a := Key{Proto: proto, LocalIP: "0.0.0.0", LocalPort: lp, RemoteIP: rIP, RemotePort: rp}
			b := Key{Proto: proto, LocalIP: rIP, LocalPort: rp, RemoteIP: "0.0.0.0", RemotePort: lp}
			info := ProcInfo{PID: pid, Process: proc}
			out[a] = info
			out[b] = info
		}
	}

	p.cached = out
	p.last = time.Now()
	return out
}

func (p *windowsProvider) procName(pid int32) string {
	if n, ok := p.names[pid]; ok {
		return n
	}
	n := ""
	if pr, err := process.NewProcess(pid); err == nil {
		if v, err := pr.Name(); err == nil {
			n = v
		}
	}
	p.names[pid] = n
	if len(p.names) > 4096 {
		p.names = map[int32]string{}
	}
	return n
}

// splitAddr, netstat adresini (1.2.3.4:80 | [::]:80) port ve IP'ye ayırır.
func splitAddr(s string) (uint16, string) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return 0, ""
	}
	p64, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, ""
	}
	return uint16(p64), strings.Trim(host, "[]")
}
