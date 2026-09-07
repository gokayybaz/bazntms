//go:build linux

package agent

import "testing"

func TestLinuxEBPFEnvOK(t *testing.T) {
	cases := []struct {
		name string
		env  linuxCapEnv
		want bool
	}{
		{
			"modern kernel + BTF + root",
			linuxCapEnv{osRelease: "6.8.0-51-generic", btfPresent: true, euid: 0},
			true,
		},
		{
			"modern kernel + BTF + CAP_BPF (root değil)",
			linuxCapEnv{osRelease: "5.15.0", btfPresent: true, euid: 1000, procStatus: "CapEff:\t0000008000000000\n"},
			true, // bit 39 = CAP_BPF
		},
		{
			"kernel 5.4 çok eski",
			linuxCapEnv{osRelease: "5.4.0-91-generic", btfPresent: true, euid: 0},
			false,
		},
		{
			"kernel 4.19",
			linuxCapEnv{osRelease: "4.19.0", btfPresent: true, euid: 0},
			false,
		},
		{
			"BTF yok",
			linuxCapEnv{osRelease: "6.8.0", btfPresent: false, euid: 0},
			false,
		},
		{
			"yetki yok",
			linuxCapEnv{osRelease: "6.8.0", btfPresent: true, euid: 1000, procStatus: "CapEff:\t0000000000000000\n"},
			false,
		},
		{
			"kernel sürümü okunamıyor",
			linuxCapEnv{osRelease: "", btfPresent: true, euid: 0},
			false,
		},
		{
			"tam sınır 5.8",
			linuxCapEnv{osRelease: "5.8.0-generic", btfPresent: true, euid: 0},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason := linuxEBPFEnvOK(c.env)
			if ok != c.want {
				t.Fatalf("linuxEBPFEnvOK = %v (%q); beklenen %v", ok, reason, c.want)
			}
			if !ok && reason == "" {
				t.Fatal("başarısızlıkta neden boş olmamalı")
			}
		})
	}
}

func TestLinuxHasBPFCap(t *testing.T) {
	if !linuxHasBPFCap(0, "") {
		t.Fatal("root her zaman true")
	}
	if linuxHasBPFCap(1000, "CapEff:\t0000000000000000\n") {
		t.Fatal("boş yetki → false")
	}
	if !linuxHasBPFCap(1000, "CapEff:\t0000000000200000\n") { // bit 21 = CAP_SYS_ADMIN
		t.Fatal("CAP_SYS_ADMIN → true")
	}
	if linuxHasBPFCap(1000, "Seccomp:\t0\n") {
		t.Fatal("CapEff satırı yok → false")
	}
}
