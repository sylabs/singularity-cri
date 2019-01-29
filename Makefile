# silent build
V := @

# source/build locations
BIN_DIR := ./bin
SY_CRI := $(BIN_DIR)/sycri
FAKE_SH := $(BIN_DIR)/fakesh

all: $(SY_CRI) $(FAKE_SH)


$(SY_CRI): SECCOMP := "$(shell echo '#include <seccomp.h>\nint main() { }' | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1; echo $$?)"
$(SY_CRI):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)export GOOS=linux && go build -tags "selinux $(BUILD_TAGS)" -o $(SY_CRI) ./cmd/server

$(FAKE_SH): ARCH := `uname -p`
$(FAKE_SH):
	@echo " $(ARCH) FAKE SH"
	$(V)wget -O $(FAKE_SH) https://busybox.net/downloads/binaries/1.21.1/busybox-$(ARCH) 2> /dev/null
	$(V)chmod +x $(FAKE_SH)


.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean
	$(V)rm -rf $(BIN_DIR)

.PHONY: test
test:
	$(V)GOOS=linux go test -v -coverprofile=cover.out ./...

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
