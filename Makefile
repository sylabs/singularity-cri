.PHONY: lint

build:
	GOOS=linux go build -o sycri ./cmd/server
	GOOS=linux go test -c -o sycri.runtime.test ./service/runtime

lint:
	gometalinter --vendor --enable=misspell --enable=unparam --enable=dupl --enable=gofmt --enable=goimports --disable=gotype --disable=gas --deadline=3m ./...
