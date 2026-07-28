# The version is derived from the git tag, never from a constant in the source.
# Override with `make build VERSION=...` only for one-off builds.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo unknown)
LDFLAGS := -X github.com/vsc-blockchain/pricefeeder/cmd.Version=$(VERSION)

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

# Portable release binary: statically linked, no libc dependency, runs on any
# linux/amd64 distro (Debian, Ubuntu, Alpine, distroless, ...).
#
# CGO cannot be turned off here: the dependency graph reaches go-ethereum's cgo
# secp256k1 via lcp-go, which calls secp256k1.RecoverPubkey and has no pure-Go
# fallback, so CGO_ENABLED=0 fails to build. Static linking against glibc would
# normally break DNS and user lookups, which is what the netgo and osusergo tags
# take care of by using Go's own resolver instead of the platform's.
.PHONY: release-static
release-static:
	mkdir -p $(RELEASE_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath \
		-tags netgo,osusergo \
		-ldflags '-s -w -linkmode external -extldflags "-static" $(LDFLAGS)' \
		-o $(RELEASE_BIN) .
	cd $(RELEASE_DIR) && sha256sum $(notdir $(RELEASE_BIN)) > $(notdir $(RELEASE_BIN)).sha256
	@file $(RELEASE_BIN)
