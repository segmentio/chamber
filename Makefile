version := $$CIRCLE_TAG

release: gh-release clean dist
	govendor sync
	github-release release \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name $(version)

	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name chamber-$(version)-darwin-amd64 \
	--file dist/chamber-$(version)-darwin-amd64

	github-release upload \
	--security-token $$GH_LOGIN \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name chamber-$(version)-linux-amd64 \
	--file dist/chamber-$(version)-linux-amd64

clean:
	rm -rf ./dist

dist:
	mkdir dist
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o dist/chamber-$(version)-darwin-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/chamber-$(version)-linux-amd64

gh-release:
	go get -u github.com/aktau/github-release

govendor:
	go get -u github.com/kardianos/govendor
