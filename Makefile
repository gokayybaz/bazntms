BINARY=bazntms
PORT?=8080

.PHONY: all frontend backend dev clean cross-mac cross-linux

all: frontend backend

frontend:
	cd frontend && npm install && npm run build

backend: frontend
	go build -o $(BINARY) .

dev-backend:
	go run . -dev -port $(PORT)

dev-frontend:
	cd frontend && npm run dev

run: all
	sudo ./$(BINARY) -port $(PORT)

test:
	go vet ./... && gofmt -l . && cd frontend && npx tsc -b --noEmit

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf frontend/dist

# Not: cross derleme icin hedef platformda libpcap/Npcap gerekir.
# Linux/macOS: CGO + libpcap; Windows: Npcap SDK + mingw-w64.
cross-mac:
	cd frontend && npm ci && npm run build
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 .

cross-linux:
	cd frontend && npm ci && npm run build
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-linux-arm64 .
