.PHONY: build generate test

default: build

build:
	go install github.com/dollarshaveclub/acyl

generate:
	go generate ./...

test:
	go test ./...
