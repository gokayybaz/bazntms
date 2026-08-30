package compliance

// Faz 9.11: bütünlük motoru testleri — Merkle, zincir, checkpoint, günlük
// mühür (mock TSA + PEM imza), kurcalama tespiti ve delil paketi doğrulaması.

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gokayybaz/bazntms/internal/store"
)

func testStore(t *testing.T) (store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "comp.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st, filepath.Join(dir, "comp.db")
}

func TestMerkleRoot(t *testing.T) {
	if MerkleRoot(nil) != nil {
		t.Fatal("boş girdi nil olmalı")
	}
	a := []byte{1}
	b := []byte{2}
	r1 := MerkleRoot([][]byte{a, b})
	r2 := MerkleRoot([][]byte{b, a})
	if equalBytes(r1, r2) {
		t.Fatal("sıra duyarlılığı yok")
	}
	// tek eleman: kendisi (hash'lenmemiş tek düğüm)
	if !equalBytes(MerkleRoot([][]byte{a}), a) {
		t.Fatal("tek eleman kökü kendisi olmalı")
	}
	// teksayı katmanı: son eleman kopyalanır — determinizm
	three := MerkleRoot([][]byte{a, b, []byte{3}})
	threeAgain := MerkleRoot([][]byte{a, b, []byte{3}})
	if !equalBytes(three, threeAgain) {
		t.Fatal("merkle deterministik değil")
	}
}

// mockTSA, minimal RFC 3161 yanıt döndüren test sunucusu.
func mockTSA(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readAll(r)
		// istekteki hash'i bul (SHA-256, 32 bayt) — mock token'a gömülür
		var req asn1TimeStampReq
		asn1.Unmarshal(body, &req)
		hash := req.MessageImprint.HashedMessage

		token := asn1.RawValue{
			Class:      0,
			Tag:        16,
			IsCompound: true,
			Bytes:      append([]byte("MOCKTSA\x00"), hash...),
		}
		resp := struct {
			Status struct {
				Status int
			}
			Token asn1.RawValue `asn1:"optional"`
		}{}
		resp.Status.Status = 0
		resp.Token = token
		der, err := asn1.Marshal(resp)
		if err != nil {
			t.Errorf("mock resp: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.Write(der)
	}))
}

func readAll(r *http.Request) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}

