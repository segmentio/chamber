# Goals:
# - user can build binaries on their system without having to install special tools
# - user can fork the canonical repo and expect to be able to run CircleCI checks
#
# This makefile is meant for humans

VERSION := $(shell git describe --tags --always --dirty="-dev")
VERSION_MAJOR_MINOR_PATCH := $(shell git describe --tags --always --dirty="-dev" | sed 's/^v\([0-9]*.[0-9]*.[0-9]*\).*/\1/')
VERSION_MAJOR_MINOR := $(shell git describe --tags --always --dirty="-dev" | sed 's/^v\([0-9]*.[0-9]*\).*/\1/')
VERSION_MAJOR := $(shell git describe --tags --always --dirty="-dev" | sed 's/^v\([0-9]*\).*/\1/')
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

docker-image: docker-image-$(VERSION)

docker-image-$(VERSION):
	docker build \
		-t segment/chamber:$(VERSION_MAJOR_MINOR_PATCH) \
		-t segment/chamber:$(VERSION_MAJOR_MINOR) \
		-t segment/chamber:$(VERSION_MAJOR) \
		.

docker-image-publish: docker-image
	docker push segment/chamber:$(VERSION_MAJOR_MINOR_PATCH)
	docker push segment/chamber:$(VERSION_MAJOR_MINOR)
	docker push segment/chamber:$(VERSION_MAJOR)

.PHONY: clean all linux docker-image docker-image-$(VERSION) docker-image-publish
