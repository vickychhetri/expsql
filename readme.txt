# 1. Basic parallel export with auto strategy selection
./mysqltool export-parallel \
  --host localhost \
  --port 3306 \
  --user root \
  --password "S" \
  --database funding_manage \
  --output ./backup \
  --workers 8

 ./mysqltool export --host 127.1.184.128 --port 3306 --user teneweweweey --password d7r838ehdhr
   --database ewe2323232 --output ./backup2 --workers 20 --compress --rows-per-batch 500000



./mysqltool export-parallel \
  --host 127.0.0.1 \
  --port 3306 \
  --strategy parallel \
  --user root \
  --password "111" \
  --database 11111 \
  --output ./11111 \
  --workers 12 \
  --partitions 16 \
  --strategy parallel \
  --rows-per-batch 50000
  --bulk-size 1000


  ./mysqltool import \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --password "111111111" \
  --database v1 \
  --input /home/vicky/Desktop/m3 \
  --workers 2




  ./mysqltool export-parallel \
  --host 127.0.0.1 \
  --port 3306 \
  --strategy parallel \
  --user root \
  --password "111111@1111111" \
  --database funding_manage \
  --output ./m122 \
  --workers 12 \
  --partitions 16 \
  --strategy streaming \
  --rows-per-batch 50000
  --bulk-size 1000



# Rebuild
go build -o mysqltool main.go

# Run the export
./mysqltool export \
  --host 1.1.1.1.1 \
  --port 3306 \
  --user e423232332 \
  --password "711111111!" \
  --database 1111 \
  --output ./11111111 \
  --workers 8 \
  --strategy parallel \
  --rows-per-batch 50000
  



# 2. Force parallel export for all tables (good for very large tables)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy parallel \
  --workers 8 \
  --partitions 16 \
  --rows-per-batch 50000

# 3. Streaming export for medium-large tables
./mysqltool export-parallel \
  --database funding_manage \
  --strategy streaming \
  --workers 4 \
  --rows-per-batch 25000

# 4. Resumable export (can be interrupted and resumed)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy auto \
  --resumable true \
  --workers 6 \
  --output ./backup_resumable

# 5. Export only specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --tables users,orders,products,sessions \
  --workers 4

# 6. Export large table with custom partition count
./mysqltool export-parallel \
  --database funding_manage \
  --tables large_table \
  --strategy parallel \
  --workers 8 \
  --partitions 32 \
  --rows-per-batch 100000 \
  --output ./backup_large

# 7. Export with compression for large datasets
./mysqltool export-parallel \
  --database funding_manage \
  --compress true \
  --workers 8 \
  --strategy parallel \
  --output ./backup_compressed

# 8. Design only export (no data)
./mysqltool export-parallel \
  --database funding_manage \
  --include-data false \
  --output ./design_only

# 9. Data only export for specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --include-design false \
  --tables sessions,logs \
  --workers 4 \
  --output ./data_only



┌─────────────────────────────────────────────────────────────┐
│                    MySQL DataStream                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Export    │  │   Import    │  │  Resumable  │        │
│  │  Commands   │  │  Commands   │  │   Export    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│         │               │               │                  │
│  ┌─────────────────────────────────────────────┐          │
│  │         Strategy Selector (Auto)            │          │
│  └─────────────────────────────────────────────┘          │
│         │               │               │                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │  Standard   │  │  Streaming  │  │  Parallel   │        │
│  │  Exporter   │  │  Exporter   │  │  Exporter   │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│         │               │               │                  │
│  ┌─────────────────────────────────────────────┐          │
│  │         MySQL Connection Pool               │          │
│  └─────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────┘

# 1. Basic parallel export with auto strategy selection
./mysqltool export-parallel \
  --host localhost \
  --port 3306 \
  --user root \
  --password "u" \
  --database funding_manage \
  --output ./backup \
  --workers 8

# 2. Force parallel export for all tables (good for very large tables)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy parallel \
  --workers 8 \
  --partitions 16 \
  --rows-per-batch 50000

# 3. Streaming export for medium-large tables
./mysqltool export-parallel \
  --database funding_manage \
  --strategy streaming \
  --workers 4 \
  --rows-per-batch 25000

# 4. Resumable export (can be interrupted and resumed)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy auto \
  --resumable true \
  --workers 6 \
  --output ./backup_resumable

# 5. Export only specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --tables users,orders,products,sessions \
  --workers 4

# 6. Export large table with custom partition count
./mysqltool export-parallel \
  --database funding_manage \
  --tables large_table \
  --strategy parallel \
  --workers 8 \
  --partitions 32 \
  --rows-per-batch 100000 \
  --output ./backup_large

# 7. Export with compression for large datasets
./mysqltool export-parallel \
  --database funding_manage \
  --compress true \
  --workers 8 \
  --strategy parallel \
  --output ./backup_compressed

# 8. Design only export (no data)
./mysqltool export-parallel \
  --database funding_manage \
  --include-data false \
  --output ./design_only

# 9. Data only export for specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --include-design false \
  --tables sessions,logs \
  --workers 4 \
  --output ./data_only





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



# build everything
make

# only linux (fedora/ubuntu)
make build-linux

# only windows
make build-windows

# only mac
make build-mac

# clean
make clean