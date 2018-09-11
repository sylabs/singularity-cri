.PHONY: test lint

build:
	GOOS=linux go build -o sycri ./cmd/server
	GOOS=linux go build -o sycri_client ./cmd/test

test:
	GOOS=linux go test -cover ./...

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
	--enable=unused \
	--deadline=3m ./...
