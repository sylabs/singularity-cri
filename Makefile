# silent build
V := @

BIN_DIR := ./bin
SY_CRI := $(BIN_DIR)/sycri
FAKE_SH := $(BIN_DIR)/fakesh

INSTALL_DIR := /usr/local/bin
SY_CRI_INSTALL := $(INSTALL_DIR)/sycri
FAKE_SH_INSTALL := $(INSTALL_DIR)/sycri-bin/fakesh

CRI_CONFIG := ./config/sycri.yaml
CRI_CONFIG_INSTALL := /usr/local/etc/sycri/sycri.yaml

all: $(SY_CRI) $(FAKE_SH)

$(SY_CRI): SECCOMP := "$(shell echo '#include <seccomp.h>\nint main() { }' | gcc -x c -o /dev/null -lseccomp - >/dev/null 2>&1; echo $$?)"
$(SY_CRI):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)GO111MODULE=on GOOS=linux go build -tags "selinux $(BUILD_TAGS)" -o $(SY_CRI) ./cmd/server

$(FAKE_SH): ARCH := `arch`
$(FAKE_SH):
	@echo " $(ARCH) SHELL"
	$(V)wget -O $(FAKE_SH) https://busybox.net/downloads/binaries/1.21.1/busybox-$(ARCH) 2> /dev/null
	$(V)chmod +x $(FAKE_SH)

install: $(SY_CRI_INSTALL) $(FAKE_SH_INSTALL) $(CRI_CONFIG_INSTALL)

$(SY_CRI_INSTALL):
	@echo " INSTALL" $@
	$(V)install -d $(@D)
	$(V)install -m 0755 $(SY_CRI) $(SY_CRI_INSTALL)

$(CRI_CONFIG_INSTALL):
	@echo " INSTALL" $@
	$(V)install -d $(@D)
	$(V)install -m 0644 $(CRI_CONFIG) $(CRI_CONFIG_INSTALL)

$(FAKE_SH_INSTALL):
	@echo " INSTALL" $@
	$(V)install -d $(@D)
	$(V)install -m 0755 $(FAKE_SH) $(FAKE_SH_INSTALL)

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)GO111MODULE=off go clean
	$(V)rm -rf $(BIN_DIR)

.PHONY: uninstall
uninstall:
	@echo " UNINSTALL"
	$(V)rm -rf $(SY_CRI_INSTALL) $(FAKE_SH_INSTALL) $(CRI_CONFIG_INSTALL)

.PHONY: test
test:
	$(V)GOOS=linux go test -v -coverprofile=cover.out ./...

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
	$(V)go mod download
	$(V)go mod tidy
