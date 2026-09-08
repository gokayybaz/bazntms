package main

// devicegen.go — S21.2: hub'a N adet vendor=mock cihaz ekler ve hub'ın kendi
// devpoll zamanlayıcısının o filoyu yoklamasını izler. Amaç: 1.000 cihazlık
// poll döngüsünün eşzamanlılık + zamanlama yolunu (internal/devpoll) hedef
// ölçekte sürmek. Hub `-mock-devices` ile başlatılmalı.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type deviceGenConfig struct {
	hub         string
	user        string
	password    string
	devices     int
	pollSeconds int
}

func runDeviceGen(ctx context.Context, cfg deviceGenConfig, st *stats) {
	client := &http.Client{Timeout: 15 * time.Second}

	token, err := loginHub(ctx, client, cfg.hub, cfg.user, cfg.password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hub login başarısız: %v\n", err)
		return
	}

	// cihazları paralel ekle — hepsi vendor=mock. Eşzamanlılık düşük tutulur:
	// SQLite (dev) yazıcı kilidi çok paralel POST'u SQLITE_BUSY ile reddeder;
	// başarısız ekleme tek kez geri çekilmeyle yeniden denenir.
	var created, failed atomic.Int64
	var wg sync.WaitGroup
	work := make(chan int)
	workers := 6
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				if ctx.Err() != nil {
					return
				}
				err := addMockDevice(ctx, client, cfg.hub, token, i, cfg.pollSeconds)
				if err != nil {
					time.Sleep(150 * time.Millisecond)
					err = addMockDevice(ctx, client, cfg.hub, token, i, cfg.pollSeconds)
				}
				if err == nil {
					created.Add(1)
				} else {
					failed.Add(1)
				}
			}
		}()
	}
	for i := 0; i < cfg.devices; i++ {
		select {
		case <-ctx.Done():
			close(work)
			wg.Wait()
			return
		case work <- i:
		}
	}
	close(work)
	wg.Wait()
	fmt.Printf("device: %d cihaz eklendi (%d hata) — hub devpoll'u izleniyor\n", created.Load(), failed.Load())

	// devpoll ilerlemesini /metrics'ten izle
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	var lastCount float64
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cnt, sum, inflight, ok := scrapeDevpoll(ctx, client, cfg.hub)
			if !ok {
				continue
			}
			var avg float64
			if d := cnt - lastCount; d > 0 {
				avg = (sum - devpollLastSum) / d
			}
			lastCount, devpollLastSum = cnt, sum
			fmt.Printf("device: poll döngüsü toplam=%.0f ort_son=%.2fs inflight=%.0f\n", cnt, avg, inflight)
		}
	}
}

var devpollLastSum float64

func loginHub(ctx context.Context, c *http.Client, hub, user, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hub+"/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login HTTP %d", resp.StatusCode)
	}
	var out struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if !out.OK || out.Token == "" {
		return "", fmt.Errorf("login reddedildi")
	}
	return out.Token, nil
}

func addMockDevice(ctx context.Context, c *http.Client, hub, token string, idx, pollSeconds int) error {
	body, _ := json.Marshal(map[string]any{
		"name":         fmt.Sprintf("mock-%04d", idx),
		"host":         fmt.Sprintf("10.99.%d.%d", idx/250, idx%250+1),
		"kind":         "switch",
		"vendor":       "mock",
		"poll_seconds": pollSeconds,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hub+"/api/v1/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("device add HTTP %d", resp.StatusCode)
	}
	return nil
}

// scrapeDevpoll, /metrics'ten devpoll histogram sayacı+toplamı ve inflight
// gauge'unu çeker (basit satır ayrıştırma — prometheus client dep'i yok).
func scrapeDevpoll(ctx context.Context, c *http.Client, hub string) (count, sum, inflight float64, ok bool) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, hub+"/metrics", nil)
	resp, err := c.Do(req)
	if err != nil {
		return 0, 0, 0, false
	}
	defer resp.Body.Close()

	tail := func(line string) float64 {
		if i := strings.LastIndexByte(line, ' '); i >= 0 {
			v, _ := strconv.ParseFloat(strings.TrimSpace(line[i+1:]), 64)
			return v
		}
		return 0
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "bazntms_devpoll_cycle_duration_seconds_count "):
			count, ok = tail(line), true
		case strings.HasPrefix(line, "bazntms_devpoll_cycle_duration_seconds_sum "):
			sum = tail(line)
		case strings.HasPrefix(line, "bazntms_devpoll_inflight "):
			inflight = tail(line)
		}
	}
	return count, sum, inflight, ok
}
