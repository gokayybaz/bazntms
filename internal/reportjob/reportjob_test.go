package reportjob

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestGenerateAndArchive(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "rj.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	dir := filepath.Join(t.TempDir(), "reports")

	// biraz filo verisi → rapor "boş" olmasın
	aid, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})
	now := time.Now()
	var b uint64 = 1_000_000
	for i := 0; i < 30; i++ {
		st.SaveIfaceSamples(aid, now.Add(time.Duration(-i)*time.Minute).Unix(), []telemetry.InterfaceSample{{Name: "eth0", RxBytes: b}})
		b += 500_000
	}

	var mailedTo []string
	mail := func(to []string, subj string, html []byte) error { mailedTo = to; return nil }

	a, err := Generate(st, nil, dir, 7, Payload{Type: "enterprise", Days: 30, Email: []string{"ops@x.com"}}, mail)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if a.ID == 0 || a.Format != "html" || a.Size == 0 {
		t.Fatalf("arşiv kaydı: %+v", a)
	}
	// dosya diske yazıldı mı, arşiv yolu doğru mu
	arch, _ := st.ReportArchiveByID(a.ID)
	if !strings.HasPrefix(arch.Path, dir) {
		t.Fatalf("yol reportsDir altında olmalı: %s", arch.Path)
	}
	if fi, err := os.Stat(arch.Path); err != nil || fi.Size() == 0 {
		t.Fatalf("dosya yok / boş: %v", err)
	}
	if len(mailedTo) != 1 || mailedTo[0] != "ops@x.com" {
		t.Fatalf("e-posta gönderilmedi: %v", mailedTo)
	}

	// liste
	list, _ := st.ListReportArchive("", 10)
	if len(list) != 1 {
		t.Fatalf("arşiv listesi: %d", len(list))
	}

	// handler yolu (scheduler.Handler) — payload JSON'dan
	h := Handler(st, nil, dir, mail)
	if err := h(context.Background(), `{"type":"traffic","days":1}`); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if l, _ := st.ListReportArchive("", 10); len(l) != 2 {
		t.Fatalf("handler arşiv eklemedi: %d", len(l))
	}
}
