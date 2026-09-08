package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripThink(t *testing.T) {
	cases := map[string]string{
		"merhaba":                   "merhaba",
		"<think>gizli</think>cevap": "cevap",
		"a <think>x</think> b <think>y</think> c": "a  b  c",
		"<THINK>buyuk</THINK>son":                 "son",
		"<think>kapanmamis dusunme":               "",
	}
	for in, want := range cases {
		if got := stripThink(in); got != want {
			t.Errorf("stripThink(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

func TestFilterThinkStreamSplitTags(t *testing.T) {
	// <think> ve </think> etiketleri parcalar arasina bolunmus akis
	pieces := []string{"onsoz ", "<thi", "nk>giz", "li dusunme</thi", "nk> asil ", "cevap"}
	var buf strings.Builder
	inThink := false
	var out strings.Builder
	for _, p := range pieces {
		got, still := filterThinkStream(p, inThink, &buf)
		inThink = still
		out.WriteString(got)
	}
	if got := strings.TrimSpace(out.String()); got != "onsoz  asil cevap" {
		t.Fatalf("akan <think> filtresi = %q", got)
	}
}

func TestIsLocalURL(t *testing.T) {
	local := []string{
		"http://localhost:11434/v1", "http://127.0.0.1:1234", "https://[::1]:8080",
		"http://192.168.1.10/v1", "http://10.0.0.5", "http://ollama.local", "http://host.docker.internal:11434",
	}
	remote := []string{
		"https://api.openai.com/v1", "https://api.anthropic.com/v1", "https://openrouter.ai/api/v1",
		"http://8.8.8.8", "",
	}
	for _, u := range local {
		if !IsLocalURL(u) {
			t.Errorf("IsLocalURL(%q) = false, beklenen true", u)
		}
	}
	for _, u := range remote {
		if IsLocalURL(u) {
			t.Errorf("IsLocalURL(%q) = true, beklenen false", u)
		}
	}
}

func TestProviderReady(t *testing.T) {
	local := Provider{Name: "o", Kind: KindOllama, BaseURL: "http://localhost:11434/v1", Enabled: true}
	if !local.Ready() || local.NeedsKey() {
		t.Errorf("yerel saglayici anahtarsiz hazir olmali: ready=%v needsKey=%v", local.Ready(), local.NeedsKey())
	}
	cloud := Provider{Name: "o", Kind: KindOpenAI, BaseURL: "https://api.openai.com/v1", Enabled: true}
	if cloud.Ready() {
		t.Error("bulut saglayici anahtarsiz hazir olmamali")
	}
	cloud.APIKey = "sk-x"
	if !cloud.Ready() {
		t.Error("bulut saglayici anahtarla hazir olmali")
	}
}

// --- OpenAI-uyumlu adaptor ---

func TestOpenAIComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("beklenmeyen yol: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth basligi: %q", got)
		}
		var body oaiChatRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Stream {
			t.Error("stream=false bekleniyordu")
		}
		if body.Messages[0].Role != "system" || !strings.Contains(body.Messages[0].Content, "/no_think") {
			t.Errorf("no_think sistem mesajina eklenmemis: %q", body.Messages[0].Content)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"<think>x</think>analiz sonucu"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`)
	}))
	defer srv.Close()

	a := AdapterFor(Provider{Kind: KindOpenAICompat, BaseURL: srv.URL, APIKey: "sk-test"}, srv.Client())
	out, usage, err := a.Complete(context.Background(), ChatRequest{
		Model: "m", NoThink: true,
		Messages: []Message{{Role: RoleSystem, Content: "sen analizcisin"}, {Role: RoleUser, Content: "veri"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "analiz sonucu" {
		t.Errorf("cevap = %q (think temizlenmeli)", out)
	}
	if usage.PromptTokens != 12 || usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestOpenAICompleteReasoningFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"yedek cevap"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindOpenAICompat, BaseURL: srv.URL}, srv.Client())
	out, _, err := a.Complete(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}})
	if err != nil || out != "yedek cevap" {
		t.Fatalf("reasoning_content yedegi = %q, err=%v", out, err)
	}
}

func TestOpenAICompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"kota doldu"}}`)
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindOpenAICompat, BaseURL: srv.URL}, srv.Client())
	_, _, err := a.Complete(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "kota doldu") {
		t.Fatalf("hata mesaji saglayicidan gelmeli: %v", err)
	}
}

func TestOpenAIStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for _, chunk := range []string{
			`{"choices":[{"delta":{"content":"Trafik "}}]}`,
			`{"choices":[{"delta":{"content":"normal."}}]}`,
			`{"choices":[{"delta":{}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`,
			`[DONE]`,
		} {
			io.WriteString(w, "data: "+chunk+"\n\n")
			if fl != nil {
				fl.Flush()
			}
		}
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindOpenAICompat, BaseURL: srv.URL}, srv.Client())
	ch, err := a.Stream(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text strings.Builder
	var gotUsage *Usage
	for d := range ch {
		if d.Err != nil {
			t.Fatalf("delta err: %v", d.Err)
		}
		text.WriteString(d.Text)
		if d.Usage != nil {
			gotUsage = d.Usage
		}
	}
	if text.String() != "Trafik normal." {
		t.Errorf("akis metni = %q", text.String())
	}
	if gotUsage == nil || gotUsage.CompletionTokens != 3 {
		t.Errorf("akis usage = %+v", gotUsage)
	}
}

func TestOpenAIModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("yol: %s", r.URL.Path)
		}
		io.WriteString(w, `{"data":[{"id":"qwen2.5:7b"},{"id":"llama3.2"},{"id":""}]}`)
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindOllama, BaseURL: srv.URL}, srv.Client())
	models, err := a.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 2 || models[0] != "qwen2.5:7b" {
		t.Errorf("modeller = %v", models)
	}
}

// --- Anthropic native adaptor ---

func TestAnthropicComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("yol: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ak-1" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("anthropic basliklari eksik: %v", r.Header)
		}
		var body anthRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.System != "sen analizcisin" {
			t.Errorf("system alani = %q (ust duzey olmali, mesaj degil)", body.System)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Errorf("mesajlar = %+v", body.Messages)
		}
		if body.MaxTokens == 0 {
			t.Error("max_tokens zorunlu, 0 gonderilmis")
		}
		io.WriteString(w, `{"content":[{"type":"text","text":"anthropic analizi"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":9}}`)
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "ak-1"}, srv.Client())
	out, usage, err := a.Complete(context.Background(), ChatRequest{
		Model:    "claude-x",
		Messages: []Message{{Role: RoleSystem, Content: "sen analizcisin"}, {Role: RoleUser, Content: "veri"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "anthropic analizi" || usage.PromptTokens != 20 || usage.CompletionTokens != 9 {
		t.Errorf("out=%q usage=%+v", out, usage)
	}
}

func TestAnthropicStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		lines := []string{
			"event: message_start",
			`data: {"type":"message_start"}`,
			"event: content_block_delta",
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Anomali "}}`,
			"event: content_block_delta",
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"yok."}}`,
			"event: message_delta",
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
			"event: message_stop",
			`data: {"type":"message_stop"}`,
		}
		for _, l := range lines {
			io.WriteString(w, l+"\n")
			if strings.HasPrefix(l, "data:") {
				io.WriteString(w, "\n")
			}
			if fl != nil {
				fl.Flush()
			}
		}
	}))
	defer srv.Close()
	a := AdapterFor(Provider{Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "ak"}, srv.Client())
	ch, err := a.Stream(context.Background(), ChatRequest{Model: "c", Messages: []Message{{Role: RoleUser, Content: "x"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text strings.Builder
	var done bool
	for d := range ch {
		if d.Err != nil {
			t.Fatalf("delta err: %v", d.Err)
		}
		text.WriteString(d.Text)
		if d.Done {
			done = true
		}
	}
	if text.String() != "Anomali yok." || !done {
		t.Errorf("akis = %q done=%v", text.String(), done)
	}
}

func TestAnthropicKindRouting(t *testing.T) {
	if _, ok := AdapterFor(Provider{Kind: KindAnthropic, BaseURL: "https://api.anthropic.com/v1"}, nil).(*anthropicAdapter); !ok {
		t.Error("anthropic turu anthropicAdapter'a yonlenmeli")
	}
	if _, ok := AdapterFor(Provider{Kind: KindOllama, BaseURL: "http://x"}, nil).(*openaiAdapter); !ok {
		t.Error("ollama turu openaiAdapter'a yonlenmeli")
	}
}
