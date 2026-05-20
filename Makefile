include .env

BIN ?= logistique
BUILD_DIRS := bin

# Used internally.  Users should pass GOOS and/or GOARCH.
OS := $(if $(GOOS),$(GOOS),$(shell GOTOOLCHAIN=local go env GOOS))
ARCH := $(if $(GOARCH),$(GOARCH),$(shell GOTOOLCHAIN=local go env GOARCH))
GO := $(if $(GOVERSION),$(GOVERSION),$(shell GOTOOLCHAIN=local go env GOVERSION))

SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset
GOFLAGS ?=
VERSION ?= $(shell git describe --tags --always --dirty)


all: # @HELP build container image 
all: build


build: # @HELP build app for local development
build: deps ci $(BUILD_DIRS)
	go build -o bin/logistique logistique.go


$(BUILD_DIRS):
	mkdir -p $@


clean: # @HELP clean build artifacts
clean:
	rm -rf bin


ci: # @HELP ci steps(lint, test)
ci: lint test


deps: # @HELP go mod tidy, download
deps:
	go mod tidy
	go mod download


lint: # @HELP lint with golangci-lint
lint:
	golangci-lint run


test: # @HELP runs unit tests
test:
	go test ./...

version: # @HELP prints the version string
version:
	@echo $(VERSION)


help: # @HELP prints this message
help:
	echo "VARIABLES:"
	echo "  BIN = $(BIN)"
	echo "  OS = $(OS)"
	echo "  ARCH = $(ARCH)"
	echo "  GOFLAGS = $(GOFLAGS)"
	echo "  GO = $(GO)"
	echo
	echo "TARGETS:"
	grep -E '^.*: *# *@HELP' $(MAKEFILE_LIST)     \
	    | awk '                                   \
	        BEGIN {FS = ": *# *@HELP"};           \
	        { printf "  %-30s %s\n", $$1, $$2 };  \
	    '

.SILENT: help
.PHONY: all build clean ci deps lint test version help
