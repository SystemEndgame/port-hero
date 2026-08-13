# ⚓ PORT HERO — Makefile
BINARY   := port-hero
VERSION  ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X github.com/golive-ly/port-hero/internal/tui.Version=$(VERSION) -X main.version=$(VERSION)
BUILD_DIR := binaries

.PHONY: all build test vet fmt lint release clean install uninstall

all: build

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run ./...

release:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/port-hero-darwin-arm64  .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/port-hero-darwin-amd64  .
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/port-hero-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/port-hero-linux-arm64   .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/port-hero-windows-amd64.exe .
	@echo "✔ release artifacts in $(BUILD_DIR)/"

install: build
	@install -d "$${HOME}/.local/bin" 2>/dev/null || true
	@install -m 0755 $(BUILD_DIR)/$(BINARY) "$${HOME}/.local/bin/$(BINARY)"
	@echo "✔ installed to $${HOME}/.local/bin/$(BINARY) (add it to PATH if needed)"

uninstall:
	@rm -f "$${HOME}/.local/bin/$(BINARY)"
	@echo "✔ removed $${HOME}/.local/bin/$(BINARY)"

clean:
	rm -rf $(BUILD_DIR) port-hero-bin
