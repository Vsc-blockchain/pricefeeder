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

PACKAGE_NAME		  := github.com/vsc-blockchain/pricefeeder
GOLANG_CROSS_VERSION  ?= v1.19.4

release:
	docker run \
		--rm \
		--platform=linux/amd64 \
		-v "$(CURDIR)":/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		-e CGO_ENABLED=1 \
		-e GITHUB_TOKEN=${GITHUB_TOKEN} \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --rm-dist

release-snapshot:
	docker run \
		--rm \
		--platform=linux/amd64 \
		-v "$(CURDIR)":/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		-e CGO_ENABLED=1 \
		-e GITHUB_TOKEN=${GITHUB_TOKEN} \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --rm-dist --snapshot
