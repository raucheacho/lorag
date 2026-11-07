#!/bin/sh
# Installs lorag and loragctl from the GitHub releases:
#   curl -fsSL https://raw.githubusercontent.com/raucheacho/lorag/main/scripts/install.sh | sh
#   ... | sh -s -- --service     also start Lorag with the session (systemd, Linux)
#
# Environment:
#   LORAG_VERSION       version to install (default: the latest release)
#   LORAG_INSTALL_DIR   where the binaries go (default: ~/.local/bin)
#   LORAG_DOWNLOAD_URL  a mirror holding the archives and SHA256SUMS
set -eu

repo=${LORAG_REPO:-raucheacho/lorag}
dir=${LORAG_INSTALL_DIR:-$HOME/.local/bin}
service=false
for arg in "$@"; do
    case "$arg" in
        --service) service=true ;;
        *) echo "unknown option: $arg" >&2; exit 2 ;;
    esac
done

fail() { echo "error: $*" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || fail "curl is required"

case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail "unsupported system: $(uname -s) (Lorag runs on macOS and Linux)" ;;
esac
case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

version=${LORAG_VERSION:-}
if [ -z "$version" ]; then
    version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
        sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
    [ -n "$version" ] || fail "cannot find the latest release of $repo"
fi
version=${version#v}

name="lorag_${version}_${os}_${arch}"
base=${LORAG_DOWNLOAD_URL:-https://github.com/$repo/releases/download/v$version}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading Lorag $version ($os/$arch)"
curl -fsSL -o "$tmp/$name.tar.gz" "$base/$name.tar.gz" || fail "download failed: $base/$name.tar.gz"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS" || fail "download failed: $base/SHA256SUMS"

expected=$(awk -v f="$name.tar.gz" '$2 == f { print $1 }' "$tmp/SHA256SUMS")
if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$tmp/$name.tar.gz" | cut -d' ' -f1)
else
    actual=$(shasum -a 256 "$tmp/$name.tar.gz" | cut -d' ' -f1)
fi
[ -n "$expected" ] && [ "$expected" = "$actual" ] || fail "checksum mismatch for $name.tar.gz"

tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
mkdir -p "$dir"
install -m 755 "$tmp/$name/lorag" "$tmp/$name/loragctl" "$dir/"
echo "Installed lorag and loragctl in $dir"

case ":$PATH:" in
    *":$dir:"*) ;;
    *) echo "Add $dir to your PATH, e.g.: echo 'export PATH=\"$dir:\$PATH\"' >> ~/.profile" ;;
esac

if $service; then
    [ "$os" = linux ] || fail "--service is for Linux; on macOS use Homebrew: brew services start lorag"
    command -v systemctl >/dev/null 2>&1 || fail "systemd is not available"
    unit="$HOME/.config/systemd/user/lorag.service"
    mkdir -p "$(dirname "$unit")"
    cat >"$unit" <<EOF
[Unit]
Description=Lorag, local memory for AI agents
After=network.target

[Service]
ExecStart=$dir/lorag server
Restart=on-failure

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
    systemctl --user enable --now lorag
    echo "Lorag runs as a user service: systemctl --user status lorag"
fi

cat <<EOF

Next:
  ollama pull bge-m3        # embeddings (https://ollama.com)
  sudo apt install poppler-utils ffmpeg tesseract-ocr   # PDF pages, audio, video, OCR (Debian, Ubuntu)
  lorag server              # unless installed with --service
  lorag open                # the Studio, http://localhost:8080
Data is stored in ~/.lorag/data.
EOF
