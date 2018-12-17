# silent build
V := @

# source/build locations
BINDIR := ./bin
SY_CRI := $(BINDIR)/sycri
SY_CRI_SELINUX := $(BINDIR)/sycri-selinux

.PHONY: build
build: $(SY_CRI) $(SY_CRI_SELINUX)

$(SY_CRI):
	@echo " GO" $@
	@if echo "#include <seccomp.h>\nint main() { }" | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1 ; then \
		export GOOS=linux && go build -tags "seccomp" -o $(SY_CRI) ./cmd/server ; \
	else \
		export GOOS=linux && go build -o $(SY_CRI) ./cmd/server ; \
	fi

$(SY_CRI_SELINUX):
	@echo " GO" $@
	@if echo "#include <seccomp.h>\nint main() { }" | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1 ; then \
		export GOOS=linux && go build -tags "selinux seccomp" -o $(SY_CRI_SELINUX) ./cmd/server ; \
	else \
		export GOOS=linux && go build -tags "selinux" -o $(SY_CRI_SELINUX) ./cmd/server ; \
	fi


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
