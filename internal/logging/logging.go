// Package logging, kurumsal loglama zeminini kurar: JSON/text format,
// seviye kontrolu ve stdlib log cikisinin slog'a yonlendirilmesi. Boylece
// tum paketlerdeki mevcut log.Printf cagrilari tek formata dusenir.
package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Options, loglama yapilandirmasi.
type Options struct {
	Level  string    // debug | info | warn | error (varsayilan info)
	Format string    // json | text (varsayilan text; prod/hub icin json onerilir)
	Out    io.Writer // nil → stdout (servis modunda dosya verilir)
}

// Setup, slog varsayilan logger'ini kurar ve stdlib log cikisini ayni
// handler'a yonlendirir. Donen deger, ayrik kullanım icin *slog.Logger'dir.
func Setup(opts Options) *slog.Logger {
	level := parseLevel(opts.Level)
	opts.Format = strings.ToLower(opts.Format)

	var handler slog.Handler
	hopts := &slog.HandlerOptions{Level: level}
	out := io.Writer(os.Stdout)
	if opts.Out != nil {
		out = opts.Out
	}
	if opts.Format == "json" {
		handler = slog.NewJSONHandler(out, hopts)
	} else {
		handler = slog.NewTextHandler(out, hopts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// stdlib log (log.Printf) cagrilari ayni handler'dan aksin:
	// tum mevcut paket loglari tek formata dusmus olur.
	hw := slog.NewLogLogger(handler, slog.LevelInfo).Writer()
	log.SetFlags(0)
	log.SetOutput(hw)

	return logger
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
