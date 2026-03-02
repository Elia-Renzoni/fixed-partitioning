.PHONY: test
.PHONY: build

test:
	go test ./...

build:
	go build ./...
