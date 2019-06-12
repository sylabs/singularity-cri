# silent build
V := @

BIN_DIR := ./bin
SY_CRI := $(BIN_DIR)/sycri
SY_CRI_TEST := $(BIN_DIR)/sycri.test

INSTALL_DIR := /usr/local/bin
SY_CRI_INSTALL := $(INSTALL_DIR)/sycri

CRI_CONFIG := ./config/sycri.yaml
CRI_CONFIG_INSTALL := /usr/local/etc/sycri/sycri.yaml

SECCOMP := "$(shell printf "\#include <seccomp.h>\nint main() { seccomp_syscall_resolve_name(\"read\"); }" | gcc -x c -o /dev/null - -lseccomp >/dev/null 2>&1; echo $$?)"
ARCH := `arch`

all: $(SY_CRI)

$(SY_CRI):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)GO111MODULE=off GOOS=linux go build -tags "selinux $(BUILD_TAGS)" -o $(SY_CRI) ./cmd/server

install: $(SY_CRI_INSTALL) $(CRI_CONFIG_INSTALL)

$(SY_CRI_INSTALL):
	@echo " INSTALL" $@
	$(V)install -d $(@D)
	$(V)install -m 0755 $(SY_CRI) $(SY_CRI_INSTALL)

$(CRI_CONFIG_INSTALL):
	@echo " INSTALL" $@
	$(V)install -d $(@D)
	$(V)install -m 0644 $(CRI_CONFIG) $(CRI_CONFIG_INSTALL)

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)GO111MODULE=off go clean
	$(V)rm -rf $(BIN_DIR)

.PHONY: uninstall
uninstall:
	@echo " UNINSTALL"
	$(V)rm -rf $(SY_CRI_INSTALL) $(CRI_CONFIG_INSTALL)

.PHONY: test
test:
	$(V)GO111MODULE=off GOOS=linux go test -v -coverpkg=./... -coverprofile=cover.out -race ./...

$(SY_CRI_TEST):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)GO111MODULE=off GOOS=linux go test -c -o $(SY_CRI_TEST) -tags "selinux $(BUILD_TAGS) testrunmain" \
	-coverpkg=./... ./cmd/server

.PHONY: lint
lint:
	$(V)golangci-lint run --disable-all \
	--enable=gofmt \
	--enable=goimports \
	--enable=vet \
	--enable=misspell \
	--enable=maligned \
	--enable=deadcode \
	--enable=ineffassign \
	--enable=golint \
	--deadline=3m ./...

dep:
	$(V)GO111MODULE=on go mod vendor
	$(V)GO111MODULE=on go mod tidy
