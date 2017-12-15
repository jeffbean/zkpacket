.PHONY: test
test:
	go test -v

.PHONY: build
build:
	go build .

default: build test