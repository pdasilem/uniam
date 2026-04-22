.PHONY: lint fix release-build
DEVBINDIR = $(CURDIR)/devbin

LINT_VERSION ?= v2.9.0
LINT_BIN := $(DEVBINDIR)/golangci-lint
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
LDFLAGS := -X uniam/internal/buildinfo.Version=$(VERSION)

lint: $(LINT_BIN)
	$(LINT_BIN) run ./...

fix: $(LINT_BIN)
	$(LINT_BIN) run --fix ./...

$(LINT_BIN):
	@echo "Installing golangci-lint $(LINT_VERSION)..."
	@mkdir -p $(DEVBINDIR)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(DEVBINDIR) $(LINT_VERSION)

.PHONY: vuln
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

release-build:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/uniam-darwin-arm64 ./cmd/uniam
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/uniam-darwin-amd64 ./cmd/uniam
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/uniam-linux-amd64 ./cmd/uniam
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/uniam-linux-arm64 ./cmd/uniam
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/uniam-windows-amd64.exe ./cmd/uniam
