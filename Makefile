BINARY=bazntms
HUB=./cmd/bazntms-hub
AGENT=./cmd/bazntms-agent
PORT?=8080

.PHONY: all frontend backend agent dev clean cross-mac cross-linux

all: frontend backend agent

frontend:
	cd frontend && npm install && npm run build

backend: frontend
	go build -o $(BINARY) $(HUB)

agent:
	go build -o bazntms-agent $(AGENT)

dev-backend:
	go run $(HUB) -dev -port $(PORT)

dev-frontend:
	cd frontend && npm run dev

run: all
	sudo ./$(BINARY) -port $(PORT)

test:
	go vet ./... && gofmt -l . && cd frontend && npx tsc -b --noEmit

clean:
	rm -f $(BINARY) bazntms-agent $(BINARY).exe
	rm -rf web/dist

# Not: cross derleme icin hedef platformda libpcap/Npcap gerekir.
# Linux/macOS: CGO + libpcap; Windows: Npcap SDK + mingw-w64.
cross-mac:
	cd frontend && npm ci && npm run build
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 $(HUB)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 $(HUB)

cross-linux:
	cd frontend && npm ci && npm run build
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 $(HUB)
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-linux-arm64 $(HUB)
