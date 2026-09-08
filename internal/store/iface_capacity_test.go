package store

import (
	"testing"
	"time"
)

func TestClassifyIfType(t *testing.T) {
	cases := []struct {
		t           int
		name, alias string
		want        string
	}{
		{6, "Gi0/1", "", "ethernet"},
		{117, "TenGigE0/0/0/1", "", "ethernet"},
		{71, "wlan0", "", "wifi"},
		{24, "lo0", "", "loopback"},
		{131, "Tunnel1", "", "tunnel"},
		{131, "tun0", "IPsec-VPN-HQ", "vpn"},
		{131, "ssl.root", "", "vpn"},
		{23, "ppp0", "", "ppp"},
		{209, "br-lan", "", "bridge"},
		{135, "Vlan10", "", "vlan"},
		{53, "vlan.20", "", "vlan"},
		{999, "weird0", "", "unknown"},
		{0, "", "", "unknown"},
	}
	for _, c := range cases {
		if got := classifyIfType(c.t, c.name, c.alias); got != c.want {
			t.Errorf("classifyIfType(%d,%q,%q) = %q, beklenen %q", c.t, c.name, c.alias, got, c.want)
		}
	}
}

func TestIfaceUtil(t *testing.T) {
	// 1 Gbps arayüz, 500 Mbit/s rx (62.5 MB/s), 100 Mbit/s tx
	rx, tx := ifaceUtil(62_500_000, 12_500_000, 1_000_000_000, 1)
	if rx < 49.9 || rx > 50.1 || tx < 9.9 || tx > 10.1 {
		t.Fatalf("util hatalı: rx=%.2f tx=%.2f", rx, tx)
	}
	// hız bilinmiyor → -1
	if rx, _ := ifaceUtil(1000, 1000, 0, 1); rx != -1 {
		t.Fatalf("hız 0 → -1 beklenirdi: %.2f", rx)
	}
	// oper down → -1
	if rx, _ := ifaceUtil(1000, 1000, 1_000_000_000, 2); rx != -1 {
		t.Fatalf("oper=2 → -1 beklenirdi: %.2f", rx)
	}
}

func TestSpeedBpsSource(t *testing.T) {
	// ifHighSpeed önceliklidir (>4 Gbps'te ifSpeed taşar)
	i := DeviceIface{Speed: 4_294_967_295, HighSpeed: 10_000} // 10 Gbps
	if i.SpeedBps() != 10_000_000_000 || i.SpeedSource() != "ifHighSpeed" {
		t.Fatalf("highSpeed: %d %s", i.SpeedBps(), i.SpeedSource())
	}
	i2 := DeviceIface{Speed: 1_000_000_000}
	if i2.SpeedBps() != 1_000_000_000 || i2.SpeedSource() != "ifSpeed" {
		t.Fatalf("ifSpeed: %d %s", i2.SpeedBps(), i2.SpeedSource())
	}
	i3 := DeviceIface{}
	if i3.SpeedBps() != 0 || i3.SpeedSource() != "" {
		t.Fatalf("bilinmeyen: %d %q", i3.SpeedBps(), i3.SpeedSource())
	}
}

func TestLatestDeviceIfacesCapacity(t *testing.T) {
	st := openTest(t)
	id, err := st.AddDevice(Device{Name: "sw", Host: "10.0.0.9", Kind: "switch", Vendor: "snmp"})
	if err != nil {
		t.Fatalf("dev: %v", err)
	}
	now := time.Now().Unix()
	// 1 Gbps ethernet, 30 sn'de 3.75 GB rx → 125 MB/s = 1 Gbit/s = %100 util
	if err := st.SaveDeviceIfaceSamples(id, now, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6, RxBytes: 0, TxBytes: 0},
		{IfIndex: 2, Name: "lo0", Speed: 0, OperStatus: 1, IfType: 24, RxBytes: 0, TxBytes: 0},
	}); err != nil {
		t.Fatalf("s1: %v", err)
	}
	if err := st.SaveDeviceIfaceSamples(id, now+30, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6, RxBytes: 3_750_000_000, TxBytes: 0},
		{IfIndex: 2, Name: "lo0", Speed: 0, OperStatus: 1, IfType: 24, RxBytes: 999, TxBytes: 999},
	}); err != nil {
		t.Fatalf("s2: %v", err)
	}
	rates, err := st.LatestDeviceIfaces(id)
	if err != nil {
		t.Fatalf("rates: %v", err)
	}
	var gi, lo *DeviceIfaceRate
	for i := range rates {
		switch rates[i].IfIndex {
		case 1:
			gi = &rates[i]
		case 2:
			lo = &rates[i]
		}
	}
	if gi == nil || gi.Class != "ethernet" || gi.RxUtilPct < 99 || gi.RxUtilPct > 101 {
		t.Fatalf("Gi0/1 util/class hatalı: %+v", gi)
	}
	if gi.SpeedSrc != "ifSpeed" || gi.SpeedBitsPS != 1_000_000_000 {
		t.Fatalf("Gi0/1 speed source hatalı: %+v", gi)
	}
	// loopback: hız 0 → util -1 (hesaplanamaz), class loopback
	if lo == nil || lo.Class != "loopback" || lo.RxUtilPct != -1 {
		t.Fatalf("lo0 hatalı: %+v", lo)
	}
}

// TestLatestDeviceIfacesCounterReset, sayaç geriye giderse (SNMP restart /
// wrap) verim ve kullanım 0 kalmalı — sahte iface_util uyarısı üretilmemeli.
func TestLatestDeviceIfacesCounterReset(t *testing.T) {
	st := openTest(t)
	id, _ := st.AddDevice(Device{Name: "sw", Host: "10.0.0.7", Kind: "switch", Vendor: "snmp"})
	now := time.Now().Unix()
	_ = st.SaveDeviceIfaceSamples(id, now, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6, RxBytes: 9_000_000_000, TxBytes: 9_000_000_000},
	})
	_ = st.SaveDeviceIfaceSamples(id, now+30, []DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6, RxBytes: 12_000, TxBytes: 5_000}, // reset
	})
	rates, _ := st.LatestDeviceIfaces(id)
	if len(rates) != 1 || rates[0].RxBps != 0 || rates[0].TxBps != 0 {
		t.Fatalf("sayaç reset → 0 verim beklenirdi: %+v", rates)
	}
	if rates[0].RxUtilPct != 0 && rates[0].RxUtilPct != -1 {
		t.Fatalf("sayaç reset → util sıçraması olmamalı: %v", rates[0].RxUtilPct)
	}
}
