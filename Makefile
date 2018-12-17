# silent build
V := @

# source/build locations
BINDIR := ./bin
SY_CRI := $(BINDIR)/sycri
SY_CRI_SELINUX := $(BINDIR)/sycri-selinux
SECCOMP := $(shell echo '\#include <seccomp.h>\nint main() { }' | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1; echo $$?)
BUILD_TAGS :=
ifeq ($(SECCOMP), 0)
	BUILD_TAGS += seccomp
endif


.PHONY: build
build: $(SY_CRI) $(SY_CRI_SELINUX)

$(SY_CRI):
	@echo " GO" $@
	$(V)export GOOS=linux && go build -tags "$(BUILD_TAGS)" -o $(SY_CRI) ./cmd/server

$(SY_CRI_SELINUX):
	@echo " GO" $@
	$(V)export GOOS=linux && go build -tags "selinux $(BUILD_TAGS)" -o $(SY_CRI_SELINUX) ./cmd/server

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BINDIR)

.PHONY: test
test:
	$(V)export GOOS=linux && go test -v -cover ./...

.PHONY: lint
lint:
	$(V)gometalinter --vendor --disable-all \
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
