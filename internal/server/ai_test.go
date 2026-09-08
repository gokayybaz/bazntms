package server

// Faz 26: AI analiz uçları — sağlayıcı CRUD (anahtar maskeleme + RBAC),
// sohbet + SSE streaming, bootstrap.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// mockLLM, OpenAI-uyumlu bir sahte model sunucusu.
func mockLLM(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			io.WriteString(w, `{"data":[{"id":"test-model"}]}`)
		case "/chat/completions":
			var body struct {
				Stream bool `json:"stream"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				fl, _ := w.(http.Flusher)
				for _, c := range []string{
					`{"choices":[{"delta":{"content":"Filo "}}]}`,
					`{"choices":[{"delta":{"content":"sağlıklı."}}]}`,
					`{"choices":[{"delta":{}}],"usage":{"prompt_tokens":40,"completion_tokens":6}}`,
					`[DONE]`,
				} {
					io.WriteString(w, "data: "+c+"\n\n")
					if fl != nil {
						fl.Flush()
					}
				}
				return
			}
			io.WriteString(w, `{"choices":[{"message":{"content":"tek atış"}}],"usage":{"prompt_tokens":10,"completion_tokens":3}}`)
		default:
			w.WriteHeader(404)
		}
	}))
}

func aiTestServer(t *testing.T, password string) (*httptest.Server, *Server) {
	t.Helper()
	ts, srv, st := newRBACServerEx(t, password, "", false)
	v, err := vault.Open(filepath.Join(t.TempDir(), "vault.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	reg := ai.NewRegistry(st, v, func() ai.Config {
		return ai.Config{Enabled: true, AllowCloud: true, MaxContextKB: 16}
	})
	srv.SetAIRegistry(reg)
	return ts, srv
}

func adminToken(t *testing.T, ts *httptest.Server, password string) string {
	t.Helper()
	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": password})
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatal("admin token alınamadı")
	}
	return tok
}

func TestAIProviderCRUDAndMasking(t *testing.T) {
	llm := mockLLM(t)
	defer llm.Close()
	ts, _ := aiTestServer(t, "admin-pass-xyz")
	admin := adminToken(t, ts, "admin-pass-xyz")

	// oluştur (anahtarlı)
	st, out := postJSON(t, ts, "/api/v1/ai/providers", admin, map[string]any{
		"name": "test", "kind": "openai-compat", "base_url": llm.URL,
		"api_key": "gizli-anahtar", "default_model": "test-model", "enabled": true,
	})
	if st != http.StatusOK {
		t.Fatalf("create: %d %v", st, out)
	}

	// liste: api_key düz görünmemeli, has_key=true
	st, listRaw := getRaw(t, ts, "/api/v1/ai/providers", admin)
	if st != http.StatusOK {
		t.Fatalf("list: %d", st)
	}
	if strings.Contains(listRaw, "gizli-anahtar") {
		t.Fatal("api_key düz metin sızdı!")
	}
	if !strings.Contains(listRaw, `"has_key":true`) {
		t.Errorf("has_key:true bekleniyordu: %s", listRaw)
	}

	// güncelle: anahtar boş → korunur
	var provs []map[string]any
	json.Unmarshal([]byte(listRaw), &provs)
	id := int(provs[0]["id"].(float64))
	st, _ = putJSON(t, ts, "/api/v1/ai/providers/"+itoa(id), admin, map[string]any{
		"name": "test2", "kind": "openai-compat", "base_url": llm.URL, "enabled": true,
	})
	if st != http.StatusOK {
		t.Fatalf("update: %d", st)
	}
	_, listRaw2 := getRaw(t, ts, "/api/v1/ai/providers", admin)
	if !strings.Contains(listRaw2, `"has_key":true`) || !strings.Contains(listRaw2, `"test2"`) {
		t.Errorf("boş anahtar update anahtarı silmemeli / ad güncellenmeli: %s", listRaw2)
	}

	// test ucu
	st, testOut := postJSON(t, ts, "/api/v1/ai/providers/"+itoa(id)+"/test", admin, nil)
	if st != http.StatusOK || testOut["ok"] != true {
		t.Fatalf("test ucu: %d %v", st, testOut)
	}

	// sil
	st, _ = deleteReq(t, ts, "/api/v1/ai/providers/"+itoa(id), admin)
	if st != http.StatusOK {
		t.Fatalf("delete: %d", st)
	}
}

func TestAIChatStreaming(t *testing.T) {
	llm := mockLLM(t)
	defer llm.Close()
	ts, _ := aiTestServer(t, "admin-pass-xyz")
	admin := adminToken(t, ts, "admin-pass-xyz")

	postJSON(t, ts, "/api/v1/ai/providers", admin, map[string]any{
		"name": "p", "kind": "openai-compat", "base_url": llm.URL,
		"default_model": "test-model", "enabled": true,
	})

	_, convOut := postJSON(t, ts, "/api/v1/ai/conversations", admin, map[string]any{"scope_kind": "fleet"})
	cid := int(convOut["id"].(float64))

	// SSE mesaj
	body, _ := json.Marshal(map[string]any{"content": "filo nasıl?"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/conversations/"+itoa(cid)+"/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE isteği: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	var text strings.Builder
	var sawDone bool
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev)
		if d, ok := ev["delta"].(string); ok {
			text.WriteString(d)
		}
		if ev["done"] == true {
			sawDone = true
		}
	}
	if text.String() != "Filo sağlıklı." || !sawDone {
		t.Fatalf("akış = %q done=%v", text.String(), sawDone)
	}

	// konuşma kaydı: user + assistant mesajı kalıcı
	st, convRaw := getRaw(t, ts, "/api/v1/ai/conversations/"+itoa(cid), admin)
	if st != http.StatusOK {
		t.Fatalf("conv get: %d", st)
	}
	if !strings.Contains(convRaw, "Filo sağlıklı.") || !strings.Contains(convRaw, "filo nasıl?") {
		t.Errorf("mesajlar kalıcı değil: %s", convRaw)
	}

	// arşivle → aktif listeden düşer
	deleteReq(t, ts, "/api/v1/ai/conversations/"+itoa(cid), admin)
	_, activeRaw := getRaw(t, ts, "/api/v1/ai/conversations?scope=fleet", admin)
	if strings.Contains(activeRaw, itoa(cid)+`,"title"`) {
		t.Errorf("arşivlenen konuşma aktif listede: %s", activeRaw)
	}
}

func TestAIRBAC(t *testing.T) {
	ts, _ := aiTestServer(t, "admin-pass-xyz")
	admin := adminToken(t, ts, "admin-pass-xyz")

	// viewer + analyst kullanıcıları
	postJSON(t, ts, "/api/v1/users", admin, map[string]any{"username": "v", "password": "vv-password", "role": "viewer"})
	postJSON(t, ts, "/api/v1/users", admin, map[string]any{"username": "an", "password": "an-password", "role": "analyst"})
	_, vo := postJSON(t, ts, "/api/login", "", map[string]string{"username": "v", "password": "vv-password"})
	viewerTok := vo["token"].(string)
	_, ao := postJSON(t, ts, "/api/login", "", map[string]string{"username": "an", "password": "an-password"})
	analystTok := ao["token"].(string)

	// viewer: sohbet listesi 403 (PermAnalyze)
	if st, _ := getJSON(t, ts, "/api/v1/ai/conversations", viewerTok); st != http.StatusForbidden {
		t.Errorf("viewer /ai/conversations 403 beklenirdi: %d", st)
	}
	// analyst: sohbet OK
	if st, _ := getJSON(t, ts, "/api/v1/ai/conversations", analystTok); st != http.StatusOK {
		t.Errorf("analyst /ai/conversations 200 beklenirdi: %d", st)
	}
	// analyst: sağlayıcı listesi 403 (PermGlobalAdmin)
	if st, _ := getJSON(t, ts, "/api/v1/ai/providers", analystTok); st != http.StatusForbidden {
		t.Errorf("analyst /ai/providers 403 beklenirdi: %d", st)
	}
}

func TestAIDisabled(t *testing.T) {
	// SetAIRegistry çağrılmamış → 503
	ts, _, _ := newRBACServerEx(t, "admin-pass-xyz", "", false)
	admin := adminToken(t, ts, "admin-pass-xyz")
	if st, _ := getJSON(t, ts, "/api/v1/ai/providers", admin); st != http.StatusServiceUnavailable {
		t.Errorf("AI kapalıyken 503 beklenirdi: %d", st)
	}
	// status yine de 200 döner (enabled:false)
	if st, out := getJSON(t, ts, "/api/v1/ai/status", admin); st != http.StatusOK || out["enabled"] != false {
		t.Errorf("status enabled:false beklenirdi: %d %v", st, out)
	}
}

// --- küçük yardımcılar ---

func getRaw(t *testing.T, ts *httptest.Server, path, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func deleteReq(t *testing.T, ts *httptest.Server, path, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func itoa(n int) string { return strconv.Itoa(n) }
