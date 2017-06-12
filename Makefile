version := $$CIRCLE_TAG
GITHUB_TOKEN := $$GH_LOGIN

release: gh-release clean dist
	github-release release \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name $(version)

	github-release upload \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name chamber-$(version)-darwin-amd64 \
	--file dist/chamber-$(version)-darwin-amd64

	github-release upload \
	--user segmentio \
	--repo chamber \
	--tag $(version) \
	--name chamber-$(version)-linux-amd64 \
	--file dist/chamber-$(version)-linux-amd64

clean:
	rm -rf ./dist

dist:
	mkdir dist
	GOOS=darwin GOARCH=amd64 go build -o dist/chamber-$(version)-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o dist/chamber-$(version)-linux-amd64

gh-release:
	go get -u github.com/aktau/github-release
