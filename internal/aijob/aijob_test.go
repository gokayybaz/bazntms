package aijob

import (
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestEnsureJobIdempotent(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "aijob.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer st.Close()

	if err := EnsureJob(st, "daily:06:00"); err != nil {
		t.Fatalf("EnsureJob: %v", err)
	}
	// ikinci çağrı yeni satır oluşturmaz
	if err := EnsureJob(st, "daily:07:00"); err != nil {
		t.Fatalf("EnsureJob 2: %v", err)
	}
	jobs, _ := st.ListScheduledJobs()
	n := 0
	for _, j := range jobs {
		if j.Kind == "ai_report" {
			n++
			if j.Spec != "daily:06:00" {
				t.Errorf("mevcut iş spec'i korunmalı: %q", j.Spec)
			}
			if j.NextRunTs == 0 {
				t.Error("next_run_ts hesaplanmalı")
			}
		}
	}
	if n != 1 {
		t.Fatalf("ai_report iş sayısı = %d, beklenen 1", n)
	}
}

func TestEnsureJobBadSpec(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "aijob2.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer st.Close()
	if err := EnsureJob(st, "her-gün-sabah"); err == nil {
		t.Fatal("geçersiz spec hata vermeli")
	}
}
