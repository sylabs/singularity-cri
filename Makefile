# silent build
V := @

# source/build locations
BINDIR := ./bin
SY_CRI := $(BINDIR)/sycri

.PHONY: build
build: $(SY_CRI)

$(SY_CRI): SECCOMP := "$(shell echo '#include <seccomp.h>\nint main() { }' | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1; echo $$?)"
$(SY_CRI):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)export GOOS=linux && go build -tags "selinux $(BUILD_TAGS)" -o $(SY_CRI) ./cmd/server

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BINDIR)

.PHONY: test
test:
	$(V)GOOS=linux go test -count=1 -v -cover ./...

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
