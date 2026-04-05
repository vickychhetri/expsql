# =========================
# Multi-OS Go Build Makefile
# =========================

GO := go
BUILD_DIR := build

APP_CLI := mysqltool
APP_GUI := exsql

# Common flags
LDFLAGS := -s -w
GUI_TAGS := gui

# Platforms
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	windows/amd64 \
	darwin/amd64 \
	darwin/arm64

# =========================
# Helpers
# =========================

define build_cli
	@echo "→ Building CLI for $(1)"
	@GOOS=$(word 1,$(subst /, ,$(1))) \
	GOARCH=$(word 2,$(subst /, ,$(1))) \
	$(GO) build -ldflags="$(LDFLAGS)" \
	-o $(BUILD_DIR)/$(APP_CLI)-$(word 1,$(subst /, ,$(1)))-$(word 2,$(subst /, ,$(1)))$(if $(findstring windows,$(1)),.exe,) \
	main.go
endef

define build_gui
	@echo "→ Building GUI for $(1)"
	@GOOS=$(word 1,$(subst /, ,$(1))) \
	GOARCH=$(word 2,$(subst /, ,$(1))) \
	$(GO) build -tags $(GUI_TAGS) \
	-ldflags="$(LDFLAGS) $(if $(findstring windows,$(1)),-H=windowsgui,)" \
	-o $(BUILD_DIR)/$(APP_GUI)-$(word 1,$(subst /, ,$(1)))-$(word 2,$(subst /, ,$(1)))$(if $(findstring windows,$(1)),.exe,) \
	gui.go
endef

# =========================
# Targets
# =========================

all: clean build-all

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

build-all: $(BUILD_DIR)
	@echo "=== Building for all platforms ==="
	$(foreach platform,$(PLATFORMS),$(call build_cli,$(platform));)
	$(foreach platform,$(PLATFORMS),$(call build_gui,$(platform));)
	@echo "✅ All builds completed."

# Individual OS builds

build-linux: $(BUILD_DIR)
	$(call build_cli,linux/amd64)
	$(call build_gui,linux/amd64)

build-windows: $(BUILD_DIR)
	$(call build_cli,windows/amd64)
	$(call build_gui,windows/amd64)

build-mac: $(BUILD_DIR)
	$(call build_cli,darwin/amd64)
	$(call build_gui,darwin/amd64)

# Clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: all build-all build-linux build-windows build-mac clean