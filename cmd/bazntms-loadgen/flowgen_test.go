package main

import (
	"math/rand"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/flows"
)

// sabit kayıtlar — çözülen değerleri kontrol edebilmek için.
func fixtureRecs() []flowRec {
	recs := make([]flowRec, recsPerDatagram)
	for i := range recs {
		recs[i] = flowRec{
			src:   [4]byte{10, 1, 2, byte(i + 1)},
			dst:   [4]byte{8, 8, 8, 8},
			sport: uint16(40000 + i),
			dport: 443,
			proto: 6,
			// v5 octet alanı uint32; büyük ama makul
			packets: uint32(10 + i),
			bytes:   uint32(1500 * (10 + i)),
		}
	}
	return recs
}

func TestBuildV5RoundTrip(t *testing.T) {
	now := time.Now()
	recs := fixtureRecs()
	rows := flows.ParseV5(buildV5(recs, 3, now), "dev-a", now)
	if len(rows) != len(recs) {
		t.Fatalf("v5: %d satır çözüldü, beklenen %d", len(rows), len(recs))
	}
	if rows[0].Src != "10.1.2.1" || rows[0].Dst != "8.8.8.8" {
		t.Errorf("v5 endpoint: %s → %s", rows[0].Src, rows[0].Dst)
	}
	if rows[0].DstPort != 443 || rows[0].Proto != "tcp" {
		t.Errorf("v5 port/proto: %d/%s", rows[0].DstPort, rows[0].Proto)
	}
	if rows[0].Packets != 10 || rows[0].Octets != 15000 {
		t.Errorf("v5 sayaç: pkts=%d octets=%d", rows[0].Packets, rows[0].Octets)
	}
}

func TestBuildV9RoundTrip(t *testing.T) {
	now := time.Now()
	recs := fixtureRecs()
	cache := flows.NewTemplateCache()

	// şablon + veri aynı datagram'da
	rows := flows.ParseV9(cache, buildV9(recs, 2, true, now), "dev-b", "127.0.0.1", now)
	if len(rows) != len(recs) {
		t.Fatalf("v9 (şablonlu): %d satır, beklenen %d", len(rows), len(recs))
	}
	if rows[5].Src != "10.1.2.6" || rows[5].DstPort != 443 || rows[5].Proto != "tcp" {
		t.Errorf("v9 kayıt: %+v", rows[5])
	}
	if rows[5].Octets != uint64(1500*15) || rows[5].Packets != 15 {
		t.Errorf("v9 sayaç: pkts=%d octets=%d", rows[5].Packets, rows[5].Octets)
	}

	// yalnız-veri datagram'ı — şablon önbellekte olduğu için çözülmeli
	rows2 := flows.ParseV9(cache, buildV9(recs, 2, false, now), "dev-b", "127.0.0.1", now)
	if len(rows2) != len(recs) {
		t.Fatalf("v9 (yalnız veri): %d satır, beklenen %d", len(rows2), len(recs))
	}

	// farklı exporter (sourceID) → ayrı şablon anahtarı; önbellekte yokken düşer
	empty := flows.ParseV9(cache, buildV9(recs, 7, false, now), "dev-c", "127.0.0.1", now)
	if len(empty) != 0 {
		t.Fatalf("v9: bilinmeyen exporter şablonu 0 satır vermeliydi, %d geldi", len(empty))
	}
}

func TestBuildIPFIXRoundTrip(t *testing.T) {
	now := time.Now()
	recs := fixtureRecs()
	cache := flows.NewTemplateCache()

	rows := flows.ParseIPFIX(cache, buildIPFIX(recs, 1, true, now), "dev-d", "127.0.0.1", now)
	if len(rows) != len(recs) {
		t.Fatalf("ipfix (şablonlu): %d satır, beklenen %d", len(rows), len(recs))
	}
	if rows[0].Src != "10.1.2.1" || rows[0].Dst != "8.8.8.8" || rows[0].Proto != "tcp" {
		t.Errorf("ipfix kayıt: %+v", rows[0])
	}

	rows2 := flows.ParseIPFIX(cache, buildIPFIX(recs, 1, false, now), "dev-d", "127.0.0.1", now)
	if len(rows2) != len(recs) {
		t.Fatalf("ipfix (yalnız veri): %d satır, beklenen %d", len(rows2), len(recs))
	}
}

func TestBuildSFlowRoundTrip(t *testing.T) {
	now := time.Now()
	rng := rand.New(rand.NewSource(1))
	recs := fixtureRecs()
	rows := flows.ParseSFlow(buildSFlow(recs, 4, rng), "dev-e", now)
	if len(rows) != len(recs) {
		t.Fatalf("sflow: %d satır, beklenen %d", len(rows), len(recs))
	}
	r := rows[0]
	if r.Src != "10.1.2.1" || r.Dst != "8.8.8.8" || r.DstPort != 443 || r.Proto != "tcp" {
		t.Errorf("sflow kayıt: %+v", r)
	}
	if r.Packets == 0 || r.Octets == 0 {
		t.Errorf("sflow ölçekli sayaç sıfır: pkts=%d octets=%d", r.Packets, r.Octets)
	}
}

func TestBuildDatagramMix(t *testing.T) {
	now := time.Now()
	rng := rand.New(rand.NewSource(2))
	for _, p := range []string{"v5", "v9", "ipfix", "sflow"} {
		if dg := buildDatagram(p, 0, true, now, rng); len(dg) == 0 {
			t.Errorf("%s: boş datagram", p)
		}
	}
}
