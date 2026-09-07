BINARY=bazntms
HUB=./cmd/bazntms-hub
AGENT=./cmd/bazntms-agent
CTL=./cmd/bazntmsctl
PORT?=8080

.PHONY: all frontend backend agent ctl dev clean cross-mac cross-linux generate-bpf generate-bpf-vmlinux

all: frontend backend agent ctl

frontend:
	cd frontend && npm install && npm run build

backend: frontend
	go build -o $(BINARY) $(HUB)

agent:
	go build -o bazntms-agent $(AGENT)

ctl:
	go build -o bazntmsctl $(CTL)

dev-backend:
	go run $(HUB) -dev -port $(PORT)

dev-frontend:
	cd frontend && npm run dev

run: all
	sudo ./$(BINARY) -port $(PORT)

test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt gerekli:"; gofmt -l .; exit 1; }
	cd frontend && npx tsc -b --noEmit

clean:
	rm -f $(BINARY) bazntms-agent bazntmsctl $(BINARY).exe
	rm -rf web/dist

# Faz 20: eBPF atıf motoru (internal/agent/bpf). Üretilen attrprog_bpfel.{go,o}
# + vmlinux.h repoya commit'lidir → normal `go build` clang GEREKTİRMEZ.
# Bu hedef yalnızca attr.c değişince çalıştırılır; sabit araç zinciri
# (golang:1.26-bookworm + clang-14) taşınabilirlik için Docker'da koşar.
# CI `ebpf-generate` işi çıktının güncelliğini aynı zincirle doğrular.
generate-bpf:
	docker run --rm -v $(CURDIR):/src -w /src golang:1.26-bookworm bash -c '\
		set -e; apt-get update -qq; apt-get install -y -qq clang-14 llvm libbpf-dev; \
		cd internal/agent/bpf && BPF2GO_CC=clang-14 go generate ./...; \
		chown -R $(shell id -u):$(shell id -g) .'

# vmlinux.h'i çalışan çekirdeğin BTF'inden tazele — nadiren gerekir (CO-RE
# alan ofsetlerini yükleme anında taşır; kernel tipi önemli değil). Sonra
# `make generate-bpf` çalıştırıp ikisini birlikte commit'leyin.
generate-bpf-vmlinux:
	docker run --rm --privileged -v /sys/kernel/btf:/sys/kernel/btf:ro \
		-v $(CURDIR):/src -w /src debian:bookworm bash -c '\
		apt-get update -qq && apt-get install -y -qq bpftool && \
		bpftool btf dump file /sys/kernel/btf/vmlinux format c > internal/agent/bpf/vmlinux.h && \
		chown $(shell id -u):$(shell id -g) internal/agent/bpf/vmlinux.h'

# Not: cross derleme icin hedef platformda libpcap/Npcap gerekir.
# Linux/macOS: CGO + libpcap; Windows: Npcap SDK + mingw-w64.
cross-mac:
	cd frontend && npm ci && npm run build
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 $(HUB)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 $(HUB)
	go build -o bazntmsctl-darwin-amd64 $(CTL)
	go build -o bazntmsctl-darwin-arm64 $(CTL)

cross-linux:
	cd frontend && npm ci && npm run build
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 $(HUB)
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-linux-arm64 $(HUB)
	go build -o bazntmsctl-linux-amd64 $(CTL)
	go build -o bazntmsctl-linux-arm64 $(CTL)
