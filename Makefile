VERSION := $(shell git describe --tags --always --dirty="-dev")
LDFLAGS := -ldflags='-X "main.Version=$(VERSION)"'

build: deps
	go build -o ./dist/chamber

deps: govendor
	@govendor sync

release: gh-release clean dist
	govendor sync
	github-release release \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name $(VERSION)

	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber-$(VERSION)-darwin-amd64 \
	--file dist/chamber-$(VERSION)-darwin-amd64

	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber-$(VERSION)-linux-amd64 \
	--file dist/chamber-$(VERSION)-linux-amd64

clean:
	rm -rf ./dist

dist:
	mkdir dist
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/chamber-$(VERSION)-darwin-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/chamber-$(VERSION)-linux-amd64

gh-release:
	@which github-release >/dev/null || go get -u github.com/aktau/github-release

govendor:
	@which govendor >/dev/null || go get -u github.com/kardianos/govendor
