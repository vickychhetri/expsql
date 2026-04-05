Mysql Data Stream

# Build Guide 

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



# Development Guide

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



FEDORA Development 
# Install required development libraries for Fyne GUI
sudo dnf install -y gcc golang libX11-devel libXcursor-devel libXrandr-devel \
    libXinerama-devel mesa-libGL-devel libXi-devel libXxf86vm-devel \
    libXxf86dga-devel wayland-devel

# Also install these additional dependencies
sudo dnf install -y alsa-lib-devel pulseaudio-libs-devel








**Dashboard**
<img width="1920" height="1200" alt="dashboard" src="https://github.com/user-attachments/assets/7d0e9069-df2c-4282-a96a-a9b717a8dddd" />

**Export**
<img width="1920" height="1200" alt="export" src="https://github.com/user-attachments/assets/1cf18304-b2cb-4d38-89c3-061c19a3f045" />

**Import**
<img width="1920" height="1200" alt="import" src="https://github.com/user-attachments/assets/ceb234a6-da9a-44b0-9396-a4cadf54476d" />

**Settings**
<img width="1920" height="1200" alt="setting" src="https://github.com/user-attachments/assets/5ce77473-cbac-41b1-8b86-05e2fb55ff09" />

**Selector**
<img width="1920" height="1200" alt="selector" src="https://github.com/user-attachments/assets/6b0e0799-eb2d-421a-a3e0-862fddc1e0d1" />
