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

all: $(SY_CRI)

$(SY_CRI):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)GOOS=linux go build -mod vendor -tags "selinux $(BUILD_TAGS)" \
		-ldflags "-X main.version=`(git describe --tags --dirty --always 2>/dev/null || echo "unknown") \
		| sed -e "s/^v//;s/-/_/g;s/_/-/;s/_/./g"`" \
		-o $(SY_CRI) ./cmd/server

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
	$(V)go clean -mod vendor
	$(V)rm -rf $(BIN_DIR)

.PHONY: uninstall
uninstall:
	@echo " UNINSTALL"
	$(V)rm -rf $(SY_CRI_INSTALL) $(CRI_CONFIG_INSTALL)

.PHONY: test
test:
	$(V)GOOS=linux go test -mod vendor -v -coverpkg=./... -coverprofile=cover.out -race ./...

$(SY_CRI_TEST):
	@echo " GO" $@
	@if [ $(SECCOMP) -eq "0" ] ; then \
		_=$(eval BUILD_TAGS = seccomp) ; \
	else \
		echo " WARNING: seccomp is not found, ignoring" ; \
	fi
	$(V)GOOS=linux go test -mod vendor -c -o $(SY_CRI_TEST) -tags "selinux $(BUILD_TAGS) testrunmain" \
	-coverpkg=./... ./cmd/server

.PHONY: lint
lint:
	$(V)golangci-lint run --config .golangci.local.yml

dep:
	$(V)go mod tidy
	$(V)go mod vendor


GITHUB_USER := sylabs
GITHUB_REPO := singularity-cri

.PHONY: release
release:
	$(V)echo "Uploading artifacts to $(GITHUB_TAG) release"
	$(V)gothub upload \
        --security-token $(GITHUB_TOKEN) \
        --user $(GITHUB_USER) \
        --repo $(GITHUB_REPO) \
        --tag $(GITHUB_TAG) \
        --name "Singularity-CRI" \
        --file $(SY_CRI) \
        --replace
