// Komut: go run ./tools/banner
// bazNTMS sosyal medya gorsellerini (1280x640 + 1200x627) uretir.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	colBg    = color.RGBA{2, 6, 23, 255}      // slate-950
	colDot   = color.RGBA{148, 163, 184, 14}  // slate-400 ~%5
	colAcc   = color.RGBA{34, 211, 238, 255}  // cyan-400
	colWhite = color.RGBA{248, 250, 252, 255} // slate-50
	colMut   = color.RGBA{148, 163, 184, 255} // slate-400
	colDim   = color.RGBA{100, 116, 139, 255} // slate-500
	colLine  = color.RGBA{51, 65, 85, 255}    // slate-700
)

type canvas struct {
	img *image.RGBA
	w   int
	h   int
}

func (c *canvas) rect(x0, y0, x1, y1 int, col color.Color) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.img.Set(x, y, col)
		}
	}
}

func (c *canvas) rectBorder(x0, y0, x1, y1, t int, col color.Color) {
	c.rect(x0, y0, x1, y0+t, col)
	c.rect(x0, y1-t, x1, y1, col)
	c.rect(x0, y0, x0+t, y1, col)
	c.rect(x1-t, y0, x1, y1, col)
}

func (c *canvas) circle(cx, cy, r int, col color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				c.img.Set(x, y, col)
			}
		}
	}
}

func (c *canvas) line(x0, y0, x1, y1 int, col color.Color) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.rect(x0-1, y0-1, x0+1, y0+1, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func textDrawer(c *canvas, size float64, bold bool, col color.Color) *font.Drawer {
	fnt, err := opentype.Parse(pickFont(bold))
	if err != nil {
		panic(err)
	}
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(err)
	}
	return &font.Drawer{Dst: c.img, Src: image.NewUniform(col), Face: face}
}

func pickFont(bold bool) []byte {
	if bold {
		return gobold.TTF
	}
	return goregular.TTF
}

// spacedText, harf aralikli buyuk harf metni cizer (kurumsal alt basliklar).
func spacedText(c *canvas, text string, x, y int, size float64, spacing int, col color.Color) {
	d := textDrawer(c, size, false, col)
	for _, r := range text {
		d.Dot = fixed.P(x, y)
		d.DrawString(string(r))
		w := d.MeasureString(string(r))
		x += w.Ceil() + spacing
	}
}

func centerText(c *canvas, text string, cx, y int, size float64, bold bool, col color.Color) {
	d := textDrawer(c, size, bold, col)
	w := d.MeasureString(text).Ceil()
	d.Dot = fixed.P(cx-w/2, y)
	d.DrawString(text)
}

func render(w, h int, path string) {
	c := &canvas{img: image.NewRGBA(image.Rect(0, 0, w, h)), w: w, h: h}
	c.rect(0, 0, w, h, colBg)

	// nokta izgarasi (blueprint dokusu)
	for y := 22; y < h; y += 26 {
		for x := 22; x < w; x += 26 {
			c.rect(x, y, x+1, y+1, colDot)
		}
	}

	// ust aksan cizgisi
	c.rect(0, 0, w, 4, colAcc)

	// sol altta ag topolojisi motifi (metin blogu sagda)
	mx, my := 190, h-190
	off := 110
	c.line(mx, my, mx-off, my-off, colLine)
	c.line(mx, my, mx+off, my-off, colLine)
	c.line(mx, my, mx-off, my+off, colLine)
	c.line(mx, my, mx+off, my+off, colLine)
	c.circle(mx-off, my-off, 10, colDim)
	c.circle(mx+off, my-off, 10, colDim)
	c.circle(mx-off, my+off, 10, colDim)
	c.circle(mx+off, my+off, 10, colDim)
	c.circle(mx, my, 14, colAcc)

	// sag kenar payi
	rx := w - 96

	// baslik: baz (cyan) + NTMS (beyaz) — sag ustte
	ty := int(float64(h) * 0.20)
	d := textDrawer(c, float64(h)*0.21, true, colAcc)
	dBaz := d.MeasureString("baz").Ceil()
	d2 := textDrawer(c, float64(h)*0.21, true, colWhite)
	dNtms := d2.MeasureString("NTMS").Ceil()
	d.Dot = fixed.P(rx-dBaz-dNtms, ty)
	d.DrawString("baz")
	d2.Dot = fixed.P(rx-dNtms, ty)
	d2.DrawString("NTMS")

	// alt baslik (saga hizali)
	sub := "B A Z   N E T W O R K   T R A F F I C   M O N I T O R I N G   S Y S T E M"
	{
		dm := textDrawer(c, float64(h)*0.038, false, colMut)
		sw := 0
		for _, r := range sub {
			sw += dm.MeasureString(string(r)).Ceil() + 1
		}
		spacedText(c, sub, rx-sw, int(float64(h)*0.32), float64(h)*0.038, 1, colMut)
	}

	// ozellik satiri (saga hizali)
	features := "Canlı paket yakalama · SQLite · Uyarılar · AI analizi · PCAP · Rapor"
	d3 := textDrawer(c, float64(h)*0.055, false, color.RGBA{203, 213, 225, 255})
	fw := d3.MeasureString(features).Ceil()
	d3.Dot = fixed.P(rx-fw, int(float64(h)*0.46))
	d3.DrawString(features)

	// teknik chip'ler (saga hizali)
	chips := []string{"Go", "gopacket / libpcap", "SQLite", "React + Vite", "Ollama AI"}
	chipY := int(float64(h) * 0.58)
	gap, pad := 16, 18
	chH := int(float64(h) * 0.062)
	d4 := textDrawer(c, float64(h)*0.042, false, colMut)
	widths := make([]int, len(chips))
	total := 0
	for i, ch := range chips {
		widths[i] = d4.MeasureString(ch).Ceil() + pad*2
		total += widths[i]
		if i < len(chips)-1 {
			total += gap
		}
	}
	cx := rx - total
	for i, ch := range chips {
		c.rectBorder(cx, chipY, cx+widths[i], chipY+chH, 1, colLine)
		d4.Dot = fixed.P(cx+pad, chipY+chH/2+int(float64(h)*0.016))
		d4.DrawString(ch)
		cx += widths[i] + gap
	}

	// alt bilgi (saga hizali)
	d5 := textDrawer(c, float64(h)*0.043, false, colDim)
	gw := d5.MeasureString("github.com/gokayybaz/bazntms").Ceil()
	d5.Dot = fixed.P(rx-gw, int(float64(h)*0.93))
	d5.DrawString("github.com/gokayybaz/bazntms")

	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, c.img); err != nil {
		panic(err)
	}
	fmt.Println("uretildi:", path, fmt.Sprintf("(%dx%d)", w, h))
}

func main() {
	render(1280, 640, "marketing/banner-1280x640.png")   // GitHub social preview
	render(1200, 627, "marketing/linkedin-1200x627.png") // LinkedIn link gorseli
}
