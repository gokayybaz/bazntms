package store

// Faz 8.7: FortiGate veri tablosu round-trip testleri (SQLite modu):
// kaynak, VPN upsert, SD-WAN, politika delta hesabı ve uyarı sorguları.

import (
	"testing"
	"time"
)

func TestFortigateStoreRoundTrip(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	// --- cihaz + vendor alanları ---
	id, err := st.AddDevice(Device{
		Name: "fgt-ofis", Host: "10.9.9.1", Kind: "firewall", Vendor: "fortigate",
		APIURL: "https://10.9.9.1", APIToken: "enc-token",
		APIVerifyTLS: false, VDOM: "root", Enabled: true,
	})
	if err != nil || id == 0 {
		t.Fatalf("cihaz: %v %d", err, id)
	}
	devs, _ := st.ListDevices()
	if len(devs) != 1 || devs[0].Vendor != "fortigate" || devs[0].APIURL != "https://10.9.9.1" ||
		devs[0].APIVerifyTLS || devs[0].VDOM != "root" {
		t.Fatalf("vendor alanları: %+v", devs)
	}

	// --- kaynak örnekleri ---
	for i := int64(0); i < 5; i++ {
		if err := st.SaveDeviceResources(DeviceResource{
			Ts: now - 300 + i*60, DeviceID: id,
			CPUPct: 20 + float64(i), MemPct: 55, DiskPct: 10, Sessions: 1000 + i,
		}); err != nil {
			t.Fatalf("resource: %v", err)
		}
	}
	res, err := st.LatestDeviceResources(id, 60)
	if err != nil || len(res) != 5 || res[4].Sessions != 1004 {
		t.Fatalf("resource sorgu: %v %+v", err, res)
	}

	// --- VPN upsert: aynı tünel güncellenir, yeni tünel eklenir ---
	vpn1 := []FortiVPNStatus{
		{DeviceID: id, VDOM: "root", Kind: "ipsec", Name: "hq", Peer: "1.1.1.1", Status: "up", RxBytes: 100, TxBytes: 200, Ts: now},
		{DeviceID: id, VDOM: "root", Kind: "ssl", Name: "ayse.k", Peer: "8.8.8.8", Status: "up", Ts: now},
	}
	if err := st.SaveFortiVPNStatus(id, now, vpn1); err != nil {
		t.Fatalf("vpn1: %v", err)
	}
	vpn2 := []FortiVPNStatus{
		{DeviceID: id, VDOM: "root", Kind: "ipsec", Name: "hq", Peer: "1.1.1.1", Status: "down", RxBytes: 500, TxBytes: 600, Ts: now + 60},
	}
	if err := st.SaveFortiVPNStatus(id, now+60, vpn2); err != nil {
		t.Fatalf("vpn2: %v", err)
	}
	vpnRows, err := st.LatestFortiVPN(id)
	if err != nil || len(vpnRows) != 2 {
		t.Fatalf("vpn satır sayısı: %v %d", err, len(vpnRows))
	}
	for _, v := range vpnRows {
		if v.Name == "hq" && (v.Status != "down" || v.RxBytes != 500) {
			t.Fatalf("hq upsert çalışmadı: %+v", v)
		}
	}

	// --- down tünelleri (uyarı sorgusu) ---
	downs, err := st.FortiVPNsDown(time.Hour)
	if err != nil || len(downs) != 1 || downs[0].Name != "hq" || downs[0].DeviceName != "fgt-ofis" {
		t.Fatalf("vpn down: %v %+v", err, downs)
	}

	// --- SD-WAN ---
	if err := st.SaveFortiSDWAN(id, now, []FortiSDWANSample{
		{Ts: now, DeviceID: id, VDOM: "root", Member: "wan1", HealthCheck: "sla1",
			LatencyMs: 50, JitterMs: 5, PacketLossPct: 0, State: "up"},
	}); err != nil {
		t.Fatalf("sdwan: %v", err)
	}
	sdRows, err := st.RecentFortiSDWANAll(time.Now().Add(-time.Hour))
	if err != nil || len(sdRows) != 1 || sdRows[0].Member != "wan1" || sdRows[0].LatencyMs != 50 {
		t.Fatalf("sdwan sorgu: %v %+v", err, sdRows)
	}

	// --- politika delta hesabı ---
	for i := int64(0); i < 3; i++ {
		if err := st.SaveFortiPolicyHits(id, now-120+i*60, []FortiPolicyHit{
			{Ts: now - 120 + i*60, DeviceID: id, VDOM: "root", PolicyID: 7,
				Name: "web-out", Action: "accept", Hits: uint64(100 + i*50), Bytes: uint64(1000 + i*2000)},
			{Ts: now - 120 + i*60, DeviceID: id, VDOM: "root", PolicyID: 9,
				Name: "blocked", Action: "deny", Hits: uint64(10), Bytes: uint64(100)},
		}); err != nil {
			t.Fatalf("policy: %v", err)
		}
	}
	pols, err := st.TopFortiPolicies(id, time.Now().Add(-time.Hour), 10)
	if err != nil || len(pols) != 2 {
		t.Fatalf("policy sorgu: %v %d", err, len(pols))
	}
	// web-out delta: 100 hit, 4000 bayt — en aktif üstte
	if pols[0].PolicyID != 7 || pols[0].Hits != 100 || pols[0].Bytes != 4000 {
		t.Fatalf("policy delta: %+v", pols)
	}

	// --- prune ---
	if err := st.Prune(30 * time.Second); err != nil {
		t.Fatalf("prune: %v", err)
	}
	res2, _ := st.LatestDeviceResources(id, 60)
	if len(res2) != 0 {
		t.Fatalf("prune sonrasi resource: %d", len(res2))
	}
}
