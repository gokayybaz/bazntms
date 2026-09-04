// Package api, hub HTTP API'sinin elle bakımlı OpenAPI 3.1 şemasını gömer.
// Şema `internal/server` içindeki bir sözleşme testiyle gerçek yönlendiriciye
// karşı doğrulanır (her yol var olmalı).
package api

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var specYAML []byte

//go:embed docs.html
var docsHTML []byte

// SpecYAML, OpenAPI dokümanını YAML olarak döndürür.
func SpecYAML() []byte { return specYAML }

// DocsHTML, şemayı tarayıcıda gösteren tek-dosya (harici bağımlılıksız) sayfa.
func DocsHTML() []byte { return docsHTML }

// SpecJSON, aynı dokümanı JSON'a çevirip döndürür. Şema derleme zamanında
// gömülü olduğu için hata pratikte "asla"; yine de döndürülür.
func SpecJSON() ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(specYAML, &doc); err != nil {
		return nil, fmt.Errorf("openapi yaml çözümlenemedi: %w", err)
	}
	return json.Marshal(doc)
}
