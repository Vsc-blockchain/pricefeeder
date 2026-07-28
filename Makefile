# The version is derived from the git tag, never from a constant in the source.
# Override with `make build VERSION=...` only for one-off builds.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo unknown)
LDFLAGS := -X github.com/vsc-blockchain/pricefeeder/cmd.Version=$(VERSION)

# Everything the build needs from the environment is set here, so that no shell
# has to be prepared before running make.
#   GOPRIVATE   core and collections are private repos: keep them away from the
#               public module proxy and the checksum database.
#   GOTOOLCHAIN core requires Go 1.24, so let the toolchain fetch itself rather
#               than failing on an older local Go.
export GOPRIVATE   := github.com/vsc-blockchain/*
export GOTOOLCHAIN := auto

generate:
	go generate ./...

build-docker:
	docker-compose build --build-arg VERSION=$(VERSION)

docker-compose:
	docker-compose up

test:
	go test ./...

run:
	go run ./main.go

run-debug:
	go run ./main.go -debug true

###############################################################################
###                                Build                                    ###
###############################################################################

.PHONY: build
build:
	go build -mod=readonly -ldflags "$(LDFLAGS)" ./...

.PHONY: install
install:
	go install -mod=readonly -ldflags "$(LDFLAGS)" ./...

.PHONY: version
version:
	@echo $(VERSION)

###############################################################################
###                               Release                                   ###
###############################################################################

PACKAGE_NAME := github.com/vsc-blockchain/pricefeeder
RELEASE_DIR  ?= build
RELEASE_BIN  := $(RELEASE_DIR)/pricefeeder-$(VERSION)-linux-amd64

# `make release` is the one command needed to produce a shippable binary: no
# environment to prepare, no flags to remember.
#
# The result is statically linked and has no libc dependency, so it runs on any
# linux/amd64 distro (Debian, Ubuntu, Alpine, distroless, ...) regardless of the
# glibc version it was built against.
#
# CGO cannot be turned off here: the dependency graph reaches go-ethereum's cgo
# secp256k1 via lcp-go, which calls secp256k1.RecoverPubkey and has no pure-Go
# fallback, so CGO_ENABLED=0 fails to build. Static linking against glibc would
# normally break DNS and user lookups, which is what the netgo and osusergo tags
# take care of by using Go's own resolver instead of the platform's.
.PHONY: release
release: release-preflight
	@mkdir -p $(RELEASE_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath \
		-tags netgo,osusergo \
		-ldflags '-s -w -linkmode external -extldflags "-static" $(LDFLAGS)' \
		-o $(RELEASE_BIN) .
	@cd $(RELEASE_DIR) && sha256sum $(notdir $(RELEASE_BIN)) > $(notdir $(RELEASE_BIN)).sha256
	@echo
	@echo "  $(RELEASE_BIN)"
	@echo "  $$(cd $(RELEASE_DIR) && cut -d' ' -f1 $(notdir $(RELEASE_BIN)).sha256)"
	@file -b $(RELEASE_BIN) | cut -d, -f1-3 | sed 's/^/  /'
	@echo

# Fails early with an actionable message instead of letting the build die deep
# inside the module graph.
.PHONY: release-preflight
release-preflight:
	@command -v gcc >/dev/null 2>&1 || { \
		echo "release needs a C compiler (CGO is required, see below)."; \
		echo "  debian/ubuntu: apt install build-essential"; \
		exit 1; }
	@go mod download github.com/vsc-blockchain/core 2>/dev/null || { \
		echo "cannot fetch the private module github.com/vsc-blockchain/core."; \
		echo "give git an ssh deploy key once:"; \
		echo "  git config --global url.\"git@github.com:\".insteadOf \"https://github.com/\""; \
		exit 1; }

# Kept so that existing scripts and muscle memory do not break.
.PHONY: release-static
release-static: release
