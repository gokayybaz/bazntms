package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openaiAdapter, OpenAI-uyumlu /chat/completions + /models uclari. OpenAI,
// Ollama, LM Studio, vLLM, llama.cpp server, OpenRouter, DeepSeek, Groq,
// Together vb. bu tel bicimini konusur.
type openaiAdapter struct {
	base string
	key  string
	opts Opts
	hc   *http.Client
}

func (a *openaiAdapter) Kind() Kind {
	if strings.Contains(a.base, "openai.com") {
		return KindOpenAI
	}
	return KindOpenAICompat
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// reasoning modelleri (LM Studio, DeepSeek, OpenRouter) final cevabi
	// buraya yazabilir; bos content'te yedek:
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
}

type oaiChatRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (a *openaiAdapter) buildBody(req ChatRequest, stream bool) oaiChatRequest {
	msgs := make([]oaiMessage, 0, len(req.Messages))
	noThink := req.NoThink || a.opts.NoThink
	for i, m := range req.Messages {
		content := m.Content
		// Qwen3 "soft switch": sistem mesajina eklenince dusunme kapali
		if noThink && i == 0 && m.Role == RoleSystem {
			content += " /no_think"
		}
		msgs = append(msgs, oaiMessage{Role: string(m.Role), Content: content})
	}
	temp := req.Temperature
	if a.opts.Temperature != nil {
		temp = *a.opts.Temperature
	}
	maxTok := req.MaxTokens
	if a.opts.MaxTokens > 0 {
		maxTok = a.opts.MaxTokens
	}
	return oaiChatRequest{Model: req.Model, Messages: msgs, Temperature: temp, MaxTokens: maxTok, Stream: stream}
}

func (a *openaiAdapter) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if a.key != "" {
		r.Header.Set("Authorization", "Bearer "+a.key)
	}
	for k, v := range a.opts.Headers {
		r.Header.Set(k, v)
	}
	return r, nil
}

func (a *openaiAdapter) Complete(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body, _ := json.Marshal(a.buildBody(req, false))
	httpReq, err := a.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, err
	}
	resp, err := a.hc.Do(httpReq)
	if err != nil {
		return "", Usage{}, fmt.Errorf("AI servisine ulasilamadi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var cr struct {
		Choices []struct {
			Message      oaiMessage `json:"message"`
			FinishReason string     `json:"finish_reason"`
		} `json:"choices"`
		Usage oaiUsage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &cr)

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(raw))
		if cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		return "", Usage{}, fmt.Errorf("AI servisi hata dondu (HTTP %d): %s", resp.StatusCode, msg)
	}
	if len(cr.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("AI yaniti bos dondu")
	}
	content := pickContent(cr.Choices[0].Message)
	usage := Usage{PromptTokens: cr.Usage.PromptTokens, CompletionTokens: cr.Usage.CompletionTokens}
	if content == "" {
		if cr.Choices[0].FinishReason == "length" {
			return "", usage, fmt.Errorf("model dusunme asamasinda token limitini asti: saglayici opts.max_tokens artirin ya da no_think acin")
		}
		return "", usage, fmt.Errorf("AI bos yanit dondu (model cok kucuk olabilir, istek reddedilmis ya da baglam tasmis olabilir)")
	}
	return content, usage, nil
}

func (a *openaiAdapter) Stream(ctx context.Context, req ChatRequest) (<-chan Delta, error) {
	body, _ := json.Marshal(a.buildBody(req, true))
	httpReq, err := a.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := a.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI servisine ulasilamadi: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		msg := strings.TrimSpace(string(raw))
		var e struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		return nil, fmt.Errorf("AI servisi hata dondu (HTTP %d): %s", resp.StatusCode, msg)
	}

	ch := make(chan Delta)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		var thinkBuf strings.Builder // <think> gozardi durumu icin akan tampon
		inThink := false
		emit := func(text string) bool {
			if text == "" {
				return true
			}
			select {
			case ch <- Delta{Text: text}:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *oaiUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Usage != nil {
				select {
				case ch <- Delta{Usage: &Usage{PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens}}:
				case <-ctx.Done():
					ch <- Delta{Err: ctx.Err()}
					return
				}
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			piece := chunk.Choices[0].Delta.Content
			if piece == "" {
				continue // reasoning_content akisini gostermeyiz
			}
			// akan <think>...</think> filtreleme
			out, stillIn := filterThinkStream(piece, inThink, &thinkBuf)
			inThink = stillIn
			if !emit(out) {
				ch <- Delta{Err: ctx.Err()}
				return
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			ch <- Delta{Err: err}
			return
		}
		emit(flushThinkStream(inThink, &thinkBuf))
		ch <- Delta{Done: true}
	}()
	return ch, nil
}

func (a *openaiAdapter) Models(ctx context.Context) ([]string, error) {
	httpReq, err := a.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("model servisine ulasilamadi (%s): %w", a.base, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model listesi alinamadi (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

// pickContent, reasoning modelleri icin content bos ise reasoning alanlarina
// duser; sonra <think> bloklarini temizler.
func pickContent(m oaiMessage) string {
	c := strings.TrimSpace(m.Content)
	if c == "" {
		c = strings.TrimSpace(m.ReasoningContent)
	}
	if c == "" {
		c = strings.TrimSpace(m.Reasoning)
	}
	return stripThink(c)
}

// filterThinkStream, akan metinde <think>...</think> arasini yutar. buf,
// etiketin parcalar arasina bolunmesine karsi kismi-eslesme tamponu: yalnizca
// bir sonraki etiketin ONEKI olabilecek son eki tutar (gercek icerik hemen
// yayilir). Akis bitiminde buf'ta kalan (yalniz kismi etiket oneki olabilir)
// icin flushThinkStream cagrilir.
func filterThinkStream(piece string, inThink bool, buf *strings.Builder) (string, bool) {
	buf.WriteString(piece)
	s := buf.String()
	var out strings.Builder
	for {
		if inThink {
			end := indexFold(s, "</think>")
			if end < 0 {
				keep := partialSuffixLen(s, "</think>")
				buf.Reset()
				buf.WriteString(s[len(s)-keep:])
				return out.String(), true
			}
			s = s[end+len("</think>"):]
			inThink = false
			continue
		}
		start := indexFold(s, "<think>")
		if start < 0 {
			keep := partialSuffixLen(s, "<think>")
			out.WriteString(s[:len(s)-keep])
			buf.Reset()
			buf.WriteString(s[len(s)-keep:])
			return out.String(), false
		}
		out.WriteString(s[:start])
		s = s[start+len("<think>"):]
		inThink = true
	}
}

// flushThinkStream, akis bitiminde tamponda kalani dondurur. inThink ise
// (kapanmamis <think>) icerik atilir; degilse geri kalan gercek metindir.
func flushThinkStream(inThink bool, buf *strings.Builder) string {
	s := buf.String()
	buf.Reset()
	if inThink {
		return ""
	}
	return s
}

// indexFold, s icinde sub'in ilk konumunu buyuk/kucuk harf duyarsiz bulur.
func indexFold(s, sub string) int {
	return strings.Index(strings.ToLower(s), sub)
}

// partialSuffixLen, s'in en uzun son ekinin (tag'in bir oneki olan; tam
// uzunluktan kisa) uzunlugunu dondurur — akis parcalari arasina bolunmus
// etiketi tutmak icin.
func partialSuffixLen(s, tag string) int {
	max := len(tag) - 1
	if len(s) < max {
		max = len(s)
	}
	for k := max; k > 0; k-- {
		if strings.EqualFold(s[len(s)-k:], tag[:k]) {
			return k
		}
	}
	return 0
}
