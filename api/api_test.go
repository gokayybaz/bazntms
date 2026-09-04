package api

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecParses(t *testing.T) {
	var doc struct {
		OpenAPI string           `yaml:"openapi"`
		Info    map[string]any   `yaml:"info"`
		Paths   map[string]any   `yaml:"paths"`
		Comp    map[string]any   `yaml:"components"`
		Tags    []map[string]any `yaml:"tags"`
	}
	if err := yaml.Unmarshal(SpecYAML(), &doc); err != nil {
		t.Fatalf("openapi.yaml çözümlenemedi: %v", err)
	}
	if doc.OpenAPI == "" || doc.Info["title"] == nil {
		t.Fatalf("openapi/info eksik: %+v", doc)
	}
	if len(doc.Paths) < 40 {
		t.Fatalf("beklenenden az yol: %d", len(doc.Paths))
	}
	if doc.Comp["schemas"] == nil || doc.Comp["responses"] == nil {
		t.Fatal("components.schemas / components.responses eksik")
	}
	if len(doc.Tags) < 5 {
		t.Fatalf("etiket tanımları eksik: %d", len(doc.Tags))
	}
}

func TestSpecJSON(t *testing.T) {
	b, err := SpecJSON()
	if err != nil {
		t.Fatalf("SpecJSON: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("JSON geçersiz: %v", err)
	}
	if m["openapi"] == nil || m["paths"] == nil {
		t.Fatal("JSON'da openapi/paths yok")
	}
}

func TestDocsHTML(t *testing.T) {
	if len(DocsHTML()) < 500 {
		t.Fatal("docs.html gömülmedi")
	}
}
