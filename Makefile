# Goals:
# - user can build binaries on their system without having to install special tools
# - user can fork the canonical repo and expect to be able to run CircleCI checks
#
# This makefile is meant for humans

VERSION := $(shell git describe --tags --always --dirty="-dev")
ANALYTICS_WRITE_KEY ?=
LDFLAGS := -ldflags='-X "main.Version=$(VERSION)" -X "main.AnalyticsWriteKey=$(ANALYTICS_WRITE_KEY)"'

test:
	GO111MODULE=on go test -v ./...

all: dist/chamber-$(VERSION)-darwin-amd64 dist/chamber-$(VERSION)-linux-amd64 dist/chamber-$(VERSION)-windows-amd64.exe

clean:
	rm -rf ./dist

dist/:
	mkdir -p dist

dist/chamber-$(VERSION)-darwin-amd64: | dist/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build $(LDFLAGS) -o $@

linux: dist/chamber-$(VERSION)-linux-amd64
	cp $^ chamber

dist/chamber-$(VERSION)-linux-amd64: | dist/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-windows-amd64.exe: | dist/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build $(LDFLAGS) -o $@

.PHONY: clean all linux
