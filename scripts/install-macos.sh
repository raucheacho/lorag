#!/bin/bash
#
# Lorag Installer for macOS
#

set -e

APP_NAME="Lorag"
INSTALL_DIR="/Applications"
BIN_DIR="/usr/local/bin"

echo "============================================"
echo "     Installation de Lorag pour macOS"
echo "============================================"
echo ""

# Check architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    DOWNLOAD_URL="https://github.com/lorag/lorag/releases/latest/download/lorag-darwin-arm64.tar.gz"
    echo "Architecture détectée: Apple Silicon (M1/M2/M3)"
elif [ "$ARCH" = "x86_64" ]; then
    DOWNLOAD_URL="https://github.com/lorag/lorag/releases/latest/download/lorag-darwin-amd64.tar.gz"
    echo "Architecture détectée: Intel"
else
    echo "Architecture non supportée: $ARCH"
    exit 1
fi

# Download
echo ""
echo "Téléchargement de Lorag..."
curl -L -o /tmp/lorag.tar.gz "$DOWNLOAD_URL"

# Extract
echo "Extraction..."
tar -xzf /tmp/lorag.tar.gz -C /tmp

# Install binaries
echo "Installation des binaires..."
sudo mkdir -p "$BIN_DIR"
sudo cp /tmp/lorag "$BIN_DIR/"
sudo cp /tmp/loragctl "$BIN_DIR/"
sudo chmod +x "$BIN_DIR/lorag"
sudo chmod +x "$BIN_DIR/loragctl"

# Create app bundle
echo "Création de l'application..."
APP_PATH="$INSTALL_DIR/$APP_NAME.app"

sudo mkdir -p "$APP_PATH/Contents/MacOS"
sudo mkdir -p "$APP_PATH/Contents/Resources"

# Create Info.plist
cat > /tmp/Info.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>lorag-launcher</string>
    <key>CFBundleIdentifier</key>
    <string>dev.lorag.app</string>
    <key>CFBundleName</key>
    <string>Lorag</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>0.1.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.14</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

sudo cp /tmp/Info.plist "$APP_PATH/Contents/"

# Create launcher script
cat > /tmp/lorag-launcher <<'EOF'
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
/usr/local/bin/lorag server &
sleep 2
open "http://localhost:8080"
EOF

sudo cp /tmp/lorag-launcher "$APP_PATH/Contents/MacOS/"
sudo chmod +x "$APP_PATH/Contents/MacOS/lorag-launcher"

# Cleanup
rm -f /tmp/lorag.tar.gz /tmp/lorag /tmp/loragctl /tmp/Info.plist /tmp/lorag-launcher

# Create data directory
mkdir -p "$HOME/Library/Application Support/Lorag"

echo ""
echo "============================================"
echo "     Installation terminée !"
echo "============================================"
echo ""
echo "Lorag est installé dans :"
echo "  - Application: $APP_PATH"
echo "  - CLI: /usr/local/bin/lorag"
echo ""
echo "Pour démarrer :"
echo "  - Double-cliquez sur l'app Lorag dans Applications"
echo "  - Ou tapez 'lorag server' dans le terminal"
echo ""
echo "Données stockées dans :"
echo "  ~/Library/Application Support/Lorag"
echo ""

# Check if Ollama is installed
if ! command -v ollama &> /dev/null; then
    echo "⚠️  Ollama n'est pas installé."
    echo "   Pour une utilisation locale, installez Ollama :"
    echo "   https://ollama.com"
    echo ""
fi

echo "Documentation : https://github.com/lorag/lorag"
echo ""
