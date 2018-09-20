# Goals:
# - user can build binaries on their system without having to install special tools
# - user can fork the canonical repo and expect to be able to run CircleCI checks
#
# This makefile is meant for humans

VERSION := $(shell git describe --tags --always --dirty="-dev")
ANALYTICS_WRITE_KEY ?=
LDFLAGS := -ldflags='-X "main.Version=$(VERSION)" -X "main.AnalyticsWriteKey=$(ANALYTICS_WRITE_KEY)"'

test: | govendor
	govendor sync
	go test -v ./...

all: dist/chamber-$(VERSION)-darwin-amd64 dist/chamber-$(VERSION)-linux-amd64

clean:
	rm -rf ./dist

dist/:
	mkdir -p dist

dist/chamber-$(VERSION)-darwin-amd64: | govendor dist/
	govendor sync
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-linux-amd64: | govendor dist/
	govendor sync
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

govendor:
	go get -u github.com/kardianos/govendor

.PHONY: clean all govendor
