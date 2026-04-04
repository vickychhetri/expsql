# Makefile for building MySQL tools for Windows

# Variables
GO := go
BUILD_DIR := windows_build
TARGET_CLI := $(BUILD_DIR)/mysqltool.exe
TARGET_GUI := $(BUILD_DIR)/exsql.exe

# Go build flags
LDFLAGS := -s -w
GUI_TAGS := gui
WINDOWS_GUI_FLAG := -H=windowsgui

# Default target
all: build-cli build-gui

# Create build directory
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Build CLI tool
build-cli: $(BUILD_DIR)
	@echo "Building CLI tool for Windows..."
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(TARGET_CLI) main.go
	@echo "✅ $(TARGET_CLI) built successfully."

# Build GUI tool
build-gui: $(BUILD_DIR)
	@echo "Building GUI tool for Windows..."
	GOOS=windows GOARCH=amd64 $(GO) build -tags $(GUI_TAGS) -ldflags="$(LDFLAGS) $(WINDOWS_GUI_FLAG)" -o $(TARGET_GUI) gui.go
	@echo "✅ $(TARGET_GUI) built successfully."

# Clean binaries
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean done."

.PHONY: all build-cli build-gui clean