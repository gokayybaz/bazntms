package devpoll

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestIntervalOverride(t *testing.T) {
	p := &Poller{}
	d := store.Device{PollSeconds: 120}

	// override yok → per-device
	if got := p.interval(d); got != 120*time.Second {
		t.Fatalf("per-device bekleniyordu: %s", got)
	}

	// override → tüm cihazlar aynı
	p.SetInterval(30 * time.Second)
	if got := p.interval(d); got != 30*time.Second {
		t.Fatalf("override 30s bekleniyordu: %s", got)
	}
	if got := p.interval(store.Device{PollSeconds: 5}); got != 30*time.Second {
		t.Fatalf("override her cihaza uygulanmali: %s", got)
	}

	// 5 sn altı taban
	p.SetInterval(2 * time.Second)
	if got := p.interval(d); got != 5*time.Second {
		t.Fatalf("5s taban bekleniyordu: %s", got)
	}

	// 0 → override kapalı
	p.SetInterval(0)
	if got := p.interval(d); got != 120*time.Second {
		t.Fatalf("override kapaninca per-device: %s", got)
	}
}
