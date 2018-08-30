.PHONY: lint

lint:
	gometalinter --vendor --enable=misspell --enable=unparam --enable=dupl --enable=gofmt --enable=goimports --disable=gotype --disable=gas --deadline=3m ./...
