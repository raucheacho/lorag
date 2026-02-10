#!/bin/bash
#
# Lorag Installation Script
#

set -e

REPO="lorag/lorag"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected OS: $OS, Architecture: $ARCH"

# Get latest release
get_latest_release() {
    curl --silent "https://api.github.com/repos/$REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

VERSION=$(get_latest_release)
echo "Latest version: $VERSION"

# Download binaries
download() {
    local binary=$1
    local url="https://github.com/$REPO/releases/download/$VERSION/${binary}_${OS}_${ARCH}.tar.gz"
    
    echo "Downloading $binary..."
    curl -L -o "/tmp/${binary}.tar.gz" "$url"
    
    echo "Extracting $binary..."
    tar -xzf "/tmp/${binary}.tar.gz" -C /tmp
    
    echo "Installing $binary..."
    sudo mv "/tmp/$binary" "$INSTALL_DIR/"
    sudo chmod +x "$INSTALL_DIR/$binary"
    
    rm "/tmp/${binary}.tar.gz"
}

# Install binaries
install_binaries() {
    download "lorag"
    download "loragctl"
}

# Create data directory
create_data_dir() {
    echo "Creating data directory..."
    sudo mkdir -p /var/lib/lorag
    sudo chmod 755 /var/lib/lorag
}

# Create systemd service (Linux only)
create_service() {
    if [ "$OS" != "linux" ]; then
        return
    fi
    
    echo "Creating systemd service..."
    
    sudo tee /etc/systemd/system/lorag.service > /dev/null <<EOF
[Unit]
Description=Lorag - Local RAG Engine
After=network.target

[Service]
Type=simple
User=lorag
Group=lorag
WorkingDirectory=/var/lib/lorag
Environment=LORAG_DATA_DIR=/var/lib/lorag/data
Environment=LORAG_HOST=0.0.0.0
Environment=LORAG_PORT=8080
ExecStart=/usr/local/bin/lorag server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    
    # Create lorag user
    if ! id -u lorag &>/dev/null; then
        sudo useradd -r -s /bin/false -d /var/lib/lorag lorag
    fi
    
    sudo chown -R lorag:lorag /var/lib/lorag
    sudo systemctl daemon-reload
    
    echo "Service created. Start with: sudo systemctl start lorag"
    echo "Enable on boot with: sudo systemctl enable lorag"
}

# Main installation
main() {
    echo "Installing Lorag..."
    echo ""
    
    install_binaries
    create_data_dir
    create_service
    
    echo ""
    echo "Installation complete!"
    echo ""
    echo "Verify installation:"
    echo "  lorag version"
    echo "  loragctl version"
    echo ""
    echo "Start the server:"
    echo "  lorag server"
    echo ""
    echo "Or use systemd (Linux):"
    echo "  sudo systemctl start lorag"
}

# Run installation
main
