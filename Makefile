PACKAGE_NAME		  := github.com/vsc-blockchain/pricefeeder
GOLANG_CROSS_VERSION  ?= v1.19.4
VERSION ?= $(shell git describe --tags --abbrev=0)
COMMIT ?= $(shell git rev-parse HEAD)
BUILD_TARGETS := build install

generate:
	go generate ./...

build-docker:
	docker-compose build

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

.PHONY: build install
$(BUILD_TARGETS):
	go $@ -mod=readonly -ldflags="-s -w -X github.com/vsc-blockchain/pricefeeder/cmd.Version=$(VERSION) -X github.com/vsc-blockchain/pricefeeder/cmd.CommitHash=$(COMMIT)" ./...
