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

// anthropicAdapter, Anthropic native Messages API (/v1/messages). OpenAI tel
// bicimiyle uyumlu DEGIL: system ust duzey alan, mesajlar yalniz user/assistant,
// SSE olaylari farkli. Proje geneli hand-rolled net/http deseniyle tutarli
// (anthropic-sdk-go bagimliligi eklenmedi — bkz. fortigate/threatintel).
type anthropicAdapter struct {
	base string
	key  string
	opts Opts
	hc   *http.Client
}

const anthropicVersion = "2023-06-01"

// anthropicDefaultMaxTokens, Anthropic max_tokens zorunlu — istekte yoksa.
const anthropicDefaultMaxTokens = 4096

func (a *anthropicAdapter) Kind() Kind { return KindAnthropic }

func (a *anthropicAdapter) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if a.key != "" {
		r.Header.Set("x-api-key", a.key)
	}
	r.Header.Set("anthropic-version", anthropicVersion)
	for k, v := range a.opts.Headers {
		r.Header.Set(k, v)
	}
	return r, nil
}

type anthMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthRequest struct {
	Model       string        `json:"model"`
	System      string        `json:"system,omitempty"`
	Messages    []anthMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// buildBody, ChatRequest'i Anthropic'in bekledigi (system, messages) ciftine
// cevirir: system rollu mesajlar tek system string'ine toplanir, geri kalan
// user/assistant mesajlari siraya girer.
func (a *anthropicAdapter) buildBody(req ChatRequest, stream bool) anthRequest {
	var sys []string
	msgs := make([]anthMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			sys = append(sys, m.Content)
			continue
		}
		msgs = append(msgs, anthMessage{Role: string(m.Role), Content: m.Content})
	}
	maxTok := req.MaxTokens
	if a.opts.MaxTokens > 0 {
		maxTok = a.opts.MaxTokens
	}
	if maxTok <= 0 {
		maxTok = anthropicDefaultMaxTokens
	}
	temp := req.Temperature
	if a.opts.Temperature != nil {
		temp = *a.opts.Temperature
	}
	return anthRequest{
		Model:       req.Model,
		System:      strings.Join(sys, "\n\n"),
		Messages:    msgs,
		MaxTokens:   maxTok,
		Temperature: temp,
		Stream:      stream,
	}
}

func (a *anthropicAdapter) Complete(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body, _ := json.Marshal(a.buildBody(req, false))
	httpReq, err := a.newRequest(ctx, http.MethodPost, "/messages", bytes.NewReader(body))
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
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
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
	var sb strings.Builder
	for _, c := range cr.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	usage := Usage{PromptTokens: cr.Usage.InputTokens, CompletionTokens: cr.Usage.OutputTokens}
	content := stripThink(strings.TrimSpace(sb.String()))
	if content == "" {
		return "", usage, fmt.Errorf("AI bos yanit dondu (stop_reason=%s)", cr.StopReason)
	}
	return content, usage, nil
}

func (a *anthropicAdapter) Stream(ctx context.Context, req ChatRequest) (<-chan Delta, error) {
	body, _ := json.Marshal(a.buildBody(req, true))
	httpReq, err := a.newRequest(ctx, http.MethodPost, "/messages", bytes.NewReader(body))
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
		var thinkBuf strings.Builder
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
			if !strings.HasPrefix(line, "data:") {
				continue // "event:" satirlarini atla
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var ev struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &ev) != nil {
				continue
			}
			switch ev.Type {
			case "content_block_delta":
				if ev.Delta.Type != "text_delta" || ev.Delta.Text == "" {
					continue
				}
				out, stillIn := filterThinkStream(ev.Delta.Text, inThink, &thinkBuf)
				inThink = stillIn
				if !emit(out) {
					ch <- Delta{Err: ctx.Err()}
					return
				}
			case "message_delta":
				if ev.Usage != nil {
					select {
					case ch <- Delta{Usage: &Usage{PromptTokens: ev.Usage.InputTokens, CompletionTokens: ev.Usage.OutputTokens}}:
					case <-ctx.Done():
						ch <- Delta{Err: ctx.Err()}
						return
					}
				}
			case "message_stop":
				emit(flushThinkStream(inThink, &thinkBuf))
				ch <- Delta{Done: true}
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

func (a *anthropicAdapter) Models(ctx context.Context) ([]string, error) {
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
