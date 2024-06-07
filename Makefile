# Goals:
# - user can build binaries on their system without having to install special tools
# - user can fork the canonical repo and expect to be able to run Github Actions checks
#
# This makefile is meant for humans

ifndef VERSION
	VERSION := $(shell git describe --tags --always --dirty="-dev")
endif

ifndef TARGETARCH
	TARGETARCH := $(shell arch)
endif

VERSION_NO_V := $(shell echo "$(VERSION)" | sed 's/^v//')
VERSION_MAJOR_MINOR_PATCH := $(shell echo "$(VERSION)" | sed 's/^v\([0-9]*.[0-9]*.[0-9]*\).*/\1/')
VERSION_MAJOR_MINOR := $(shell echo "$(VERSION)" | sed 's/^v\([0-9]*.[0-9]*\).*/\1/')
VERSION_MAJOR := $(shell echo "$(VERSION)" | sed 's/^v\([0-9]*\).*/\1/')
ANALYTICS_WRITE_KEY ?=
LDFLAGS := -ldflags='-X "main.Version=$(VERSION)" -X "main.AnalyticsWriteKey=$(ANALYTICS_WRITE_KEY)"'
MOQ := $(shell command -v moq 2> /dev/null)
SRC := $(shell find . -name '*.go')
GOLANGCI_LINT := $(shell command -v golangci-lint 2> /dev/null)

vet:
	go vet ./...

test: store/awsapi_mock.go
	go test -v ./...

coverage:
	go test -coverpkg ./... -coverprofile coverage.out ./...

lint: vet
ifdef GOLANGCI_LINT
	@golangci-lint run --max-same-issues 0 --max-issues-per-linter 0
else
	@echo "Please install golangci-lint: brew install golangci-lint"
	@false
endif

store/awsapi_mock.go: store/awsapi.go
ifdef MOQ
	rm -f $@
	go generate ./...
else
	@echo "Unable to generate mocks"
	@echo "Please install moq: go install github.com/matryer/moq@latest"
endif

all: dist/chamber-$(VERSION)-darwin-amd64 dist/chamber-$(VERSION)-linux-amd64 dist/chamber-$(VERSION)-windows-amd64.exe

clean:
	rm -rf ./dist

dist/:
	mkdir -p dist

fmt:
	go fmt ./...

build: chamber

chamber: fmt $(SRC)
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-darwin-amd64: | dist/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-darwin-arm64: | dist/
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

linux: dist/chamber-$(VERSION)-linux-$(TARGETARCH)
	cp $^ chamber

dist/chamber-$(VERSION)-linux-amd64: | dist/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-linux-arm64 dist/chamber-$(VERSION)-linux-aarch64: | dist/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-windows-amd64.exe: | dist/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $@

.PHONY: vet test coverage lint clean all fmt build linux
