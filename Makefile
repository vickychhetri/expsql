# =========================
# Multi-OS Go Build Makefile (FINAL)
# =========================

GO := go
BUILD_DIR := build

APP_CLI := mysqltool
APP_GUI := exsql

LDFLAGS := -s -w
GUI_TAGS := gui

# Platforms (CLI only)
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	windows/amd64 \
	darwin/amd64 \
	darwin/arm64

# Native system
HOST_OS := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)

# =========================
# Helpers
# =========================

define build_cli
echo "→ CLI $(1)"
GOOS=$(word 1,$(subst /, ,$(1))) \
GOARCH=$(word 2,$(subst /, ,$(1))) \
CGO_ENABLED=0 \
$(GO) build -ldflags="$(LDFLAGS)" \
-o $(BUILD_DIR)/$(APP_CLI)-$(word 1,$(subst /, ,$(1)))-$(word 2,$(subst /, ,$(1)))$(if $(findstring windows,$(1)),.exe,) \
main.go
endef

define build_gui
echo "→ GUI $(1)"
GOOS=$(word 1,$(subst /, ,$(1))) \
GOARCH=$(word 2,$(subst /, ,$(1))) \
CGO_ENABLED=1 \
$(GO) build -tags $(GUI_TAGS) \
-ldflags="$(LDFLAGS) $(if $(findstring windows,$(1)),-H=windowsgui,)" \
-o $(BUILD_DIR)/$(APP_GUI)-$(word 1,$(subst /, ,$(1)))-$(word 2,$(subst /, ,$(1)))$(if $(findstring windows,$(1)),.exe,) \
gui.go
endef

# =========================
# Targets
# =========================

all: clean build

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Main build
build: $(BUILD_DIR)
	@echo "=== CLI (All Platforms) ==="
	@$(foreach platform,$(PLATFORMS),$(call build_cli,$(platform));)

	@echo "=== GUI (Native Only) ==="
	@$(call build_gui,$(HOST_OS)/$(HOST_ARCH))

	@echo "✅ Build complete"

# Native GUI only
build-gui:
	@$(call build_gui,$(HOST_OS)/$(HOST_ARCH))

# Clean
clean:
	@rm -rf $(BUILD_DIR)

.PHONY: all build clean build-gui