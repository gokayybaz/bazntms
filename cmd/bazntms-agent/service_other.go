//go:build !windows

// Windows disi platformlarda servis modu ve kayit defteri override'u yok;
// daemon systemd/launchd gibi supervisor'lar altinda surec olarak calisir
// ve sinyalle kapanir.

package main

func serviceMode() bool { return false }

func runService(_ func(stop chan struct{}) error) error {
	return nil
}

func platformValue(string) string { return "" }
