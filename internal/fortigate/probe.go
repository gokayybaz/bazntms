package fortigate

// Bağlantı sınama (probe) — Faz 27.
//
// Cihaz eklerken/sonrasında elle tetiklenir. FortiGate'in şema vermediği
// gerçeğiyle: her veri ucuna gerçek istek atar, dönen yanıta bakar
// (bağlandı mı? HTTP kodu? bizim parser anladı mı? kaç kayıt?), yanıt
// zarfından sürümü/VDOM modunu çıkarır ve kullanıcıya rapor döndürür.

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

// EndpointResult, tek bir ucun sınama sonucu.
type EndpointResult struct {
	Endpoint   string `json:"endpoint"`             // "monitor/system/interface"
	Label      string `json:"label"`                // "Arayüzler"
	HTTPStatus int    `json:"http_status"`          // 0 = ağ hatası
	OK         bool   `json:"ok"`                   // 2xx + parser en az bir kayıt anladı
	Count      int    `json:"count"`                // ayrıştırılan kayıt sayısı
	Shape      string `json:"shape"`                // "array" | "object" | "map-of-arrays" | ""
	Note       string `json:"note,omitempty"`       // hata / "veri yok" / "403 — token kapsamı"
	RawSample  string `json:"raw_sample,omitempty"` // results'ın ilk ~2KB'ı (UI "ham yanıt")
}

// Report, tam sınama raporu.
type Report struct {
	OK        bool              `json:"ok"` // system/status ulaşıldı mı
	Version   string            `json:"version"`
	Build     int               `json:"build"`
	Serial    string            `json:"serial"`
	Hostname  string            `json:"hostname"`
	VDOMMode  string            `json:"vdom_mode"` // "single" | "multi" | "unknown"
	VDOMs     []string          `json:"vdoms"`
	ProfileID string            `json:"profile_id"`
	Error     string            `json:"error,omitempty"` // system/status başarısızsa
	Endpoints []EndpointResult  `json:"endpoints"`
	Caps      map[string]string `json:"caps"` // kısa ad → "ok"|"empty"|"denied"|"error"
}

const rawSampleLimit = 2048

// probeEndpoint, tek uç tanımı.
type probeEndpoint struct {
	key   string // caps anahtarı: "interface", "resource", ...
	path  string
	label string
	query url.Values
}

// Probe, verilen kimlikle FortiGate'i sınar. Ağ/timeout hataları rapora yazılır,
// panik olmaz.
func Probe(ctx context.Context, opts Options) Report {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = -1 // sınamada retry yok — hızlı sonuç
	}
	if opts.Timeout == 0 {
		opts.Timeout = 8 * time.Second // ulaşılamayan cihazda UI'yı 30 sn bekletme
	}
	c := New(opts)
	rep := Report{Caps: map[string]string{}, VDOMs: []string{}, Endpoints: []EndpointResult{}}

	// 1. system/status — sürüm + kimlik
	st, err := c.SystemStatus(ctx)
	if err != nil {
		rep.Error = err.Error()
		rep.ProfileID = c.ProfileID()
		return rep
	}
	rep.OK = true
	rep.Version, rep.Build, rep.Serial, rep.Hostname = st.Version, st.Build, st.Serial, st.Hostname

	// 2. VDOM modu
	if vdoms, err := c.VDOMs(ctx); err == nil {
		rep.VDOMs = vdoms
		if len(vdoms) > 1 {
			rep.VDOMMode = "multi"
		} else {
			rep.VDOMMode = "single"
		}
	} else {
		rep.VDOMMode = "unknown"
	}
	rep.ProfileID = c.ProfileID()

	vdom := opts.VDOM

	// 3. veri uçları
	eps := []probeEndpoint{
		{"resource", "/api/v2/monitor/system/resource/usage", "Kaynak kullanımı", vdomQuery(nil, vdom)},
		{"interface", "/api/v2/monitor/system/interface", "Arayüzler", vdomQuery(url.Values{"include_vlan": {"true"}}, vdom)},
		{"vpn_ipsec", "/api/v2/monitor/vpn/ipsec", "IPsec tünelleri", vdomQuery(nil, vdom)},
		{"vpn_ssl", "/api/v2/monitor/vpn/ssl", "SSL-VPN oturumları", vdomQuery(nil, vdom)},
		{"sdwan", c.sdwanPath(), "SD-WAN health-check", vdomQuery(nil, vdom)},
		{"policy_mon", "/api/v2/monitor/firewall/policy", "Politika sayaçları", vdomQuery(nil, vdom)},
		{"policy_cmdb", "/api/v2/cmdb/firewall/policy", "Politika tanımları", vdomQuery(url.Values{"fields": {"policyid,name,action"}, "count": {"20"}}, vdom)},
	}
	for _, ep := range eps {
		res := probeOne(ctx, c, ep)
		rep.Endpoints = append(rep.Endpoints, res)
		rep.Caps[ep.key] = capOf(res)
	}
	return rep
}

func probeOne(ctx context.Context, c *Client, ep probeEndpoint) EndpointResult {
	res := EndpointResult{Endpoint: strings.TrimPrefix(ep.path, "/api/v2/"), Label: ep.label}
	raw, err := c.getRaw(ctx, ep.path, ep.query)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			res.HTTPStatus = apiErr.Status
			if apiErr.Status == 401 || apiErr.Status == 403 {
				res.Note = "yetki reddi — API admin profili / trusthost"
			} else {
				res.Note = err.Error()
			}
		} else {
			res.Note = err.Error()
		}
		return res
	}
	res.HTTPStatus = 200
	res.Shape = shapeOf(raw)
	if s := strings.TrimSpace(string(raw)); len(s) > 0 {
		if len(s) > rawSampleLimit {
			s = s[:rawSampleLimit] + "…"
		}
		res.RawSample = s
	}
	if ep.key == "resource" {
		n := len(parseResource(raw))
		res.Count = n
		res.OK = n > 0
	} else {
		items := resultItems(unwrapKey(raw, "users"))
		res.Count = len(items)
		res.OK = len(items) > 0
	}
	if res.HTTPStatus == 200 && !res.OK {
		res.Note = "bağlandı, veri yok"
	}
	return res
}

func shapeOf(raw json.RawMessage) string {
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return ""
	}
	switch t[0] {
	case '[':
		return "array"
	case '{':
		// map-of-arrays mı? (resource/usage biçimi)
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil && len(obj) > 0 {
			allArr := true
			for _, v := range obj {
				if s := strings.TrimSpace(string(v)); s == "" || s[0] != '[' {
					allArr = false
					break
				}
			}
			if allArr {
				return "map-of-arrays"
			}
		}
		return "object"
	}
	return ""
}

func capOf(r EndpointResult) string {
	switch {
	case r.HTTPStatus == 401 || r.HTTPStatus == 403:
		return "denied"
	case r.HTTPStatus == 0 || r.HTTPStatus >= 400:
		return "error"
	case r.OK:
		return "ok"
	default:
		return "empty"
	}
}
