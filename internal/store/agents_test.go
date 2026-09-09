package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestListAgentsRateCounterReset, arayuz sayaci geriledigi (arayuz/agent
// resetlendi) durumda ListAgents'in uint64 alt tasmasiyla dev bir sayi
// uretmedigini, oran yerine 0 dondurdugunu dogrular.
func TestListAgentsRateCounterReset(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "laptop-01", Site: "ofis", TokenHash: TokenHash("t")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := st.TouchAgent(id, "v1", 1, "10.0.0.5"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// onceki ornek: yuksek kumulatif sayac (ornegin uzun sure calismis wifi arayuzu)
	if err := st.SaveIfaceSamples(id, now-30, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 5_000_000, TxBytes: 2_000_000, RxPackets: 5000, TxPackets: 2000},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	// sonraki ornek: arayuz resetlendi (uyku/uyanma, surucu yeniden yuklendi)
	// — sayac kucuk bir degere dustu
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
	}); err != nil {
		t.Fatalf("ornek2: %v", err)
	}

	agents, err := st.ListAgents(time.Hour, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || len(agents[0].Rates) != 1 {
		t.Fatalf("beklenmedik filo: %+v", agents)
	}
	r := agents[0].Rates[0]
	// duzeltmeden once: RxBps = float64(uint64(1000)-uint64(5_000_000)) → devasa
	// (yaklasik 1.8e19) bir sayi olurdu. Duzeltmeden sonra: 0 olmali.
	if r.RxBps != 0 || r.TxBps != 0 || r.Pps != 0 {
		t.Fatalf("sayac gerilemesinde oran 0 olmali, uint64 alt tasmasi supheli: %+v", r)
	}
	// kumulatif degerler yine de son ornegi yansitmali (goruntu icin)
	if r.RxBytes != 1000 || r.TxBytes != 500 {
		t.Fatalf("kumulatif degerler son ornekten gelmeli: %+v", r)
	}
}

// TestListAgentsRateNormal, normal (sayac ilerleyen) durumda oranin dogru
// hesaplandigini dogrular — regresyon testinin "saglikli yol"u.
func TestListAgentsRateNormal(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "sunucu-01", Site: "dc1", TokenHash: TokenHash("t2")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := st.TouchAgent(id, "v1", 1, "10.0.0.6"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	if err := st.SaveIfaceSamples(id, now-10, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 11000, TxBytes: 5500, RxPackets: 110, TxPackets: 55},
	}); err != nil {
		t.Fatalf("ornek2: %v", err)
	}

	agents, err := st.ListAgents(time.Hour, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || len(agents[0].Rates) != 1 {
		t.Fatalf("beklenmedik filo: %+v", agents)
	}
	r := agents[0].Rates[0]
	// (11000-1000)/10sn = 1000 bayt/sn
	if r.RxBps != 1000 {
		t.Fatalf("rx_bps beklenen 1000, gelen %v", r.RxBps)
	}
	if r.TxBps != 500 {
		t.Fatalf("tx_bps beklenen 500, gelen %v", r.TxBps)
	}
	// (100+50)/10sn = 15 pps
	if r.Pps != 15 {
		t.Fatalf("pps beklenen 15, gelen %v", r.Pps)
	}
}

// TestRegisterOrReuseAgent, C3: state dosyasi kaybinda ayni makinenin yeniden
// enroll'unun YENI satir degil, mevcut CEVRIMDISI satiri guncellemesini dogrular.
func TestRegisterOrReuseAgent(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()
	sq := st.(*sqlStore)
	eskit := func(id, secondsAgo int64) {
		t.Helper()
		if _, err := sq.db.Exec(sq.q(`UPDATE agents SET last_seen = ? WHERE id = ?`), now-secondsAgo, id); err != nil {
			t.Fatalf("eskit: %v", err)
		}
	}

	// çevrimiçi eşleşme → yeni satır
	id1, reused, err := st.RegisterOrReuseAgent(Agent{
		Name: "laptop", Site: "ofis", TokenHash: TokenHash("tok1"), MachineID: "mid-A",
	}, now-120)
	if err != nil || reused {
		t.Fatalf("ilk enroll: id=%d reused=%v err=%v", id1, reused, err)
	}
	id2, reused, err := st.RegisterOrReuseAgent(Agent{
		Name: "laptop", Site: "ofis", TokenHash: TokenHash("tok2"), MachineID: "mid-A",
	}, now-120)
	if err != nil || reused || id2 == id1 {
		t.Fatalf("çevrimiçi eşleşmede yeni satır bekleniyordu: id=%d reused=%v", id2, reused)
	}

	// her ikisini de eskit (id2 daha taze) → yeniden enroll en taze eşleşeni (id2) güncellemeli
	eskit(id1, 7200)
	eskit(id2, 3600)
	id3, reused, err := st.RegisterOrReuseAgent(Agent{
		Name: "laptop-yeni-ad", Site: "ofis", TokenHash: TokenHash("tok3"),
		Version: "0.2.0", ProtocolVersion: 1, RemoteIP: "10.0.0.9", MachineID: "mid-A",
	}, now-120)
	if err != nil {
		t.Fatalf("yeniden enroll: %v", err)
	}
	if !reused || id3 != id2 {
		t.Fatalf("çevrimdışı kayıt yeniden kullanılmalıydı: id3=%d id2=%d reused=%v", id3, id2, reused)
	}
	a, err := st.AgentByTokenHash(TokenHash("tok3"))
	if err != nil {
		t.Fatalf("yeni token ile bulunamadı: %v", err)
	}
	if a.ID != id2 || a.Name != "laptop-yeni-ad" || a.Version != "0.2.0" || a.RemoteIP != "10.0.0.9" {
		t.Fatalf("satır güncellenmedi: %+v", a)
	}
	if _, err := st.AgentByTokenHash(TokenHash("tok2")); err == nil {
		t.Fatal("eski token hâlâ çalışıyor")
	}

	// farklı site → aynı machine_id yeniden kullanılmaz
	eskit(id3, 3600)
	id4, reused, _ := st.RegisterOrReuseAgent(Agent{
		Name: "laptop", Site: "dc1", TokenHash: TokenHash("tok4"), MachineID: "mid-A",
	}, now-120)
	if reused || id4 == id3 {
		t.Fatalf("farklı site: yeni satır bekleniyordu (id4=%d)", id4)
	}

	// machine_id boş → her zaman yeni satır
	idX, r1, _ := st.RegisterOrReuseAgent(Agent{Name: "x", TokenHash: TokenHash("x"), Site: "ofis"}, now)
	idY, r2, _ := st.RegisterOrReuseAgent(Agent{Name: "x", TokenHash: TokenHash("y"), Site: "ofis"}, now)
	if r1 || r2 || idX == idY {
		t.Fatalf("machine_id boş: ayrı satırlar bekleniyordu (idX=%d idY=%d)", idX, idY)
	}
}

// TestSetAgentAttrInfo, süreç-atıf teşhis bilgisinin (yöntem + yakalama arayüzü
// + kapalı neden) yazıldığını, "off"un geçerli olduğunu ve boş yöntemin mevcut
// değeri KORUDUĞUNU (alan taşımayan eski agent) doğrular.
func TestSetAgentAttrInfo(t *testing.T) {
	st := openTest(t)
	id, err := st.RegisterAgent(Agent{Name: "a", TokenHash: TokenHash("t")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	// yeni agent → boş
	if a, _ := st.AgentByID(id); a.AttrMethod != "" || a.AttrIface != "" || a.AttrNote != "" {
		t.Fatalf("başlangıçta boş beklenirdi: %+v", a)
	}
	// bildir → yazılır, hem AgentByID hem ListAgents okur
	if err := st.SetAgentAttrInfo(id, "pcap", "Ethernet", ""); err != nil {
		t.Fatalf("set: %v", err)
	}
	if a, _ := st.AgentByID(id); a.AttrMethod != "pcap" || a.AttrIface != "Ethernet" {
		t.Fatalf("AgentByID: %+v", a)
	}
	agents, _ := st.ListAgents(time.Minute, "")
	if len(agents) != 1 || agents[0].AttrMethod != "pcap" || agents[0].AttrIface != "Ethernet" {
		t.Fatalf("ListAgents: %+v", agents)
	}
	// motor kapandı → "off" + neden; iface temizlenir
	if err := st.SetAgentAttrInfo(id, "off", "", "hub PCAP politikasi kapali (-agent-pcap=false)"); err != nil {
		t.Fatalf("set off: %v", err)
	}
	if a, _ := st.AgentByID(id); a.AttrMethod != "off" || a.AttrIface != "" || a.AttrNote == "" {
		t.Fatalf("off + neden yazılmalıydı: %+v", a)
	}
	// boş yöntem → yöntemi değiştirme (eski agent); iface/note yine de güncellenir
	if err := st.SetAgentAttrInfo(id, "", "", ""); err != nil {
		t.Fatalf("set empty: %v", err)
	}
	if a, _ := st.AgentByID(id); a.AttrMethod != "off" {
		t.Fatalf("boş yöntem mevcut değeri korumalıydı: %q", a.AttrMethod)
	}
}

func TestTouchAgentVersionGuard(t *testing.T) {
	st := openTest(t)
	id, err := st.RegisterAgent(Agent{Name: "a", TokenHash: TokenHash("t"), Version: "0.1.0", ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := st.TouchAgent(id, "0.2.0", 1, "10.0.0.1"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if a, _ := st.AgentByTokenHash(TokenHash("t")); a.Version != "0.2.0" {
		t.Fatalf("dolu surum yazilmaliydi, gelen: %q", a.Version)
	}

	// bos surum + 0 protokol → degistirme
	if err := st.TouchAgent(id, "", 0, "10.0.0.2"); err != nil {
		t.Fatalf("touch2: %v", err)
	}
	a, _ := st.AgentByTokenHash(TokenHash("t"))
	if a.Version != "0.2.0" || a.ProtocolVersion != 1 {
		t.Fatalf("bos degerler mevcut surumu korumaliydi: %q pv=%d", a.Version, a.ProtocolVersion)
	}
	if a.RemoteIP != "10.0.0.2" {
		t.Fatalf("remote_ip yine de guncellenmeliydi, gelen: %q", a.RemoteIP)
	}
}

// TestDeleteAgentCascade, agent silinince ona bagli TUM zaman-serisi
// tablolarinin (iface ornekleri + surec trafigi + L7 + DNS) temizlendigini,
// diger agent'in verisine dokunulmadigini dogrular. Regresyon: eskiden
// DeleteAgent yalnizca agents/agent_iface_samples/agent_conn_latest siliyordu,
// process_traffic/l7_endpoints/agent_dns oksuz kaliyordu.
func TestDeleteAgentCascade(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	victim, err := st.RegisterAgent(Agent{Name: "kurban", Site: "ofis", TokenHash: TokenHash("v")})
	if err != nil {
		t.Fatalf("register kurban: %v", err)
	}
	keep, err := st.RegisterAgent(Agent{Name: "kalan", Site: "ofis", TokenHash: TokenHash("k")})
	if err != nil {
		t.Fatalf("register kalan: %v", err)
	}

	for _, id := range []int64{victim, keep} {
		name := "kurban"
		if id == keep {
			name = "kalan"
		}
		if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
		}); err != nil {
			t.Fatalf("iface ornek (agent %d): %v", id, err)
		}
		if err := st.SaveAgentSubnets(id, name, []string{"10.0.0.0/24"}); err != nil {
			t.Fatalf("subnet (agent %d): %v", id, err)
		}
		if err := st.MarkAlertSeen("agent-proc:"+name, "sshd"); err != nil {
			t.Fatalf("alert_seen (agent %d): %v", id, err)
		}
		if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
			{PID: 42, Process: "curl", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 900, BytesOut: 100},
		}); err != nil {
			t.Fatalf("process_traffic (agent %d): %v", id, err)
		}
		if err := st.SaveL7(id, now, []telemetry.L7Sample{
			{PID: 42, Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.1.1.1", Bytes: 500, Count: 3},
		}); err != nil {
			t.Fatalf("l7 (agent %d): %v", id, err)
		}
		if err := st.SaveAgentDNS(id, now, []telemetry.DNSSample{
			{PID: 42, Process: "curl", Domain: "example.com", Queries: 2, Responses: 2},
		}); err != nil {
			t.Fatalf("agent_dns (agent %d): %v", id, err)
		}
	}

	if err := st.DeleteAgent(victim); err != nil {
		t.Fatalf("delete: %v", err)
	}

	since := time.Unix(now-60, 0)
	// kurban: her tabloda 0 satir
	if pt, _ := st.TopProcessTraffic(since, victim, 10, ""); len(pt) != 0 {
		t.Errorf("silinen agent'in process_traffic satirlari kaldi: %+v", pt)
	}
	if l7, _ := st.TopL7(since, victim, 10, ""); len(l7) != 0 {
		t.Errorf("silinen agent'in l7_endpoints satirlari kaldi: %+v", l7)
	}
	if dns, _ := st.TopAgentDNS(since, victim, 10, ""); len(dns) != 0 {
		t.Errorf("silinen agent'in agent_dns satirlari kaldi: %+v", dns)
	}
	if ag, _ := st.ListAgents(time.Hour, ""); len(ag) != 1 || ag[0].ID != keep {
		t.Errorf("yalnizca 'kalan' agent durmali: %+v", ag)
	}
	// kalan agent: verisi bozulmadi
	if pt, _ := st.TopProcessTraffic(since, keep, 10, ""); len(pt) != 1 {
		t.Errorf("kalan agent'in process_traffic'i silinmis: %+v", pt)
	}
	if l7, _ := st.TopL7(since, keep, 10, ""); len(l7) != 1 {
		t.Errorf("kalan agent'in l7'si silinmis: %+v", l7)
	}
	if dns, _ := st.TopAgentDNS(since, keep, 10, ""); len(dns) != 1 {
		t.Errorf("kalan agent'in dns'i silinmis: %+v", dns)
	}

	// S13.7: topology_links (subnet) + alert_seen (agent-proc:<ad>) da cascade
	sq := st.(*sqlStore)
	var topo, seen int
	sq.db.QueryRow(`SELECT COUNT(*) FROM topology_links WHERE source_type='agent' AND source_id=?`, victim).Scan(&topo)
	if topo != 0 {
		t.Errorf("silinen agent'in topoloji kenarlari kaldi: %d", topo)
	}
	if n, _ := st.CountAlertSeen("agent-proc:kurban"); n != 0 {
		t.Errorf("silinen agent'in alert_seen anahtarlari kaldi: %d", n)
	}
	sq.db.QueryRow(`SELECT COUNT(*) FROM topology_links WHERE source_type='agent' AND source_id=?`, keep).Scan(&seen)
	if seen != 1 {
		t.Errorf("kalan agent'in topoloji kenari silinmis: %d", seen)
	}
	if n, _ := st.CountAlertSeen("agent-proc:kalan"); n != 1 {
		t.Errorf("kalan agent'in alert_seen anahtari silinmis: %d", n)
	}
}

// TestAgentHistoryCounterReset, AgentHistory'nin de ayni alt tasma korumasina
// sahip oldugunu dogrular.
func TestAgentHistoryCounterReset(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "laptop-02", Site: "ofis", TokenHash: TokenHash("t3")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := st.SaveIfaceSamples(id, now-60, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 9_000_000, TxBytes: 4_000_000, RxPackets: 9000, TxPackets: 4000},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 200, TxBytes: 100, RxPackets: 2, TxPackets: 1},
	}); err != nil {
		t.Fatalf("ornek2 (reset sonrasi): %v", err)
	}

	buckets, err := st.AgentHistory(id, time.Unix(now-120, 0))
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	for _, b := range buckets {
		if b.In < 0 || b.Out < 0 || b.Pps < 0 {
			t.Fatalf("negatif deger olmamali: %+v", b)
		}
		// duzeltmeden once burada ~1.5e17 bayt/sn gibi devasa bir deger olurdu
		if b.In > 1e6 || b.Out > 1e6 {
			t.Fatalf("sayac gerilemesinde makul olmayan yuksek deger: %+v", b)
		}
	}
}

