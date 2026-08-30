//go:build linux

package proctraffic

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type linuxProvider struct {
	mu     sync.Mutex
	cached map[Key]ProcInfo
	last   time.Time
}

func newPlatformProvider() Provider { return &linuxProvider{} }

func (p *linuxProvider) Snapshot() map[Key]ProcInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.last) < 3*time.Second && p.cached != nil {
		return p.cached
	}
	m := parseProcNet()
	attachPIDs(m)
	p.cached = m
	p.last = time.Now()
	return m
}

// parseProcNet, /proc/net/{tcp,tcp6,udp,udp6} tablolarini okuyup
// soket anahtarlarini uretir (her iki yonlu).
func parseProcNet() map[Key]ProcInfo {
	out := map[Key]ProcInfo{}
	files := map[string]string{
		"tcp":  "/proc/net/tcp",
		"tcp6": "/proc/net/tcp6",
		"udp":  "/proc/net/udp",
		"udp6": "/proc/net/udp6",
	}
	for proto, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 64*1024)
		first := true
		for sc.Scan() {
			if first {
				first = false
				continue
			}
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 {
				continue
			}
			localIP, localPort, err := parseHexAddr(fields[1])
			if err != nil {
				continue
			}
			remoteIP, remotePort, err := parseHexAddr(fields[2])
			if err != nil {
				continue
			}
			inode := fields[9]
			a := Key{Proto: proto, LocalIP: localIP, LocalPort: localPort, RemoteIP: remoteIP, RemotePort: remotePort}
			b := Key{Proto: proto, LocalIP: remoteIP, LocalPort: remotePort, RemoteIP: localIP, RemotePort: localPort}
			out[a] = ProcInfo{inode: inode}
			out[b] = ProcInfo{inode: inode}
		}
		f.Close()
	}
	return out
}

// parseHexAddr, /proc/net adres formatini (AABBCCDD:1F90) cozer.
func parseHexAddr(s string) (string, uint16, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, os.ErrInvalid
	}
	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, err
	}
	raw := parts[0]
	// IPv4: 4 bayt little-endian; IPv6: 16 bayt little-endian (4'lupolar halinde)
	var ip []byte
	if len(raw) == 8 {
		ip = make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := strconv.ParseUint(raw[i*2:i*2+2], 16, 8)
			if err != nil {
				return "", 0, err
			}
			ip[3-i] = byte(b)
		}
		return formatIP(ip), uint16(port), nil
	}
	if len(raw) == 32 {
		ip = make([]byte, 16)
		for i := 0; i < 16; i++ {
			b, err := strconv.ParseUint(raw[i*2:i*2+2], 16, 8)
			if err != nil {
				return "", 0, err
			}
			ip[i/4*4+(3-(i%4))] = byte(b)
		}
		return formatIP(ip), uint16(port), nil
	}
	return "", 0, os.ErrInvalid
}

func formatIP(ip []byte) string {
	return net.IP(ip).String()
}

// attachPIDs, soket inode'larini /proc/[pid]/fd uzerinden sureclere esler.
func attachPIDs(m map[Key]ProcInfo) {
	inodeToKeys := map[string][]Key{}
	for k, pi := range m {
		if pi.inode == "" {
			continue
		}
		inodeToKeys[pi.inode] = append(inodeToKeys[pi.inode], k)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // izin yok
		}
		comm := readComm(pid)
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			for _, k := range inodeToKeys[inode] {
				pi := m[k]
				pi.PID = int32(pid)
				pi.Process = comm
				pi.inode = ""
				m[k] = pi
			}
		}
	}
}

func readComm(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
