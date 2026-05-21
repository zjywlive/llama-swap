# Define variables for the application
APP_NAME = llama-swap
BUILD_DIR = build
DEV_DIR = $(CURDIR)/.dev
GO_ENV = GOPATH=$(DEV_DIR)/go GOMODCACHE=$(DEV_DIR)/go/pkg/mod GOCACHE=$(DEV_DIR)/go-build
GO = $(GO_ENV) go
STATICCHECK_VERSION ?= v0.7.0
STATICCHECK = $(DEV_DIR)/go/bin/staticcheck

# Get the current Git hash
GIT_HASH := $(shell git rev-parse --short HEAD)
ifneq ($(shell git status --porcelain),)
    # There are untracked changes
    GIT_HASH := $(GIT_HASH)+
endif

# Capture the current build date in RFC3339 format
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Default target: Builds binaries for both OSX and Linux
all: mac linux simple-responder

# Clean build directory
clean:
	rm -rf $(BUILD_DIR)

proxy/ui_dist/placeholder.txt:
	mkdir -p proxy/ui_dist
	touch $@

$(STATICCHECK):
	mkdir -p $(DEV_DIR)/go/bin
	$(GO_ENV) GOBIN=$(DEV_DIR)/go/bin go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

tools: $(STATICCHECK)

# use cached test results while developing
test-dev: proxy/ui_dist/placeholder.txt $(STATICCHECK)
	$(GO) test -short ./proxy/... ./internal/...
	$(GO_ENV) $(STATICCHECK) ./proxy/... ./internal/...

test: proxy/ui_dist/placeholder.txt
	$(GO) test -short -count=1 ./proxy/... ./internal/...

# for CI - full test (takes longer)
test-all: proxy/ui_dist/placeholder.txt
	$(GO) test -race -count=1 ./proxy/... ./internal/...

ui/node_modules:
	cd ui-svelte && npm install

# build react UI
ui: ui/node_modules
	cd ui-svelte && npm run build

# Build OSX binary
mac: ui
	@echo "Building Mac binary..."
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-X main.commit=${GIT_HASH} -X main.version=local_${GIT_HASH} -X main.date=${BUILD_DATE}" -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64

# Build Linux binary
linux: linux-arm64 linux-amd64

linux-amd64: ui
	@echo "Building Linux AMD64 binary..."
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-X main.commit=${GIT_HASH} -X main.version=local_${GIT_HASH} -X main.date=${BUILD_DATE}" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64

linux-arm64: ui
	@echo "Building Linux ARM64 binary..."
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-X main.commit=${GIT_HASH} -X main.version=local_${GIT_HASH} -X main.date=${BUILD_DATE}" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64

# Build Windows binary
windows: ui
	@echo "Building Windows binary..."
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-X main.commit=${GIT_HASH} -X main.version=local_${GIT_HASH} -X main.date=${BUILD_DATE}" -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe

# for testing proxy.Process
simple-responder:
	@echo "Building simple responder"
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/simple-responder_darwin_arm64 cmd/simple-responder/simple-responder.go
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/simple-responder_linux_amd64 cmd/simple-responder/simple-responder.go

simple-responder-windows:
	@echo "Building simple responder for windows"
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/simple-responder.exe cmd/simple-responder/simple-responder.go

# Ensure build directory exists
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Create a new release tag
release:
	@echo "Checking for unstaged changes..."
	@if [ -n "$(shell git status --porcelain)" ]; then \
		echo "Error: There are unstaged changes. Please commit or stash your changes before creating a release tag." >&2; \
		exit 1; \
	fi

# Get the highest tag in v{number} format, increment it, and create a new tag
	@highest_tag=$$(git tag --sort=-v:refname | grep -E '^v[0-9]+$$' | head -n 1 || echo "v0"); \
	new_tag="v$$(( $${highest_tag#v} + 1 ))"; \
	echo "tagging new version: $$new_tag"; \
	git tag "$$new_tag";

GOOS ?= $(shell $(GO_ENV) go env GOOS 2>/dev/null || echo linux)
GOARCH ?= $(shell $(GO_ENV) go env GOARCH 2>/dev/null || echo amd64)
wol-proxy: $(BUILD_DIR)
	@echo "Building wol-proxy"
	$(GO) build -o $(BUILD_DIR)/wol-proxy-$(GOOS)-$(GOARCH)-$(shell date +%Y-%m-%d) cmd/wol-proxy/wol-proxy.go

test-ui:
	cd ui-svelte && npm ci && npm run check && npm test

# Phony targets
.PHONY: all clean ui mac windows simple-responder simple-responder-windows test test-all test-dev test-ui tools wol-proxy
.PHONE: linux linux-arm64 linux-amd64
