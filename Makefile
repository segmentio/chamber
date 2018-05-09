.PHONY: build clean deps test

build:
	go build -o ./bin/chamber

clean:
	rm -rf ./bin

deps:
	@which govendor >/dev/null || go get -u github.com/kardianos/govendor
	@govendor sync

test:
	go test ./...
