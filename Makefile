# silent build
V := @

# source/build locations
BINDIR := ./bin
SY_CRI := $(BINDIR)/sycri
SY_CRI_SECURE := $(BINDIR)/sycri-secure

.PHONY: build
build: clean $(SY_CRI) $(SY_CRI_SECURE)

$(SY_CRI):
	@echo " GO" $@
	$(V)export GOOS=linux && go build -o $(SY_CRI) ./cmd/server

$(SY_CRI_SECURE):
	@echo " GO" $@
	$(V)export GOOS=linux && go build -tags 'selinux seccomp' -o $(SY_CRI_SECURE) ./cmd/server


.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BINDIR)

.PHONY: test
test:
	@export GOOS=linux && go test -v -cover ./...

.PHONY: lint
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

dep:
	dep ensure -vendor-only -v
