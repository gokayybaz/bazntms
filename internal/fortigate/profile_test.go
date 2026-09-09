package fortigate

import "testing"

func TestProfileForVersion(t *testing.T) {
	cases := map[string]string{
		"v7.2.11": "7.2",
		"7.2.0":   "7.2",
		"v7.4.3":  "7.4",
		"v6.4.9":  "default", // bilinmeyen
		"":        "default",
		"garbage": "default",
	}
	for in, want := range cases {
		if got := ProfileForVersion(in).ID; got != want {
			t.Errorf("ProfileForVersion(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

func TestResolveProfile(t *testing.T) {
	// pin kazanır
	if got := ResolveProfile("7.4", "v7.2.11").ID; got != "7.4" {
		t.Errorf("pin: %q", got)
	}
	// auto / boş → sürümden
	if got := ResolveProfile("auto", "v7.2.11").ID; got != "7.2" {
		t.Errorf("auto: %q", got)
	}
	if got := ResolveProfile("", "v7.6.0").ID; got != "7.6" {
		t.Errorf("boş: %q", got)
	}
	// geçersiz pin → sürümden (yok sayılır)
	if got := ResolveProfile("9.9", "v7.2.0").ID; got != "7.2" {
		t.Errorf("geçersiz pin: %q", got)
	}
	// hiçbiri → default
	if got := ResolveProfile("", "").ID; got != "default" {
		t.Errorf("default: %q", got)
	}
}

func TestValidProfileID(t *testing.T) {
	for _, ok := range []string{"", "auto", "7.0", "7.2", "7.4", "7.6", "default"} {
		if !ValidProfileID(ok) {
			t.Errorf("%q geçerli olmalı", ok)
		}
	}
	for _, bad := range []string{"7.1", "8.0", "xxx"} {
		if ValidProfileID(bad) {
			t.Errorf("%q geçersiz olmalı", bad)
		}
	}
}
