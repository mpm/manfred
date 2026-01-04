# MANFRED Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := manfred

.PHONY: build
build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/manfred

.PHONY: install
install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY)

.PHONY: test
test:
	go test -v ./...

.PHONY: clean
clean:
	rm -rf bin/ dist/

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint: fmt vet

# Cross-compilation for releases
.PHONY: release
release: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/manfred
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/manfred
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/manfred
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/manfred
	@cd dist && sha256sum * > checksums.txt
	@echo "Release binaries created in dist/"

# Claude bundle (portable Node.js + Claude Code)
.PHONY: bundle
bundle:
	cd claude-bundle && ./build.sh

.PHONY: bundle-install
bundle-install: bundle
	@mkdir -p $(HOME)/.manfred
	tar -xzf claude-bundle/dist/claude-bundle-linux-amd64.tar.gz -C $(HOME)/.manfred
	mv $(HOME)/.manfred/claude-bundle-linux-amd64 $(HOME)/.manfred/claude-bundle
	@echo "Claude bundle installed to $(HOME)/.manfred/claude-bundle"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Build and install to GOPATH/bin"
	@echo "  test           - Run tests"
	@echo "  clean          - Remove build artifacts"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Run fmt and vet"
	@echo "  release        - Build release binaries for all platforms"
	@echo "  bundle         - Build the portable Claude Code bundle"
	@echo "  bundle-install - Build and install Claude bundle to ~/.manfred/"
