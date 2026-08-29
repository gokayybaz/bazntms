// Package web, derlenmis frontend ciktisini (web/dist) go:embed ile
// binary'ye gomuler. Vite build outDir'i web/dist olarak ayarlidir.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist, frontend build ciktisini fs.FS olarak dondurur. Git'te yalnizca
// web/dist/.gitkeep bulunur (npm run build ile gercek dosyalar gelir).
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
