.PHONY: lint

build:
	GOOS=linux go build -o sycri.out ./cmd/server
	GOOS=linux go build -o sycri.test.out ./cmd/test
	GOOS=linux go test -c -o sycri.runtime.test ./pkg/runtime
	GOOS=linux go test -c -o sycri.image.test ./pkg/image

lint:
	gometalinter --vendor --disable-all \
	--enable=gofmt \
	--enable=goimports \
	--enable=vet \
	--enable=misspell \
	--enable=maligned \
	--enable=deadcode \
	--enable=ineffassign \
	--enable=golint \
	--enable=errcheck \
	--enable=varcheck \
	--deadline=3m ./...
