package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJevDecideSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/systemone" {
			t.Fatalf("beklenmeyen yol: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization basligi = %q", got)
		}
		var body jevRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("istek govdesi ayristirilamadi: %v", err)
		}
		if body.Model != "jev-latest" {
			t.Fatalf("model = %q", body.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"worth_triage": {"type": "noul", "noul": 0.87},
				"kind": {"type": "choice", "choice": "exfil", "probabilities": {"exfil": 0.7, "benign": 0.3}, "confidence": 0.7},
				"risk": {"type": "score", "score": 1.6, "probabilities": {"0": 0.05, "1": 0.3, "2": 0.65}}
			},
			"usage": {"input_tokens": 120, "output_tokens": 8}
		}`))
	}))
	defer srv.Close()

	c := NewJevClient(srv.URL, "test-key", "", nil)
	answers, usage, err := c.Decide(context.Background(), map[string]any{"x": 1}, map[string]JevQuestion{
		"worth_triage": {Type: JevNoul, Instructions: "?"},
	})
	if err != nil {
		t.Fatalf("Decide hata döndürdü: %v", err)
	}
	if usage.PromptTokens != 120 || usage.CompletionTokens != 8 {
		t.Fatalf("usage = %+v", usage)
	}
	if a := answers["worth_triage"]; a.Noul == nil || *a.Noul != 0.87 {
		t.Fatalf("worth_triage.noul = %+v", a)
	}
	if a := answers["kind"]; a.Choice != "exfil" || a.Confidence == nil || *a.Confidence != 0.7 {
		t.Fatalf("kind cevabı = %+v", a)
	}
	if a := answers["risk"]; a.Score == nil || *a.Score != 1.6 {
		t.Fatalf("risk cevabı = %+v", a)
	}
}

func TestJevDecideHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"missing or invalid api key"}`))
	}))
	defer srv.Close()

	c := NewJevClient(srv.URL, "bad-key", "", nil)
	_, _, err := c.Decide(context.Background(), "x", map[string]JevQuestion{"q": {Type: JevNoul}})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("beklenen HTTP 401 hatası, gelen: %v", err)
	}
}

func TestJevDecideMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := NewJevClient(srv.URL, "k", "", nil)
	_, _, err := c.Decide(context.Background(), "x", map[string]JevQuestion{"q": {Type: JevNoul}})
	if err == nil {
		t.Fatal("bozuk yanıt hata döndürmeliydi")
	}
}

func TestNewJevClientDefaults(t *testing.T) {
	c := NewJevClient("", "k", "", nil)
	if c.baseURL != jevDefaultBaseURL || c.model != jevDefaultModel {
		t.Fatalf("varsayılanlar: baseURL=%q model=%q", c.baseURL, c.model)
	}
}
