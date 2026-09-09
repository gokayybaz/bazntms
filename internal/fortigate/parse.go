package fortigate

// Toleranslı yanıt ayrıştırma çekirdeği (Faz 27).
//
// FortiOS'un `monitor/*` uçlarında canlı şema keşfi yoktur ve `results` alanının
// biçimi hem uçlar hem sürümler arasında değişir: bazen dizi, bazen ada-göre
// anahtarlı obje; alan adları (`user`/`user_name`), tipleri (`link` bool/obje)
// oynar. Bu dosya bu farkları tek bir yerde yutar — endpoint metotları yalnızca
// mantıksal alan adlarını sorar, biçimi düşünmez.

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

// item, tek bir kaydın ham alan haritası. `_key` yalnızca `results` obje-anahtarlı
// geldiğinde enjekte edilir (kaydın adı = harita anahtarı).
type item map[string]json.RawMessage

// asItem, ham JSON'u bir kayda çözer; obje değilse nil.
func asItem(raw json.RawMessage) item {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || t[0] != '{' {
		return nil
	}
	var m item
	if json.Unmarshal(t, &m) != nil {
		return nil
	}
	return m
}

// resultItems, bir `results` gövdesini kayıt listesine indirger:
//   - `[ {...}, {...} ]`            → öğeler
//   - `{ "port1": {...}, ... }`     → değerler; anahtar `_key` olarak enjekte
//   - `{ tek kayıt scalar alanlı }` → tek elemanlı liste
//
// Ayrıştırılamayan / boş gövde → nil.
func resultItems(raw json.RawMessage) []item {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return nil
	}
	switch t[0] {
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(t, &arr) != nil {
			return nil
		}
		out := make([]item, 0, len(arr))
		for _, e := range arr {
			if m := asItem(e); m != nil {
				out = append(out, m)
			}
		}
		return out
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(t, &obj) != nil || len(obj) == 0 {
			return nil
		}
		// Her değer obje/dizi ise → ada-göre anahtarlı harita. Aksi halde
		// (en az bir scalar alan) → gövdenin kendisi tek bir kayıt.
		keyed := true
		for _, v := range obj {
			vt := bytes.TrimSpace(v)
			if len(vt) == 0 || (vt[0] != '{' && vt[0] != '[') {
				keyed = false
				break
			}
		}
		if !keyed {
			if m := asItem(t); m != nil {
				return []item{m}
			}
			return nil
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministik sıra
		out := make([]item, 0, len(keys))
		for _, k := range keys {
			m := asItem(obj[k])
			if m == nil {
				continue
			}
			if _, ok := m["_key"]; !ok {
				m["_key"], _ = json.Marshal(k)
			}
			out = append(out, m)
		}
		return out
	}
	return nil
}

// unwrapKey, `{ "<key>": X }` (tek alanlı) gövdesini X'e indirger; aksi halde
// gövdeyi olduğu gibi döndürür. FortiOS'un `{"users":[...]}` gibi sarmaları için.
func unwrapKey(raw json.RawMessage, key string) json.RawMessage {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || t[0] != '{' {
		return raw
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(t, &obj) != nil || len(obj) != 1 {
		return raw
	}
	if v, ok := obj[key]; ok {
		return v
	}
	return raw
}

// --- toleranslı getter'lar: sırayla anahtar dener, tip çevirir, eksikte sıfır ---

func (m item) str(keys ...string) string {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if s != "" {
				return s
			}
			continue
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return strconv.FormatBool(b)
		}
	}
	return ""
}

func (m item) f64(keys ...string) float64 {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return f
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func (m item) u64(keys ...string) uint64 {
	f := m.f64(keys...)
	if f <= 0 {
		return 0
	}
	return uint64(f)
}

// boolOr, bir alanı bool'a çözer. found=false → hiçbir anahtar yok / çözülemedi
// (çağıran fallback deneyebilsin diye ayrı).
func (m item) boolOr(keys ...string) (val, found bool) {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return b, true
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "up", "true", "enable", "enabled", "1", "yes", "connected", "ready":
				return true, true
			case "down", "false", "disable", "disabled", "0", "no", "disconnected":
				return false, true
			}
			continue
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return f != 0, true
		}
	}
	return false, false
}

// sub, iç obje alanı (yoksa nil).
func (m item) sub(key string) item {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	return asItem(raw)
}

// arr, iç dizi alanını kayıt listesine çözer (yoksa nil).
func (m item) arr(key string) []item {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	var a []json.RawMessage
	if json.Unmarshal(raw, &a) != nil {
		return nil
	}
	out := make([]item, 0, len(a))
	for _, e := range a {
		if it := asItem(e); it != nil {
			out = append(out, it)
		}
	}
	return out
}

// stableIndex, FortiGate arayüzü için kararlı sayısal indeks üretir: sayısal bir
// id alanı varsa onu, yoksa ad'ın FNV hash'ini (poll'lar arası sabit), o da
// yoksa sıra numarasını kullanır. Store örnekleri if_index'e göre grupladığı
// için sıfır-çakışması olmamalı.
func stableIndex(it item, fallback int) int64 {
	if n := it.u64("id", "index", "ifindex", "if_index"); n > 0 {
		return int64(n)
	}
	name := it.str("_key", "name")
	if name == "" {
		return int64(fallback)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum32())
}

// linkSpeedBps, FortiOS link hız gösterimlerini bit/sn'ye çevirir:
// "1000FDX" / "1000full" → 1e9; sayısal Mbps ("1000") → 1e9; boş/0 → 0.
func linkSpeedBps(s string) uint64 {
	s = strings.TrimSpace(s)
	digits := strings.Builder{}
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
		} else {
			break
		}
	}
	mbps, err := strconv.ParseUint(digits.String(), 10, 64)
	if err != nil || mbps == 0 {
		return 0
	}
	return mbps * 1_000_000
}
