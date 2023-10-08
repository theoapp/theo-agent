.PHONY: all test clean buildx

PACKAGE_NAME ?=theo-agent
PACKAGE_NAMESPACE=github.com/theoapp/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE=$(PACKAGE_NAMESPACE)/common

VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell ./ci/branch)
BUILT := $(shell date -u +%Y-%m-%dT%H:%M:%S%z)

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(PACKAGE_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).REVISION=$(REVISION) -X $(COMMON_PACKAGE_NAMESPACE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -s -w

BUILD_DIR=build

all: test build

buildx:
	mkdir -p build
	go build -ldflags "$(GO_LDFLAGS)" -o $(BUILD_DIR)/$(PACKAGE_NAME)-$(shell echo "$(GOOS)-$(GOARCH)v$(GOARM)l" | sed 's/amd64/x86_64/; s/386/i686/; s/darwin/Darwin/; s/linux/Linux/; s/freebsd-x86_64/FreeBSD-amd64/; s/arm64/aarch64/; s/vl$$//')

build: test
	mkdir -p build
	go build -ldflags "$(GO_LDFLAGS)" -o $(BUILD_DIR)/$(PACKAGE_NAME)

test:
	go test ./...

clean:
	go clean ./...
	rm -rf $(BUILD_DIR)
