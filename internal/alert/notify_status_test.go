package alert

// D3 / S12.8: bildirim kanalı durumu + Test() + fail hook.

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

// TestNotifierTestChannelStatus: Test() etkin kanallara sınama gönderir; her
// kanalın sonucu Status()'ta görünür, hata olursa fail hook tetiklenir.
func TestNotifierTestChannelStatus(t *testing.T) {
	var okHits, failHits int32
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&okHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&failHits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	n := NewNotifier()
	var failCh atomic.Value // string
	n.SetFailHook(func(channel string) { failCh.Store(channel) })

	res := n.Test(Notifiers{SlackURL: ok.URL, DiscordURL: bad.URL})

	if okHits == 0 || failHits == 0 {
		t.Fatalf("her iki uç da çağrılmalıydı: ok=%d bad=%d", okHits, failHits)
	}
	if s := res[ChSlack]; !s.OK || s.LastAttempt == 0 {
		t.Fatalf("slack durumu OK olmalı: %+v", s)
	}
	if s := res[ChDiscord]; s.OK || s.Error == "" || s.LastAttempt == 0 {
		t.Fatalf("discord durumu hatalı olmalı: %+v", s)
	}
	if failCh.Load() != ChDiscord {
		t.Fatalf("fail hook 'discord' ile çağrılmalıydı, gelen: %v", failCh.Load())
	}
	if _, ok := n.Status()[ChTelegram]; ok {
		t.Fatal("etkin olmayan kanal durum haritasında olmamalı")
	}
}

// TestNotifierDispatchRecordsStatus, dispatch (Deliver'ın senkron çekirdeği)
// yolunun da durum kaydettiğini doğrular.
func TestNotifierDispatchRecordsStatus(t *testing.T) {
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	n := NewNotifier()
	n.dispatch(Notifiers{GenericURL: srv.URL}, store.AlertEvent{Kind: "test", Message: "x"})
	if s := n.Status()[ChGeneric]; !s.OK {
		t.Fatalf("generic durumu OK olmalı: %+v", s)
	}
	<-done
}
