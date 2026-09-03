package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/vault"
)

func TestDevicesAndPoll(t *testing.T) {
	dir := t.TempDir()
	v, err := vault.Open(dir + "/vault.key")
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	st := openTest(t)

	enc, _ := v.Encrypt("gizli-comm")
	id, err := st.AddDevice(Device{
		Name: "core-sw", Host: "10.0.0.2", Kind: "switch",
		SNMPVersion: 2, Community: enc, PollSeconds: 60, Enabled: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	dev, err := st.DeviceByID(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got, err := v.Decrypt(dev.Community)
	if err != nil || got != "gizli-comm" {
		t.Fatalf("kimlik kasa donusumu: %v %q", err, got)
	}

	if err := st.UpdateDevicePoll(id, "sw-01.core", "Cisco IOS", ""); err != nil {
		t.Fatalf("poll guncelleme: %v", err)
	}

	now := time.Now().Unix()
	ifaces := []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1e9, OperStatus: 1, RxBytes: 1000, TxBytes: 500},
		{IfIndex: 2, Name: "Gi0/2", Speed: 1e9, OperStatus: 2, RxBytes: 2000, TxBytes: 900},
	}
	if err := st.SaveDeviceIfaceSamples(id, now, ifaces); err != nil {
		t.Fatalf("ornek kaydi: %v", err)
	}
	if err := st.SaveDeviceIfaceSamples(id, now+30, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1e9, OperStatus: 1, RxBytes: 7000, TxBytes: 2900},
	}); err != nil {
		t.Fatalf("ornek kaydi 2: %v", err)
	}

	rates, err := st.LatestDeviceIfaces(id)
	if err != nil {
		t.Fatalf("rates: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("2 arayuz beklenirdi: %d", len(rates))
	}
	var gi01 *DeviceIfaceRate
	for i := range rates {
		if rates[i].Name == "Gi0/1" {
			gi01 = &rates[i]
		}
	}
	if gi01 == nil {
		t.Fatalf("Gi0/1 bulunamadi: %+v", rates)
	}
	if gi01.RxBps != 200 || gi01.TxBps != 80 {
		t.Fatalf("Gi0/1 verimi hatali: %+v", *gi01)
	}

	if err := st.DeleteDevice(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if rates2, _ := st.LatestDeviceIfaces(id); len(rates2) != 0 {
		t.Fatal("silinen cihazin ornekleri kalmamaliydi")
	}
}

func TestFlowsAndSyslog(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	if err := st.SaveFlows([]FlowRow{
		{Ts: now, Device: "rt-01", Src: "1.2.3.4", Dst: "8.8.8.8", SrcPort: 51000, DstPort: 443, Proto: "tcp", Packets: 10, Octets: 5000},
		{Ts: now, Device: "rt-01", Src: "8.8.8.8", Dst: "1.2.3.4", SrcPort: 53, DstPort: 51001, Proto: "udp", Packets: 2, Octets: 200},
	}); err != nil {
		t.Fatalf("flow kaydi: %v", err)
	}
	flows, err := st.TopFlows(time.Now().Add(-time.Hour), 20, "")
	if err != nil || len(flows) != 2 {
		t.Fatalf("akislar: %v %d", err, len(flows))
	}
	if flows[0].Octets != 5000 {
		t.Fatalf("siralama hatali: %+v", flows[0])
	}

	if err := st.SaveSyslogEvent(SyslogEvent{Ts: now, Host: "rt-01", Severity: 4, Tag: "kernel", Message: "link down"}); err != nil {
		t.Fatalf("syslog kaydi: %v", err)
	}
	events, err := st.RecentSyslog(10, "")
	if err != nil || len(events) != 1 || events[0].Severity != 4 {
		t.Fatalf("syslog: %v %+v", err, events)
	}
}
