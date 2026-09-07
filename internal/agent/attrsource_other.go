//go:build !linux && !windows

package agent

import (
	"fmt"
	"runtime"
)

// platformAttrCaps, Linux/Windows dışı platformlar (macOS, *BSD) için: yalnız
// pcap. Endpoint Security / eBPF karşılıkları kapsam dışı (bkz. Faz 20 planı).
func platformAttrCaps() attrCaps {
	return attrCaps{pcap: true}
}

func platformBuildAttrSource(method string, cfg AttrConfig) (AttrSource, error) {
	if method == "pcap" {
		src, err := newPcapAttrSource(cfg.Iface)
		if err != nil {
			return nil, err
		}
		return src, nil
	}
	return nil, fmt.Errorf("%s'te yalnızca pcap atıf arka ucu var (istenen: %q)", runtime.GOOS, method)
}
