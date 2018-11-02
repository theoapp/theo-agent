NAME ?= theo-agent
PACKAGE_NAME ?= $(NAME)

export VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
BUILT := $(shell date -u +%Y-%m-%dT%H:%M:%S%z)

LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PKG = github.com/theoapp/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE=$(PKG)/common

BUILD_PLATFORMS ?= -os '!netbsd' -os '!openbsd' -os '!windows'

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

ORIGINAL_GOPATH = $(shell echo $$GOPATH)
LOCAL_GOPATH := $(CURDIR)/.gopath
GOPATH_SETUP := $(LOCAL_GOPATH)/.ok
GOPATH_BIN := $(LOCAL_GOPATH)/bin
PKG_BUILD_DIR := $(LOCAL_GOPATH)/src/$(PKG)

export GOPATH = $(LOCAL_GOPATH)
export PATH := $(GOPATH_BIN):$(PATH)

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(PACKAGE_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).REVISION=$(REVISION) -X $(COMMON_PACKAGE_NAMESPACE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -s -w

# Development Tools
DEP = $(GOPATH_BIN)/dep
GOX = $(GOPATH_BIN)/gox
DEVELOPMENT_TOOLS = $(DEP) $(GOX)

OSARCH := "linux/amd64 linux/386 darwin/amd64"

.PHONY: clean version
.DEFAULT: help # Running Make will run the help target

all: deps build

help: 
	@echo Help
	# Commands:
	# make all => deps build
	# make version - show information about current version
	#
	#
	# Deployment commands:
	# make deps - install all dependencies

version:
	@echo Current version: $(VERSION)


deps: $(DEVELOPMENT_TOOLS)

build: $(GOX)
	# Building $(NAME) in version $(VERSION) for $(BUILD_PLATFORMS)
	gox $(BUILD_PLATFORMS) \
	    -ldflags "$(GO_LDFLAGS)" \
		-output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}" \
		$(PKG)

build_simple: dep_check
	# Building $(NAME) in version $(VERSION) for current platform
	go build \
		-ldflags "$(GO_LDFLAGS)" \
		-o "out/binaries/$(NAME)" \
		$(PKG)


dep_check: $(DEP)
	@cd $(PKG_BUILD_DIR) && $(DEP) check

# local GOPATH
$(GOPATH_SETUP): $(PKG_BUILD_DIR)
	mkdir -p $(GOPATH_BIN)
	touch $@

$(PKG_BUILD_DIR):
	mkdir -p $(@D)
	ln -s ../../../.. $@

# development tools
$(DEP): $(GOPATH_SETUP)
	go get github.com/golang/dep/cmd/dep

$(GOX): $(GOPATH_SETUP)
	go get github.com/mitchellh/gox

clean:
	-$(RM) -rf $(LOCAL_GOPATH)
	-$(RM) -rf $(TARGET_DIR)
