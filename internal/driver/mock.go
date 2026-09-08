package driver

// MockDriver, ölçek testi (S21.2) için sentetik bir cihaz sürücüsüdür: ağ I/O
// yapmaz, deterministik ve monoton artan arayüz sayaçları üretir. Yalnızca
// hub `-mock-devices` bayrağıyla açıldığında `vendor=mock` cihazlar eklenebilir;
// devpoll zamanlayıcı + eşzamanlılık yolu bu sürücü üzerinden gerçek şekilde
// çalışır (SNMP wire'ı zaten snmp_test.go'da birim testli).

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

const mockIfaces = 4

type mockCounters struct {
	mu       sync.Mutex
	rx, tx   [mockIfaces]uint64
	rxp, txp [mockIfaces]uint64
}

// mockState, cihaz başına kümülatif sayaçları tutar (gerçek SNMP sayaçları da
// kümülatiftir; devpoll ardışık örnek farkından hız hesaplar).
var mockState sync.Map // deviceID(int64) -> *mockCounters

// MockDriver, deterministik snapshot üreten test sürücüsü.
type MockDriver struct{}

// Poll, simüle edilmiş bir SNMP round-trip gecikmesi sonrası monoton artmış
// sayaçlarla bir Snapshot döndürür. ctx iptal edilirse hemen döner.
func (m *MockDriver) Poll(ctx context.Context, d store.Device, _ *vault.Vault) (Snapshot, error) {
	// gerçek SNMP poll'u ~10-500 ms sürer; zamanlayıcı yükünü anlamlı kılmak
	// için küçük, cihaza göre deterministik bir gecikme.
	delay := 10*time.Millisecond + time.Duration(d.ID%25)*time.Millisecond
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}

	cAny, _ := mockState.LoadOrStore(d.ID, &mockCounters{})
	c := cAny.(*mockCounters)
	c.mu.Lock()
	defer c.mu.Unlock()

	ifaces := make([]store.DeviceIface, mockIfaces)
	for i := 0; i < mockIfaces; i++ {
		c.rx[i] += 1_000_000 + uint64(i)*250_000
		c.tx[i] += 500_000 + uint64(i)*120_000
		c.rxp[i] += 1_000 + uint64(i)*250
		c.txp[i] += 800 + uint64(i)*100
		ifaces[i] = store.DeviceIface{
			IfIndex:    int64(i + 1),
			Name:       fmt.Sprintf("eth%d", i),
			Speed:      1_000_000_000,
			OperStatus: 1,
			RxBytes:    c.rx[i],
			TxBytes:    c.tx[i],
		}
	}

	return Snapshot{
		SysName:  d.Name,
		SysDescr: "bazntms-loadgen mock device (S21.2)",
		Ifaces:   ifaces,
	}, nil
}
