NAME ?= theo-agent
PACKAGE_NAME ?= $(NAME)


LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PKG = theoapp.com/$(PACKAGE_NAME)


BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

ORIGINAL_GOPATH = $(shell echo $$GOPATH)
LOCAL_GOPATH := $(CURDIR)/.gopath
GOPATH_SETUP := $(LOCAL_GOPATH)/.ok
GOPATH_BIN := $(LOCAL_GOPATH)/bin
PKG_BUILD_DIR := $(LOCAL_GOPATH)/src/$(PKG)

export GOPATH = $(LOCAL_GOPATH)
export PATH := $(GOPATH_BIN):$(PATH)


# Development Tools
DEP = $(GOPATH_BIN)/dep
GOX = $(GOPATH_BIN)/gox
DEVELOPMENT_TOOLS = $(DEP) $(GOX)

OSARCH := "linux/amd64 linux/386 darwin/amd64"

.PHONY: all # All targets are accessible for user
.DEFAULT: help # Running Make will run the help target

help: 
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
	ln -s ../../.. $@

# development tools
$(DEP): $(GOPATH_SETUP)
	go get github.com/golang/dep/cmd/dep

$(GOX): $(GOPATH_SETUP)
	go get github.com/mitchellh/gox

clean:
	-$(RM) -rf $(LOCAL_GOPATH)
	-$(RM) -rf $(TARGET_DIR)
