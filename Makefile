# silent build
V := @

# source/build locations
BUILDDIR := ./vendor/github.com/singularityware/singularity/builddir
BINDIR := ./bin
SY_CRI := $(BINDIR)/sycri
CRI_CLIENT := $(BINDIR)/sycri_client

$(SY_CRI):
	@echo " GO" $@
	$(V)export GOOS=linux && go build -o $(SY_CRI) ./cmd/server

$(CRI_CLIENT): 
	@echo " GO" $@
	$(V)export GOOS=linux && go build -o $(CRI_CLIENT) ./cmd/test

.PHONY: build
build: $(SY_CRI) $(CRI_CLIENT)

.PHONY: clean
clean:
	@printf " CLEAN\n"
	$(V)go clean
	$(V)rm -rf $(BINDIR)
	$(V)rm -rf $(BUILDDIR)

.PHONY: test
test: $(CONFIG_GO)
	@export GOOS=linux && go test -cover ./...

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
