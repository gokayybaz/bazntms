package server

// Guncelleme kanali sunumu (Faz 7.3): hub, updates/<channel>/ dizinindeki
// manifest.json + binary dosyalarini agent'lara sunar. Icerik uretimi
// bazntmsctl update sign ile yapilir; hub yalnizca sunucudur.
//
// Uclar (agentAuth korumali):
//   GET /api/v1/agent/update/manifest?channel=stable
//   GET /api/v1/agent/update/file/{channel}/{name}

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SetUpdatesDir, guncelleme kanali dizinini ayarlar (bos = kapali).
func (s *Server) SetUpdatesDir(dir string) { s.updatesDir = dir }

// updatesSafe, dosya adi path traversal'a karsi temizler: yalnizca
// [A-Za-z0-9._-] kabul edilir; ".." reddedilir.
func updatesSafe(name string) (string, bool) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", false
	}
	if strings.HasPrefix(name, ".") {
		return "", false
	}
	for _, ch := range name {
		ok := ch == '-' || ch == '_' || ch == '.' ||
			(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if !ok {
			return "", false
		}
	}
	return name, true
}

func updatesChannel(ch string) (string, bool) {
	switch ch {
	case "stable", "beta":
		return ch, true
	}
	return "", false
}

func (s *Server) handleUpdateManifest(w http.ResponseWriter, r *http.Request) {
	ch, ok := updatesChannel(r.URL.Query().Get("channel"))
	if !ok {
		http.Error(w, "channel stable|beta", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.updatesDir, ch, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, fmt.Sprintf("kanal sunulmuyor: %s", ch), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request) {
	ch, ok := updatesChannel(r.PathValue("channel"))
	if !ok {
		http.Error(w, "channel stable|beta", http.StatusBadRequest)
		return
	}
	name, ok := updatesSafe(r.PathValue("name"))
	if !ok {
		http.Error(w, "geçersiz dosya adı", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.updatesDir, ch, name)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "dosya bulunamadı", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.Error(w, "dosya bulunamadı", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprint(fi.Size()))
	w.WriteHeader(http.StatusOK)
	buf := make([]byte, 256*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
