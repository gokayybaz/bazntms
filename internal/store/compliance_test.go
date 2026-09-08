package store

import (
	"sync"
	"testing"
	"time"
)

// walkComplianceChain, [from,to) kayıtlarını sırayla yürür; ilk kopuk seq'i
// (veya 0) ve doğrulanan kayıt sayısını döndürür.
func walkComplianceChain(t *testing.T, st Store, from, to int64) (brokenSeq int64, checked int) {
	t.Helper()
	logs, err := st.(*sqlStore).ComplianceLogsBetween(from, to)
	if err != nil {
		t.Fatalf("ComplianceLogsBetween: %v", err)
	}
	prev := ""
	for _, l := range logs {
		if l.PrevHash != prev || ComplianceHash(prev, l) != l.Hash {
			return l.Seq, checked
		}
		prev = l.Hash
		checked++
	}
	return 0, checked
}

// TestComplianceChainConcurrent, çok sayıda goroutine aynı anda
// AppendComplianceLog çağırınca 5651 log zinciri tek-yönlü kalmalı: hiçbir
// prev_hash birden çok kayıtta olmamalı (çatal yok) ve zincir baştan sona
// doğrulanmalı. SQLite'ta process-içi complianceMu bunu zaten sağlar — bu
// regresyon koruması; asıl kazanç pg yolunda (TestPostgresComplianceChainConcurrent).
func TestComplianceChainConcurrent(t *testing.T) {
	st := openTest(t)
	base := time.Now().Unix()

	const workers, each = 8, 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers*each)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := st.AppendComplianceLog(ComplianceLog{
					Ts: base + int64(i), SourceType: "syslog", SourceName: "sw1",
					Category: "syslog", Message: "test olayı",
				}); err != nil {
					errCh <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("eşzamanlı ekleme: %v", err)
	}

	broken, checked := walkComplianceChain(t, st, base-10, base+3600)
	if broken != 0 {
		t.Fatalf("zincir kopuk: kayıt seq %d", broken)
	}
	if checked != workers*each {
		t.Fatalf("%d kayıt beklenirdi, doğrulanan: %d", workers*each, checked)
	}

	// çatal noktası (bir prev_hash birden çok kayıtta) olmamalı
	rows, err := st.(*sqlStore).db.Query(
		`SELECT COUNT(*) FROM compliance_logs GROUP BY prev_hash HAVING COUNT(*) > 1`)
	if err != nil {
		t.Fatalf("çatal sorgu: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("çatal noktası: bir prev_hash birden çok compliance_logs kaydında")
	}
}
