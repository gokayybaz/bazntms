//go:build windows

// Windows servis entegrasyonu (Faz 7.3): MSI ile kurulan servis, SCM tarafindan
// baslatildiginda exe'nin 30 sn icinde StartServiceCtrlDispatcher'a baglanmasi
// gerekir — aksi halde hata 1053 (MSI kurulumunda 1920 olarak gorunur).
// svc.Run bu dispatcher'i kurar; Stop/Shutdown istekleri agent'in stop
// kanalini kapatarak graceful shutdown tetikler.

package main

import (
	"sync"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
)

const (
	serviceName    = "bazntms-agent"
	registrySubKey = `SOFTWARE\bazNTMS\Agent`
)

// serviceMode, surec SCM tarafindan baslatildiysa true dondurur.
func serviceMode() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

// platformValue, MSI property'lerinden yazilan kayit defteri degerini okur
// (HUBURL/ENROLLTOKEN/SITE → hub_url/enroll_token/site). Env degiskenleri
// SCM tarafindan reboot'a kadar tazelenmedigi icin servis konfigurasyonu
// icin kayit defteri kullanilir; yoksa bos doner (config.yml'ye duser).
func platformValue(name string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, registrySubKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return v
}

// runService, agent'i servis olarak kaydeder ve SCM isteklerini isler.
func runService(run func(stop chan struct{}) error) error {
	return svc.Run(serviceName, &svcHandler{run: run})
}

type svcHandler struct {
	run      func(stop chan struct{}) error
	stopOnce sync.Once
}

func (h *svcHandler) stopChan(stop chan struct{}) func() {
	return func() { h.stopOnce.Do(func() { close(stop) }) }
}

func (h *svcHandler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	stop := make(chan struct{})
	done := make(chan struct{})
	var runErr error
	stopFn := h.stopChan(stop)
	go func() {
		defer close(done)
		runErr = h.run(stop)
	}()

	status <- svc.Status{State: svc.Running, Accepts: accepts}
	for {
		select {
		case <-done:
			// run kendi kendine sonlandi (config hatasi, guncelleme cikisi...)
			status <- svc.Status{State: svc.Stopped}
			if runErr != nil {
				return false, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				stopFn()
				<-done
				status <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}
