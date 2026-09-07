package agent

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// stubAttrSource, seçici testlerinde gerçek pcap/eBPF/ETW kurmadan AttrSource
// arayüzünü karşılar.
type stubAttrSource struct{ method string }

func (s stubAttrSource) Deltas() []telemetry.ProcessTrafficSample { return nil }
func (s stubAttrSource) L7Deltas() []telemetry.L7Sample           { return nil }
func (s stubAttrSource) DNSDeltas() []telemetry.DNSSample         { return nil }
func (s stubAttrSource) Stop()                                    {}
func (s stubAttrSource) Method() string                           { return s.method }

func TestAttrPlan(t *testing.T) {
	cases := []struct {
		name   string
		method string
		caps   attrCaps
		want   []string
	}{
		{"auto linux eBPF+pcap", "", attrCaps{ebpf: true, pcap: true}, []string{"ebpf", "pcap"}},
		{"auto linux yalniz pcap", "auto", attrCaps{pcap: true}, []string{"pcap"}},
		{"auto windows ETW+pcap", "", attrCaps{etw: true, pcap: true}, []string{"etw", "pcap"}},
		{"auto darwin", "", attrCaps{pcap: true}, []string{"pcap"}},
		{"auto hicbir yetenek yok", "auto", attrCaps{}, nil},
		{"zorlanmis ebpf yetenekleri yok sayar", "ebpf", attrCaps{pcap: true}, []string{"ebpf"}},
		{"zorlanmis pcap", "pcap", attrCaps{ebpf: true, pcap: true}, []string{"pcap"}},
		{"off", "off", attrCaps{ebpf: true, pcap: true}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := attrPlan(c.method, c.caps)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("attrPlan(%q, %+v) = %v; beklenen %v", c.method, c.caps, got, c.want)
			}
		})
	}
}

func TestNewAttrSourceFallbackOrder(t *testing.T) {
	orig := buildAttrSource
	t.Cleanup(func() { buildAttrSource = orig })

	var calls []string
	buildAttrSource = func(method string, _ AttrConfig) (AttrSource, error) {
		calls = append(calls, method)
		if method == "pcap" {
			return stubAttrSource{method: "pcap"}, nil
		}
		return nil, errors.New(method + " kurulamadi")
	}

	// auto + eBPF yeteneği var ama kurulum başarısız → pcap'e düşer
	src, err := newAttrSource(AttrConfig{Method: "auto"}, attrCaps{ebpf: true, pcap: true})
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if src.Method() != "pcap" {
		t.Fatalf("pcap bekleniyordu, %q döndü", src.Method())
	}
	if !reflect.DeepEqual(calls, []string{"ebpf", "pcap"}) {
		t.Fatalf("deneme sırası: %v", calls)
	}
}

func TestNewAttrSourceForcedNoFallback(t *testing.T) {
	orig := buildAttrSource
	t.Cleanup(func() { buildAttrSource = orig })
	buildAttrSource = func(method string, _ AttrConfig) (AttrSource, error) {
		if method == "pcap" {
			return stubAttrSource{method: "pcap"}, nil
		}
		return nil, errors.New(method + " yok")
	}

	// zorlanmış ebpf başarısız → pcap kullanılabilir olsa bile düşme yok
	_, err := newAttrSource(AttrConfig{Method: "ebpf"}, attrCaps{ebpf: true, pcap: true})
	if err == nil {
		t.Fatal("zorlanmış yöntem başarısızlığı hata döndürmeliydi")
	}
}

func TestNewAttrSourceOff(t *testing.T) {
	_, err := newAttrSource(AttrConfig{Method: "off"}, attrCaps{pcap: true})
	if err == nil {
		t.Fatal("method=off hata (kaynak yok) döndürmeliydi")
	}
}

func TestNewAttrSourceNoBackend(t *testing.T) {
	_, err := newAttrSource(AttrConfig{Method: "auto"}, attrCaps{})
	if err == nil {
		t.Fatal("hiçbir arka uç kullanılamadığında hata beklenirdi")
	}
}
