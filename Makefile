VERSION := $(shell git describe --tags --always --dirty="-dev")
LDFLAGS := -ldflags='-X "main.Version=$(VERSION)"'
# set --pre-release if not tagged or tree is dirty or there's a `-` in the tag
ifneq (,$(findstring -,$(VERSION)))
	GITHUB_RELEASE_FLAGS := "--pre-release"
endif

all: dist/chamber-$(VERSION)-darwin-amd64 dist/chamber-$(VERSION)-linux-amd64

release: gh-release
	github-release release \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	$(GITHUB_RELEASE_FLAGS) \
	--tag $(VERSION) \
	--name $(VERSION)

publish: publish-github

publish-github: publish-github-darwin publish-github-linux publish-github-deb publish-github-rpm

publish-github-darwin: dist/chamber-$(VERSION)-darwin-amd64 release
	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber-$(VERSION)-darwin-amd64 \
	--file $<

publish-github-linux: dist/chamber-$(VERSION)-linux-amd64 release
	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber-$(VERSION)-linux-amd64 \
	--file $<

publish-github-deb: dist/chamber_$(VERSION)_amd64.deb release
	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber_$(VERSION)_amd64.deb \
	--file $<

publish-github-rpm: dist/chamber_$(VERSION)_amd64.rpm release
	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber_$(VERSION)_amd64.rpm \
	--file $<

	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(VERSION) \
	--name chamber-$(VERSION).sha256sums \
	--file dist/chamber-$(VERSION).sha256sums

clean:
	rm -rf ./dist

dist: dist/chamber-$(VERSION)-darwin-amd64 dist/chamber-$(VERSION)-linux-amd64 dist/chamber_$(VERSION)_amd64.deb dist/chamber_$(VERSION)_amd64.rpm
	@which sha256sum 2>&1 > /dev/null || ( \
		echo 'missing sha256sum; install on MacOS with `brew install coreutils && ln -s $$(which gsha256sum) /usr/local/bin/sha256sum`' ; \
		exit 1; \
	)
	cd dist && \
		sha256sum chamber-$(VERSION)-* > chamber-$(VERSION).sha256sums

dist/:
	mkdir -p dist
	

dist/chamber-$(VERSION)-darwin-amd64: dist/ govendor
	govendor sync
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

dist/chamber-$(VERSION)-linux-amd64: dist/ govendor
	govendor sync
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

dist/nfpm-$(VERSION).yaml: dist/
	sed -e "s/\$${VERSION}/$(VERSION)/g" -e "s|\$${DIST_BIN}|dist/chamber-$(VERSION)-linux-amd64|g"  < nfpm.yaml.tmpl > $@

dist/chamber_$(VERSION)_amd64.deb: dist/nfpm-$(VERSION).yaml
	nfpm -f $^ pkg --target $@

dist/chamber_$(VERSION)_amd64.rpm: dist/nfpm-$(VERSION).yaml
	nfpm -f $^ pkg --target $@

gh-release:
	go get -u github.com/aktau/github-release

govendor:
	go get -u github.com/kardianos/govendor

.PHONY: clean gh-release govendor publish-github publish-github-linux publish-github-rpm publish-github-deb publish-github-darwin release all
