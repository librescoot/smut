# Simple Mender Update Tool (SMUT) Makefile

# Version information
VERSION := $(shell git describe --always --dirty=-$(shell hostname)-$(shell date -u +%Y%m%d-%H%M%S))

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=smut
BINARY_UNIX=$(BINARY_NAME)
MAIN_PATH=./cmd/smut

# Cross-compilation parameters
GOOS_ARM=linux
GOARCH_ARM=arm
GOARM=7
CGO_ENABLED=0

all: clean deps build

smut: all

build:
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)

build-arm:
	env GOOS=$(GOOS_ARM) GOARCH=$(GOARCH_ARM) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME) $(MAIN_PATH)

dist-arm:
	env GOOS=$(GOOS_ARM) GOARCH=$(GOARCH_ARM) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BINARY_NAME)-arm-dist $(MAIN_PATH)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

deps:
	$(GOMOD) tidy

# Build for multiple platforms
build-all: build build-arm

# Run the application
run:
	./$(BINARY_NAME) --redis-addr=localhost:6379 --download-dir=/tmp

# Help
help:
	@echo "make - Build the application for the current platform"
	@echo "make build-arm - Build the application for ARM (armv7l)"
	@echo "make test - Run tests"
	@echo "make clean - Remove binaries"
	@echo "make deps - Update dependencies"
	@echo "make build-all - Build for all platforms"
	@echo "make run - Run the application"

.PHONY: all build build-arm test clean deps build-all run help
