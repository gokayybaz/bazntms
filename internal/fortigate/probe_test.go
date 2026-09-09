package fortigate

import (
	"context"
	"net/http"
	"testing"
)

func TestProbeMixedResults(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/monitor/system/status":
			w.Write([]byte(env72(`{"hostname":"fgt-probe","serial":"FGT123","version":"v7.2.11","build":1639}`)))
		case "/api/v2/cmdb/system/vdom":
			w.Write([]byte(env72(`[{"name":"root"},{"name":"guest"}]`)))
		case "/api/v2/monitor/system/interface":
			w.Write([]byte(env72(`{"port1":{"name":"port1","link":true}}`)))
		case "/api/v2/monitor/system/resource/usage":
			w.Write([]byte(env72(`{"cpu":[{"current":5}]}`)))
		case "/api/v2/monitor/vpn/ipsec":
			w.Write([]byte(env72(`[]`))) // bağlandı, veri yok
		case "/api/v2/monitor/firewall/policy":
			w.WriteHeader(http.StatusForbidden) // yetki reddi
		default:
			w.Write([]byte(env72(`[]`)))
		}
	}
	c, url := newMockServer(t, handler)
	_ = c

	rep := Probe(context.Background(), Options{BaseURL: url, Token: "t", VerifyTLS: false, MinInterval: 0})
	if !rep.OK || rep.Version != "v7.2.11" || rep.Hostname != "fgt-probe" {
		t.Fatalf("rapor başlığı: %+v", rep)
	}
	if rep.VDOMMode != "multi" || len(rep.VDOMs) != 2 {
		t.Fatalf("vdom modu: %+v", rep)
	}
	if rep.ProfileID != "7.2" {
		t.Fatalf("profil: %s", rep.ProfileID)
	}
	if rep.Caps["interface"] != "ok" {
		t.Errorf("interface caps: %+v", rep.Caps)
	}
	if rep.Caps["vpn_ipsec"] != "empty" {
		t.Errorf("boş uç 'empty' olmalı: %+v", rep.Caps)
	}
	if rep.Caps["policy_mon"] != "denied" {
		t.Errorf("403 uç 'denied' olmalı: %+v", rep.Caps)
	}
	// ham örnek dolmalı (UI "ham yanıt")
	var ifRes *EndpointResult
	for i := range rep.Endpoints {
		if rep.Endpoints[i].Endpoint == "monitor/system/interface" {
			ifRes = &rep.Endpoints[i]
		}
	}
	if ifRes == nil || ifRes.RawSample == "" || ifRes.Shape != "object" {
		t.Fatalf("interface uç sonucu: %+v", ifRes)
	}
}

func TestProbeUnreachable(t *testing.T) {
	rep := Probe(context.Background(), Options{BaseURL: "https://127.0.0.1:1", Token: "t", MinInterval: 0, MaxRetries: 1})
	if rep.OK || rep.Error == "" {
		t.Fatalf("ulaşılamayan cihaz OK dönmemeli: %+v", rep)
	}
}
