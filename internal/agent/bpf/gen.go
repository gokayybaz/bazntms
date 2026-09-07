// Package bpf, agent'ın eBPF atıf motorunun derlenmiş nesnesini ve (S20.5'te
// eklenecek) yükleyici yardımcılarını taşır.
//
// CO-RE eBPF programı attr.c'dedir. Üretilen Go binding'i ve derlenmiş nesne
// (attrprog_bpfel.{go,o}) repoya commit'lidir — bu yüzden `go build` clang
// gerektirmez. Yeniden üretmek: `make generate-bpf` (clang + libbpf-dev gerekir);
// CI `ebpf-generate-check` işi çıktının güncel olduğunu doğrular.
package bpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -tags linux -target bpfel -type flow_key -type flow_stat attrprog attr.c
