package driver

import (
	"context"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestMockDriverDeterministicMonotonic(t *testing.T) {
	mockState.Delete(int64(42))
	m := &MockDriver{}
	d := store.Device{ID: 42, Name: "mock-0042", Vendor: "mock"}

	s1, err := m.Poll(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("poll 1: %v", err)
	}
	if len(s1.Ifaces) != mockIfaces {
		t.Fatalf("arayüz sayısı = %d, beklenen %d", len(s1.Ifaces), mockIfaces)
	}
	if s1.SysName != "mock-0042" {
		t.Errorf("SysName = %q", s1.SysName)
	}

	s2, err := m.Poll(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("poll 2: %v", err)
	}
	for i := range s2.Ifaces {
		if s2.Ifaces[i].RxBytes <= s1.Ifaces[i].RxBytes || s2.Ifaces[i].TxBytes <= s1.Ifaces[i].TxBytes {
			t.Errorf("arayüz %d sayaçları artmadı: %d→%d rx", i, s1.Ifaces[i].RxBytes, s2.Ifaces[i].RxBytes)
		}
		if s2.Ifaces[i].OperStatus != 1 || s2.Ifaces[i].Speed != 1_000_000_000 {
			t.Errorf("arayüz %d oper/speed hatalı", i)
		}
	}
}

func TestMockDriverContextCancel(t *testing.T) {
	m := &MockDriver{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Poll(ctx, store.Device{ID: 1, Vendor: "mock"}, nil); err == nil {
		t.Fatal("iptal edilmiş ctx ile hata beklenirdi")
	}
}

func TestForReturnsMockDriver(t *testing.T) {
	if _, ok := For(store.Device{Vendor: "mock"}).(*MockDriver); !ok {
		t.Fatal("For(vendor=mock) MockDriver döndürmeliydi")
	}
	if _, ok := For(store.Device{Vendor: ""}).(*SNMPDriver); !ok {
		t.Fatal("For(vendor='') SNMPDriver döndürmeliydi")
	}
}

func TestMockDriverPollLatencyBounded(t *testing.T) {
	m := &MockDriver{}
	start := time.Now()
	if _, err := m.Poll(context.Background(), store.Device{ID: 24, Vendor: "mock"}, nil); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("mock poll çok yavaş: %v", d)
	}
}