func TestTSAAndSeal(t *testing.T) {
	st, dbPath := testStore(t)
	tsaSrv := mockTSA(t)
	defer tsaSrv.Close()

	keyFile := filepath.Join(t.TempDir(), "compliance.key")
	wormDir := t.TempDir()

	sealer, err := NewSealer(st, Config{
		Enabled: true, TSAURL: tsaSrv.URL, SignKeyFile: keyFile,
		WormDir: wormDir, CheckpointHour: true,
	})
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}

	// iki saate yayılan loglar
	now := time.Now()
	h1 := now.Truncate(time.Hour).Add(-2 * time.Hour) // önceki-önceki saat
	h2 := now.Truncate(time.Hour).Add(-1 * time.Hour) // önceki saat
	for i := 0; i < 5; i++ {
		st.AppendComplianceLog(store.ComplianceLog{
			Ts: h1.Unix() + int64(i)*60, SourceType: "syslog", SourceName: "fgt",
			SrcIP: "10.0.0.9", SrcMAC: "aa:bb:cc:dd:ee:ff", Category: "traffic",
			Message: "kullanıcı oturumu",
		})
	}
	for i := 0; i < 3; i++ {
		st.AppendComplianceLog(store.ComplianceLog{
			Ts: h2.Unix() + int64(i)*60, SourceType: "syslog", SourceName: "fgt",
			Category: "traffic", Message: "erişim",
		})
	}

	// saatlik checkpoint'ler
	if err := sealer.HourlyCheckpoint(context.Background(), h1); err != nil {
		t.Fatalf("hourly1: %v", err)
	}
	if err := sealer.HourlyCheckpoint(context.Background(), h2); err != nil {
		t.Fatalf("hourly2: %v", err)
	}
	cp1, _ := st.LatestLogCheckpoint("hourly")
	if cp1 == nil || cp1.RecordCount != 3 {
		t.Fatalf("son checkpoint: %+v", cp1)
	}

	// günlük mühür: h1'in günü
	day := time.Date(h1.Year(), h1.Month(), h1.Day(), 0, 0, 0, 0, time.Local)
	if err := sealer.DailySeal(context.Background(), day); err != nil {
		t.Fatalf("daily: %v", err)
	}
	daily, _ := st.LatestLogCheckpoint("daily")
	if daily == nil || daily.TSAStatus != "ok" || daily.Signature == "" {
		t.Fatalf("daily seal: %+v", daily)
	}
	if !TokenContainsHash(daily.TSAToken, sha256Sum(mustHex(daily.Root))) {
		t.Fatal("TSA token kök hash'ini içermiyor")
	}

	// anahtar dosyaları üretildi + pub key okundu
	pub, err := readPubKey(keyFile + ".pub")
	if err != nil {
		t.Fatalf("pub key: %v", err)
	}

	// delil paketi + offline doğrulama
	bundle, err := BuildEvidence(st, day.Unix(), day.Add(24*time.Hour).Unix(), false)
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	rep := VerifyBundle(bundle, pub)
	if !rep.OK || !rep.MerkleOK || !rep.DailyOK || !rep.SigOK || !rep.SigChecked {
		t.Fatalf("doğrulama başarısız: %+v", rep)
	}
	if rep.CheckedRecords != 8 {
		t.Fatalf("doğrulanan kayıt: %d", rep.CheckedRecords)
	}

	// WORM paketi yazıldı
	dayDir := filepath.Join(wormDir, day.Format("2006"), day.Format("01"))
	if _, err := os.Stat(filepath.Join(dayDir, "bazntms-manifest-"+day.Format("2006-01-02")+".json")); err != nil {
		t.Fatalf("worm manifest: %v", err)
	}

	// PII maskeleme
	bundle, _ = BuildEvidence(st, day.Unix(), day.Add(24*time.Hour).Unix(), true)
	if bundle.Masked != true || strings.Contains(bundle.Logs[0].SrcMAC, "aa:bb:cc:dd") && bundle.Logs[0].SrcMAC != "" {
		if !strings.HasSuffix(bundle.Logs[0].SrcMAC, "xx:xx") {
			t.Fatalf("MAC maskeleme: %s", bundle.Logs[0].SrcMAC)
		}
	}
	if strings.HasSuffix(bundle.Logs[0].SrcIP, ".9") {
		t.Fatalf("IP maskeleme: %s", bundle.Logs[0].SrcIP)
	}

	// --- kurcalama tespiti: ham veritabanında mesaj değiştirilir ---
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE compliance_logs SET message = 'kurcalandi' WHERE seq = 1`); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	tampered, _ := BuildEvidence(st, day.Unix(), day.Add(24*time.Hour).Unix(), false)
	tamperedRep := VerifyBundle(tampered, nil)
	if tamperedRep.OK || tamperedRep.BrokenSeq == 0 {
		t.Fatalf("kurcalama tespit edilemedi: %+v", tamperedRep)
	}
}

func TestTSAAcceptance(t *testing.T) {
	// reddedilen istek → hata
	reject := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Status struct {
				Status int
			}
			Token asn1.RawValue `asn1:"optional"`
		}{}
		resp.Status.Status = 2 // rejection
		der, _ := asn1.Marshal(resp)
		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.Write(der)
	}))
	defer reject.Close()
	c := NewTSAClient(reject.URL, 5*time.Second)
	if _, _, err := c.Timestamp(context.Background(), sha256Sum([]byte("x"))); err == nil {
		t.Fatal("reddedilen istek hatasız geçti")
	}

	// bozuk hash boyutu
	ok := mockTSA(t)
	defer ok.Close()
	c2 := NewTSAClient(ok.URL, 5*time.Second)
	if _, _, err := c2.Timestamp(context.Background(), []byte("kisa")); err == nil {
		t.Fatal("yanlış hash boyutu kabul edildi")
	}
}

func readPubKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, os.ErrInvalid
	}
	return ed25519.PublicKey(block.Bytes), nil
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

var _ = json.Marshal // koru
