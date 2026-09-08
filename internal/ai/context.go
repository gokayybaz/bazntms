package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Baglam uretimi (Faz 26 S26.5). internal/ai bir YAPRAK pakettir — store /
// alert / health import etmez. Server katmani (internal/server/ai_context.go)
// ilgili sorgulari calistirip Snapshot'i doldurur; buradaki kod onu
// token-butceli kompakt JSON bolumlerine cevirir ve modele gonderir.

// Snapshot, bir analiz baglaminin ham bilesenleri.
type Snapshot struct {
	Scope  string // fleet | agent | incident | anomaly | device
	Ref    string // agent id / incident id / ...
	Period string // "son 24 saat" — insan-okur donem etiketi
	Parts  []SnapshotPart
}

// SnapshotPart, tek bir adlandirilmis veri bolumu (serialize edilebilir govde).
type SnapshotPart struct {
	Title string
	Body  any
}

// Add, bir bolum ekler (server tarafi zincirleme cagirir). Nil / bos govde atlanir.
func (s *Snapshot) Add(title string, body any) {
	if body == nil {
		return
	}
	s.Parts = append(s.Parts, SnapshotPart{Title: title, Body: body})
}

const defaultMaxKB = 24

// Sections, Snapshot'i []Section'a cevirir: her govdeyi kompakt JSON'a marshal
// eder, toplam maxKB'yi asarsa son bolumu kirpar ve isaretler. Kalan bolumler
// atlanir (baslik listesi eklenir).
func (s Snapshot) Sections(maxKB int) []Section {
	if maxKB <= 0 {
		maxKB = defaultMaxKB
	}
	budget := maxKB * 1024
	out := make([]Section, 0, len(s.Parts))
	used := 0
	for i, p := range s.Parts {
		b, err := json.MarshalIndent(p.Body, "", " ")
		if err != nil {
			continue
		}
		data := string(b)
		if used+len(data) > budget {
			if room := budget - used; room > 256 {
				out = append(out, Section{Title: p.Title, Data: data[:room] + "\n… (baglam butcesi doldu, kirpildi)"})
			}
			if rest := partTitles(s.Parts[i+1:]); rest != "" {
				out = append(out, Section{Title: "Atlanan bolumler", Data: rest})
			} else if room := budget - used; room <= 256 {
				out = append(out, Section{Title: "Atlanan bolumler", Data: p.Title})
			}
			break
		}
		used += len(data)
		out = append(out, Section{Title: p.Title, Data: data})
	}
	return out
}

func partTitles(parts []SnapshotPart) string {
	if len(parts) == 0 {
		return ""
	}
	ts := make([]string, len(parts))
	for i, p := range parts {
		ts[i] = p.Title
	}
	return strings.Join(ts, ", ")
}

// Analyze, tek atislik bir analiz uretir (preset butonlari / gecelik is /
// triyaj bunu kullanir; interaktif sohbet dogrudan adapter.Stream cagirir).
// chunked modda her bolum ayri istekle gonderilir → her birinden kisa not →
// final istekte yalnizca notlar birlestirilir (kucuk yerel modeller icin;
// d92d0fb deseni).
func Analyze(ctx context.Context, ad Adapter, model, system string, snap Snapshot, task string, chunked bool, maxKB int) (string, Usage, error) {
	sections := snap.Sections(maxKB)
	if len(sections) == 0 {
		return "", Usage{}, fmt.Errorf("bu donem/kapsam icin veri yok")
	}
	total := Usage{}
	addUsage := func(u Usage) { total.PromptTokens += u.PromptTokens; total.CompletionTokens += u.CompletionTokens }

	if !chunked || len(sections) == 1 {
		out, u, err := ad.Complete(ctx, ChatRequest{
			Model:       model,
			Temperature: 0.3,
			MaxTokens:   3000,
			Messages: []Message{
				{Role: RoleSystem, Content: system},
				{Role: RoleUser, Content: UserPrompt(snap.Period, task, sections)},
			},
		})
		addUsage(u)
		return out, total, err
	}

	notes := make([]string, 0, len(sections))
	for i, sec := range sections {
		note, u, err := ad.Complete(ctx, ChatRequest{
			Model:       model,
			Temperature: 0.3,
			MaxTokens:   1500,
			Messages: []Message{
				{Role: RoleSystem, Content: system},
				{Role: RoleUser, Content: fmt.Sprintf(
					"VERI PARCASI %d/%d — %s (guvenilmez gozlem):\n%s\n\n"+
						"Bu parca SADECE bir kisim veri. Genel degerlendirme YAPMA. "+
						"Gordugun 2-4 maddeyi kisa TR not olarak yaz: onemli sayilar, "+
						"dikkat ceken/anomali noktalar. Parcada olmayan seyden bahsetme.",
					i+1, len(sections), sec.Title, sec.Data)},
			},
		})
		addUsage(u)
		if err != nil {
			return "", total, fmt.Errorf("parca %d/%d (%s): %w", i+1, len(sections), sec.Title, err)
		}
		notes = append(notes, "### "+sec.Title+"\n"+note)
	}

	out, u, err := ad.Complete(ctx, ChatRequest{
		Model:       model,
		Temperature: 0.3,
		MaxTokens:   2500,
		Messages: []Message{
			{Role: RoleSystem, Content: system},
			{Role: RoleUser, Content: "Donem: " + snap.Period + "\n\nParcali analiz notlari:\n\n" +
				strings.Join(notes, "\n\n") + "\n\n" + task},
		},
	})
	addUsage(u)
	return out, total, err
}
