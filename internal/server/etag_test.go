package server

// Faz 16 S16.4 (D4): sık pollanan GET'lerde ETag / If-None-Match → 304.

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestETagNotModified(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "etag.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	st.RegisterAgent(store.Agent{Name: "a1", Site: "", TokenHash: "h1"})

	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	get := func(path, inm string) (int, string, string) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		defer resp.Body.Close()
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, resp.Header.Get("ETag"), string(buf[:n])
	}

	for _, path := range []string{"/api/v1/agents", "/api/alerts/events", "/api/v1/devices"} {
		code, etag, body := get(path, "")
		if code != 200 || etag == "" {
			t.Fatalf("%s: ilk istek 200 + ETag beklenirdi (code=%d etag=%q)", path, code, etag)
		}

		// aynı ETag → 304, gövde yok
		code2, _, body2 := get(path, etag)
		if code2 != http.StatusNotModified {
			t.Fatalf("%s: If-None-Match eşleşince 304 beklenirdi, %d", path, code2)
		}
		if body2 != "" {
			t.Fatalf("%s: 304 yanıtında gövde olmamalı: %q", path, body2)
		}

		// farklı ETag → 200 + gövde
		code3, _, body3 := get(path, `"deadbeef"`)
		if code3 != 200 || body3 == "" || body3 != body {
			t.Fatalf("%s: eşleşmeyen ETag → 200 + tam gövde beklenirdi (code=%d)", path, code3)
		}
	}

	// içerik değişince ETag değişir → yeni istek 200
	code, etag1, _ := get("/api/v1/agents", "")
	_ = code
	st.RegisterAgent(store.Agent{Name: "a2", TokenHash: "h2"})
	st.TouchAgent(2, "v", 1, "2.2.2.2")
	code2, etag2, _ := get("/api/v1/agents", etag1)
	if code2 != 200 || etag2 == etag1 {
		t.Fatalf("içerik değişince yeni ETag + 200 beklenirdi (code=%d etag eşit mi=%v)", code2, etag2 == etag1)
	}
}
