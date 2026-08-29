// Package syslogd, RFC3164 UDP syslog alıcısıdır (minimal parse + test edilebilir).
package syslogd

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Event struct {
	Host     string
	Severity int
	Tag      string
	Message  string
}

type Listener struct {
	Conn    *net.UDPConn
	OnEvent func(srcIP string, ev Event)
}

// Listen, UDP dinleyicisini baslatir.
func (l *Listener) Listen(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	l.Conn = conn
	go func() {
		buf := make([]byte, 4096)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			src := ""
			if peer != nil {
				src = peer.IP.String()
			}
			ev := ParseRFC3164(string(buf[:n]), src, time.Now())
			if l.OnEvent != nil {
				l.OnEvent(src, ev)
			}
		}
	}()
	return nil
}

func (l *Listener) Close() {
	if l.Conn != nil {
		l.Conn.Close()
	}
}

var rfc3164Re = regexp.MustCompile(`^<(\d{1,3})>([A-Z][a-z]{2}\s+\d{1,2}\s\d{2}:\d{2}:\d{2})\s(\S+)\s([^:\[\s]+)(?:\[(\d+)\])?:?\s*(.*)$`)

// ParseRFC3164, "<PRI>Mon dd hh:mm:ss host tag[pid]: msg" formatini cozer.
func ParseRFC3164(line string, srcIP string, now time.Time) Event {
	ev := Event{Severity: 7, Host: srcIP, Message: strings.TrimSpace(line)}
	m := rfc3164Re.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return ev
	}
	pri, err := strconv.Atoi(m[1])
	if err == nil {
		ev.Severity = pri % 8
	}
	ev.Host = m[3]
	ev.Tag = m[4]
	ev.Message = strings.TrimSpace(m[6])
	return ev
}
