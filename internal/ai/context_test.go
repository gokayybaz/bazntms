package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSnapshotSectionsBudget(t *testing.T) {
	var snap Snapshot
	snap.Period = "son 1 saat"
	snap.Add("kucuk", map[string]int{"a": 1})
	snap.Add("buyuk", map[string]string{"blob": strings.Repeat("x", 5000)})
	snap.Add("son", map[string]int{"z": 9})

	secs := snap.Sections(2) // 2 KB butce
	if len(secs) == 0 {
		t.Fatal("hic bolum uretilmedi")
	}
	var joined strings.Builder
	for _, s := range secs {
		joined.WriteString(s.Data)
	}
	if !strings.Contains(joined.String(), "kirpildi") && !hasTitle(secs, "Atlanan bolumler") {
		t.Errorf("butce asimi kirpma/atlama isareti yok: %+v titles=%v", secs, titles(secs))
	}
	// ilk (kucuk) bolum tam gecmeli
	if secs[0].Title != "kucuk" || !strings.Contains(secs[0].Data, `"a": 1`) {
		t.Errorf("ilk bolum bozuldu: %+v", secs[0])
	}
}

func TestSnapshotSectionsNoBudgetPressure(t *testing.T) {
	var snap Snapshot
	snap.Add("a", map[string]int{"x": 1})
	snap.Add("b", map[string]int{"y": 2})
	secs := snap.Sections(64)
	if len(secs) != 2 {
		t.Fatalf("butce baski yokken 2 bolum bekleniyordu: %d", len(secs))
	}
}

// fakeAdapter, Analyze testleri icin.
type fakeAdapter struct {
	calls   int
	replies []string
	err     error
}

func (f *fakeAdapter) Kind() Kind                                   { return KindOpenAICompat }
func (f *fakeAdapter) Models(ctx context.Context) ([]string, error) { return nil, nil }
func (f *fakeAdapter) Stream(ctx context.Context, req ChatRequest) (<-chan Delta, error) {
	return nil, errors.New("stream kullanilmadi")
}
func (f *fakeAdapter) Complete(ctx context.Context, req ChatRequest) (string, Usage, error) {
	if f.err != nil {
		return "", Usage{}, f.err
	}
	r := "cevap"
	if f.calls < len(f.replies) {
		r = f.replies[f.calls]
	}
	f.calls++
	return r, Usage{PromptTokens: 10, CompletionTokens: 5}, nil
}

func TestAnalyzeSingleShot(t *testing.T) {
	var snap Snapshot
	snap.Period = "son 24 saat"
	snap.Add("ozet", map[string]int{"gb": 42})
	f := &fakeAdapter{replies: []string{"tek analiz"}}
	out, usage, err := Analyze(context.Background(), f, "m", SystemAnalyst, snap, TaskFleetSummary, false, 16)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if out != "tek analiz" || f.calls != 1 {
		t.Errorf("out=%q calls=%d", out, f.calls)
	}
	if usage.PromptTokens != 10 {
		t.Errorf("usage toplanmadi: %+v", usage)
	}
}

func TestAnalyzeChunked(t *testing.T) {
	var snap Snapshot
	snap.Period = "son 24 saat"
	snap.Add("trafik", map[string]int{"gb": 42})
	snap.Add("hedefler", []string{"1.2.3.4"})
	snap.Add("surecler", []string{"nginx"})
	f := &fakeAdapter{replies: []string{"not1", "not2", "not3", "final analiz"}}
	out, usage, err := Analyze(context.Background(), f, "m", SystemAnalyst, snap, TaskFleetSummary, true, 64)
	if err != nil {
		t.Fatalf("Analyze chunked: %v", err)
	}
	// 3 parca notu + 1 final = 4 cagri
	if f.calls != 4 {
		t.Errorf("chunked cagri sayisi = %d, beklenen 4", f.calls)
	}
	if out != "final analiz" {
		t.Errorf("final cikti = %q", out)
	}
	if usage.CompletionTokens != 20 {
		t.Errorf("chunked usage toplami = %+v", usage)
	}
}

func TestAnalyzeEmptySnapshot(t *testing.T) {
	f := &fakeAdapter{}
	_, _, err := Analyze(context.Background(), f, "m", SystemAnalyst, Snapshot{}, TaskFleetSummary, false, 16)
	if err == nil {
		t.Fatal("bos snapshot hata vermeli")
	}
}

func hasTitle(secs []Section, title string) bool {
	for _, s := range secs {
		if s.Title == title {
			return true
		}
	}
	return false
}

func titles(secs []Section) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		out[i] = s.Title
	}
	return out
}
