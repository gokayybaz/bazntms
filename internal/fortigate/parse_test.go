package fortigate

import (
	"encoding/json"
	"testing"
)

func TestResultItemsArray(t *testing.T) {
	items := resultItems(json.RawMessage(`[{"name":"a"},{"name":"b"}]`))
	if len(items) != 2 || items[0].str("name") != "a" || items[1].str("name") != "b" {
		t.Fatalf("dizi: %+v", items)
	}
}

func TestResultItemsObjectKeyed(t *testing.T) {
	// monitor/system/interface biçimi: ada-göre anahtarlı
	items := resultItems(json.RawMessage(`{"port2":{"speed":1000},"port1":{"speed":100}}`))
	if len(items) != 2 {
		t.Fatalf("beklenen 2, alınan %d", len(items))
	}
	// deterministik sıra (anahtara göre)
	if items[0].str("_key") != "port1" || items[1].str("_key") != "port2" {
		t.Fatalf("_key sırası: %s %s", items[0].str("_key"), items[1].str("_key"))
	}
	if items[0].u64("speed") != 100 {
		t.Fatalf("port1 speed: %d", items[0].u64("speed"))
	}
}

func TestResultItemsSingleRecord(t *testing.T) {
	// monitor/system/status biçimi: tek kayıt, scalar alanlar
	items := resultItems(json.RawMessage(`{"hostname":"fgt","uptime":3600,"nested":{"x":1}}`))
	if len(items) != 1 || items[0].str("hostname") != "fgt" || items[0].u64("uptime") != 3600 {
		t.Fatalf("tek kayıt: %+v", items)
	}
}

func TestResultItemsEmpty(t *testing.T) {
	for _, in := range []string{``, `null`, `[]`, `{}`, `"x"`, `123`} {
		if got := resultItems(json.RawMessage(in)); len(got) != 0 {
			t.Fatalf("%q → %+v (boş bekleniyordu)", in, got)
		}
	}
}

func TestGettersCoercion(t *testing.T) {
	m := item{
		"s":      json.RawMessage(`"hi"`),
		"nAsStr": json.RawMessage(`"1500"`),
		"n":      json.RawMessage(`42.7`),
		"bTrue":  json.RawMessage(`true`),
		"bStr":   json.RawMessage(`"up"`),
		"bNum":   json.RawMessage(`1`),
	}
	if m.str("miss", "s") != "hi" {
		t.Fatal("str fallback anahtar")
	}
	if m.u64("nAsStr") != 1500 {
		t.Fatalf("u64 string: %d", m.u64("nAsStr"))
	}
	if m.f64("n") != 42.7 {
		t.Fatalf("f64: %v", m.f64("n"))
	}
	if m.u64("n") != 42 {
		t.Fatalf("u64 float trunc: %d", m.u64("n"))
	}
	if v, ok := m.boolOr("bTrue"); !ok || !v {
		t.Fatal("boolOr bool")
	}
	if v, ok := m.boolOr("bStr"); !ok || !v {
		t.Fatal("boolOr string 'up'")
	}
	if v, ok := m.boolOr("bNum"); !ok || !v {
		t.Fatal("boolOr sayı")
	}
	if _, ok := m.boolOr("yok"); ok {
		t.Fatal("boolOr eksik anahtar found=true döndü")
	}
	if m.str("yok") != "" || m.u64("yok") != 0 || m.f64("yok") != 0 {
		t.Fatal("eksik anahtar sıfır dönmeli")
	}
}

func TestLinkBoolVsObject(t *testing.T) {
	// link: bool (7.2)
	up, ok := item{"link": json.RawMessage(`true`)}.boolOr("link")
	if !ok || !up {
		t.Fatal("link bool")
	}
	// link: {status:"up"} (eski)
	m := item{"link": json.RawMessage(`{"status":"up","speed":"1000FDX"}`)}
	if _, ok := m.boolOr("link"); ok {
		t.Fatal("link obje boolOr'da çözülmemeli (sub'a düşmeli)")
	}
	if sub := m.sub("link"); sub == nil || sub.str("speed") != "1000FDX" {
		t.Fatalf("sub link: %+v", m.sub("link"))
	}
}

func TestUnwrapKey(t *testing.T) {
	got := unwrapKey(json.RawMessage(`{"users":[{"user_name":"a"}]}`), "users")
	items := resultItems(got)
	if len(items) != 1 || items[0].str("user_name") != "a" {
		t.Fatalf("unwrap users: %+v", items)
	}
	// tek alan değilse dokunma
	same := json.RawMessage(`{"a":1,"b":2}`)
	if string(unwrapKey(same, "a")) != string(same) {
		t.Fatal("çok alanlı obje unwrap edilmemeli")
	}
}

func TestLinkSpeedBps(t *testing.T) {
	cases := map[string]uint64{"1000FDX": 1_000_000_000, "100full": 100_000_000, "10": 10_000_000, "": 0, "auto": 0}
	for in, want := range cases {
		if got := linkSpeedBps(in); got != want {
			t.Fatalf("linkSpeedBps(%q) = %d, beklenen %d", in, got, want)
		}
	}
}
