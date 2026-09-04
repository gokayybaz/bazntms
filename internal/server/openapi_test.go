package server

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/api"
	"gopkg.in/yaml.v3"
)

// routeRe, server.go icindeki `mux.Handle*("METHOD /yol", ...)` kayitlarini yakalar.
var routeRe = regexp.MustCompile(`mux\.(?:Handle|HandleFunc)\("(GET|POST|PUT|PATCH|DELETE) ([^"]+)"`)

// registeredRoutes, gercek yonlendiricide tanimli (method, path) ciftlerini
// dogrudan kaynaktan cikarir — canli istek yok, panic riski yok.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("server.go okunamadi: %v", err)
	}
	out := map[string]bool{}
	for _, m := range routeRe.FindAllStringSubmatch(string(src), -1) {
		method, path := m[1], m[2]
		if path == "/" || path == "/api/" || path == "/ws" {
			continue
		}
		out[method+" "+path] = true
	}
	if len(out) < 30 {
		t.Fatalf("cok az rota bulundu (regex bozuk?): %d", len(out))
	}
	return out
}

// specRoutes, OpenAPI semasindaki (method, path) ciftleri.
func specRoutes(t *testing.T) map[string]bool {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(api.SpecYAML(), &doc); err != nil {
		t.Fatalf("openapi.yaml: %v", err)
	}
	out := map[string]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				out[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	return out
}

// TestOpenAPICoversRouter — sema ile gercek yonlendirici birbirini tutmali.
// Yeni bir uc eklenip semaya islenmezse (ya da tam tersi) bu test kirilir.
func TestOpenAPICoversRouter(t *testing.T) {
	reg := registeredRoutes(t)
	spec := specRoutes(t)

	// /metrics, /healthz, /readyz gozlemlenebilirlik uclari da semada; onlar
	// server.go'da kayitli oldugu icin reg'de de var — ozel istisna gerekmez.

	var missingFromSpec, missingFromRouter []string
	for r := range reg {
		if !spec[r] {
			missingFromSpec = append(missingFromSpec, r)
		}
	}
	for r := range spec {
		// sema-ozel meta uclari (kaynakta da kayitli olmali; degilse hata)
		if !reg[r] {
			missingFromRouter = append(missingFromRouter, r)
		}
	}
	sort.Strings(missingFromSpec)
	sort.Strings(missingFromRouter)

	if len(missingFromSpec) > 0 {
		t.Errorf("yonlendiricide olup api/openapi.yaml'de olmayan %d uc:\n  %s",
			len(missingFromSpec), strings.Join(missingFromSpec, "\n  "))
	}
	if len(missingFromRouter) > 0 {
		t.Errorf("api/openapi.yaml'de olup yonlendiricide olmayan %d uc:\n  %s",
			len(missingFromRouter), strings.Join(missingFromRouter, "\n  "))
	}
}

// TestOpenAPIEndpointsServe — uc uclarin gercekten servis edildigini dogrular.
func TestOpenAPIEndpointsServe(t *testing.T) {
	ts := newTestServerWithEnroll(t)

	for _, tc := range []struct{ path, ctype string }{
		{"/api/openapi.yaml", "yaml"},
		{"/api/openapi.json", "json"},
		{"/api/docs", "html"},
	} {
		resp, err := ts.Client().Get(ts.URL + tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s: durum %d", tc.path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, tc.ctype) {
			t.Errorf("%s: content-type %q, %q bekleniyordu", tc.path, ct, tc.ctype)
		}
	}
}