// TestAgentUplink, Canlı Akış gruplama: agent'a switch/AP ata, oku, kaldır;
// cihaz silinince referans NULL'lanır.
func TestAgentUplink(t *testing.T) {
	st := openTest(t)

	sw, err := st.AddDevice(Device{Name: "kat1-sw", Kind: "switch", PollSeconds: 60})
	if err != nil {
		t.Fatalf("add device: %v", err)
	}
	id, err := st.RegisterAgent(Agent{Name: "loadgen-0001", TokenHash: TokenHash("t")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := st.TouchAgent(id, "v1", 1, "10.0.0.9"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// başlangıçta uplink yok
	if a, _ := st.AgentByID(id); a.UplinkDeviceID != nil {
		t.Fatalf("yeni agent uplink'siz olmalı: %v", *a.UplinkDeviceID)
	}

	// ata
	if err := st.SetAgentUplink(id, &sw); err != nil {
		t.Fatalf("set uplink: %v", err)
	}
	a, err := st.AgentByID(id)
	if err != nil || a.UplinkDeviceID == nil || *a.UplinkDeviceID != sw {
		t.Fatalf("uplink atanmadı: %+v", a)
	}
	agents, _ := st.ListAgents(time.Hour, "")
	if len(agents) != 1 || agents[0].UplinkDeviceID == nil || *agents[0].UplinkDeviceID != sw {
		t.Fatalf("ListAgents uplink taşımıyor: %+v", agents)
	}

	// kaldır
	if err := st.SetAgentUplink(id, nil); err != nil {
		t.Fatalf("clear uplink: %v", err)
	}
	if a, _ := st.AgentByID(id); a.UplinkDeviceID != nil {
		t.Fatalf("uplink kaldırılmadı: %v", *a.UplinkDeviceID)
	}

	// yeniden ata, sonra cihazı sil → referans NULL
	if err := st.SetAgentUplink(id, &sw); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	if err := st.DeleteDevice(sw); err != nil {
		t.Fatalf("delete device: %v", err)
	}
	if a, _ := st.AgentByID(id); a.UplinkDeviceID != nil {
		t.Fatalf("cihaz silinince uplink NULL olmalı: %v", *a.UplinkDeviceID)
	}
}

// TestPruneSweepsOrphanConns, agent silinmiş ama agent_conn_latest satırları
// kalmışsa (eski sürüm / farklı yol) Prune'un onları süpürdüğünü doğrular.
func TestPruneSweepsOrphanConns(t *testing.T) {
	st := openTest(t)

	id, _ := st.RegisterAgent(Agent{Name: "ws-1", TokenHash: TokenHash("t")})
	if err := st.ReplaceConnLatest(id, []telemetry.ConnectionSample{
		{Proto: "tcp", LocalAddr: "10.0.0.1:5000", RemoteAddr: "1.1.1.1:443", Status: "ESTABLISHED"},
	}); err != nil {
		t.Fatalf("conn: %v", err)
	}
	// canlı agent'ın bağlantısı Prune'da korunur
	if err := st.Prune(time.Hour); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if conns := st.LatestAgentConnections(id); len(conns) != 1 {
		t.Fatalf("canlı agent bağlantısı silinmemeli: %d", len(conns))
	}

	// agent'ı elle (cascade'siz) sil → yetim satır oluştur
	if _, err := st.(*sqlStore).db.Exec(`DELETE FROM agents WHERE id = $1`, id); err != nil {
		if _, err2 := st.(*sqlStore).db.Exec(`DELETE FROM agents WHERE id = ?`, id); err2 != nil {
			t.Fatalf("elle sil: %v / %v", err, err2)
		}
	}
	if err := st.Prune(time.Hour); err != nil {
		t.Fatalf("prune2: %v", err)
	}
	if conns := st.LatestAgentConnections(id); len(conns) != 0 {
		t.Fatalf("yetim bağlantı süpürülmedi: %d", len(conns))
	}
}
