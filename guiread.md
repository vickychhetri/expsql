FEDORA


# Install required development libraries for Fyne GUI
sudo dnf install -y gcc golang libX11-devel libXcursor-devel libXrandr-devel \
    libXinerama-devel mesa-libGL-devel libXi-devel libXxf86vm-devel \
    libXxf86dga-devel wayland-devel

# Also install these additional dependencies
sudo dnf install -y alsa-lib-devel pulseaudio-libs-devel

# For MySQL client library (optional, for better MySQL support)
sudo dnf install -y mysql-devel





 

# Update Fyne to latest version
go get fyne.io/fyne/v2@latest
go mod tidy

# If you get CGO errors on Linux
sudo apt-get install gcc libgl1-mesa-dev xorg-dev

# For Fedora/RHEL
sudo dnf install gcc libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel mesa-libGL-devel

# For macOS
xcode-select --install

# For Windows (with MSYS2)
# Install mingw-w64



# Install dependencies
go mod download
go mod tidy

# Build CLI version
go build -o mysqltool main.go

# Build GUI version (requires CGO for Fyne)
CGO_ENABLED=1 go build -tags gui -o exsql gui.go

# Run CLI
./mysqltool export --database testdb

# Run GUI
./mysqltool-gui

# For production builds with optimizations
go build -ldflags="-s -w" -o mysqltool main.go
go build -tags gui -ldflags="-s -w" -o  exsql gui.go

# Cross-platform build for GUI (example for Windows from Linux)
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -tags gui -o mysqltool-gui.exe gui.go




go build -tags gui -ldflags="-s -w" -o exsql gui.go









DOCKER

# Dockerfile
FROM golang:1.25.8-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git gcc musl-dev linux-headers

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build CLI
RUN go build -o mysqltool main.go

# Build GUI (requires X11 for GUI, so CLI only in Docker)
# RUN go build -tags gui -o mysqltool-gui gui.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/mysqltool .

EXPOSE 3306

ENTRYPOINT ["./mysqltool"]









LINUX

#!/bin/bash
# install.sh

echo "Installing MySQL DataStream..."

# Build the application
make build

# Create directory structure
sudo mkdir -p /usr/local/mysqltool
sudo mkdir -p /etc/mysqltool

# Copy binaries
sudo cp mysqltool /usr/local/bin/
sudo cp mysqltool-gui /usr/local/bin/

# Create desktop entry for GUI
cat > ~/.local/share/applications/mysqltool.desktop << EOF
[Desktop Entry]
Version=1.0
Name=MySQL DataStream
Comment=High Performance MySQL Database Tool
Exec=/usr/local/bin/mysqltool-gui
Icon=applications-development
Terminal=false
Type=Application
Categories=Development;Database;
StartupNotify=true
EOF

# Create man page (optional)
sudo mkdir -p /usr/local/share/man/man1
# Copy man page if exists

echo "Installation complete!"
echo "CLI: mysqltool"
echo "GUI: mysqltool-gui"