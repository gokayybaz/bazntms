package server

import (
	"net/http"

	"github.com/gokayybaz/bazntms/api"
)

// handleOpenAPIYAML, gömülü OpenAPI şemasını YAML olarak sunar.
func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Write(api.SpecYAML())
}

// handleOpenAPIJSON, aynı şemayı JSON'a çevirip sunar (Postman/Insomnia vb.).
func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	b, err := api.SpecJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// handleAPIDocs, şemayı tarayıcıda gezilebilir kılan tek-dosya sayfayı sunar.
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(api.DocsHTML())
}
