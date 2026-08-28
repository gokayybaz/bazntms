package sysmon

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type Interface struct {
	Name      string   `json:"name"`
	HwAddr    string   `json:"hw_addr,omitempty"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	CanSniff  bool     `json:"can_sniff"`
	Addresses []string `json:"addresses"`
	RxBytes   uint64   `json:"rx_bytes"`
	TxBytes   uint64   `json:"tx_bytes"`
	RxPackets uint64   `json:"rx_packets"`
	TxPackets uint64   `json:"tx_packets"`
}

func ListInterfaces() []Interface {
	stdIfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	io, _ := psnet.IOCounters(true)
	ioMap := map[string]psnet.IOCountersStat{}
	for _, c := range io {
		ioMap[c.Name] = c
	}

	out := make([]Interface, 0, len(stdIfaces))
	for _, i := range stdIfaces {
		ifi := Interface{
			Name:     i.Name,
			HwAddr:   i.HardwareAddr.String(),
			MTU:      i.MTU,
			Up:       i.Flags&net.FlagUp != 0,
			Loopback: i.Flags&net.FlagLoopback != 0,
		}
		if i.Flags&net.FlagBroadcast != 0 && !ifi.Loopback {
			ifi.CanSniff = true
		}
		addrs, err := i.Addrs()
		if err == nil {
			for _, a := range addrs {
				ifi.Addresses = append(ifi.Addresses, a.String())
			}
		}
		if c, ok := ioMap[i.Name]; ok {
			ifi.RxBytes = c.BytesRecv
			ifi.TxBytes = c.BytesSent
			ifi.RxPackets = c.PacketsRecv
			ifi.TxPackets = c.PacketsSent
		}
		out = append(out, ifi)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Up != out[b].Up {
			return out[a].Up
		}
		return out[a].Name < out[b].Name
	})
	return out
}

type Connection struct {
	Proto      string `json:"proto"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Status     string `json:"status,omitempty"`
	PID        int32  `json:"pid"`
	Process    string `json:"process,omitempty"`
	Count      uint64 `json:"count"`
	Country    string `json:"country,omitempty"`
	ASN        string `json:"asn,omitempty"`
}

var (
	procMu    sync.Mutex
	procCache = map[int32]string{}
)

func procName(pid int32) string {
	if pid <= 0 {
		return ""
	}
	procMu.Lock()
	defer procMu.Unlock()
	if n, ok := procCache[pid]; ok {
		return n
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		procCache[pid] = ""
		return ""
	}
	n, err := p.Name()
	if err != nil || n == "" {
		procCache[pid] = ""
		return ""
	}
	procCache[pid] = n
	return n
}

func ListConnections() []Connection {
	cons, err := psnet.Connections("all")
	if err != nil {
		return nil
	}
	merged := map[string]*Connection{}
	for _, c := range cons {
		if c.Laddr.Port == 0 && c.Raddr.Port == 0 {
			continue
		}
		proto := "tcp"
		if c.Type == 2 { // SOCK_DGRAM
			proto = "udp"
		}
		local := formatAddr(c.Laddr.IP, c.Laddr.Port)
		remote := formatAddr(c.Raddr.IP, c.Raddr.Port)
		if remote == "0.0.0.0:0" || remote == "[::]:0" || remote == "[::ffff:0.0.0.0]:0" {
			remote = ""
		}
		status := c.Status
		if proto == "udp" {
			status = ""
		}
		key := fmt.Sprintf("%s|%s|%s", proto, local, remote)
		m := merged[key]
		if m == nil {
			m = &Connection{
				Proto:      proto,
				LocalAddr:  local,
				RemoteAddr: remote,
				Status:     status,
				PID:        c.Pid,
				Count:      1,
			}
			merged[key] = m
		} else {
			m.Count++
			if m.PID == 0 && c.Pid != 0 {
				m.PID = c.Pid
			}
			if m.Status == "" && status != "" {
				m.Status = status
			}
		}
	}
	out := make([]Connection, 0, len(merged))
	for _, m := range merged {
		m.Process = procName(m.PID)
		out = append(out, *m)
	}
	sort.Slice(out, func(a, b int) bool {
		if (out[a].Process == "") != (out[b].Process == "") {
			return out[a].Process != ""
		}
		if out[a].Process != out[b].Process {
			return out[a].Process < out[b].Process
		}
		return out[a].LocalAddr < out[b].LocalAddr
	})
	return out
}

func formatAddr(ip string, port uint32) string {
	if ip == "" {
		ip = "0.0.0.0"
	}
	if strings.Contains(ip, ":") {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}
