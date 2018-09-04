.PHONY: lint

build:
	GOOS=linux go build -o sycri.out ./cmd/server
	GOOS=linux go test -c -o sycri.runtime.test ./pkg/runtime
	GOOS=linux go test -c -o sycri.image.test ./pkg/image

lint:
	gometalinter --vendor --enable=misspell --enable=unparam --enable=dupl --enable=gofmt --enable=goimports --disable=gotype --disable=gas --deadline=3m ./...
