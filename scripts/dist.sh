#!/bin/sh
# Builds the release archives of Lorag:
#   scripts/dist.sh 0.1.0 darwin_arm64 linux_amd64 ...
# writes dist/lorag_<version>_<os>_<arch>.tar.gz for each platform.
#
# lorag links the VectixDB static library (prebuilt, in the Go module) through
# cgo, so each platform needs a C toolchain for it:
#   darwin_*  Apple clang, on macOS only (the Go runtime links Apple frameworks)
#   linux_*   zig cc, from any host, against glibc 2.28 (Debian 10, Ubuntu 20.04,
#             RHEL 8 and later)
# loragctl is pure Go. The Studio must be built first (ui/dist).
set -eu
cd "$(dirname "$0")/.."

[ $# -ge 2 ] || { echo "usage: $0 <version> <goos_goarch>..." >&2; exit 2; }
version=${1#v}
shift
[ -f ui/dist/index.html ] || { echo "ui/dist is missing: run (cd ui && npm ci && npm run build)" >&2; exit 1; }

commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
ldflags="-s -w -X main.Version=$version -X main.BuildTime=$date -X main.Commit=$commit"

mkdir -p dist
for platform in "$@"; do
    goos=${platform%_*}
    goarch=${platform#*_}
    case "$platform" in
        darwin_arm64) cc="clang -arch arm64 -mmacosx-version-min=13.0" ;;
        darwin_amd64) cc="clang -arch x86_64 -mmacosx-version-min=13.0" ;;
        linux_amd64) cc="zig cc -target x86_64-linux-gnu.2.28" ;;
        linux_arm64) cc="zig cc -target aarch64-linux-gnu.2.28" ;;
        *) echo "unsupported platform: $platform" >&2; exit 1 ;;
    esac
    if [ "$goos" = darwin ] && [ "$(uname -s)" != Darwin ]; then
        echo "$platform must be built on macOS" >&2
        exit 1
    fi

    name="lorag_${version}_${platform}"
    out="dist/$name"
    rm -rf "$out"
    mkdir -p "$out"
    echo "building $name"
    GOOS=$goos GOARCH=$goarch CGO_ENABLED=1 CC="$cc" MACOSX_DEPLOYMENT_TARGET=13.0 \
        go build -trimpath -ldflags "$ldflags" -o "$out/lorag" ./cmd/lorag
    GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 \
        go build -trimpath -ldflags "$ldflags" -o "$out/loragctl" ./cmd/loragctl

    # Lorag is MIT; the VectixDB engine linked into it is Apache-2.0, whose
    # license and notice must ship with the binary.
    cp LICENSE README.md "$out/"
    cp third_party/vectixdb/LICENSE "$out/LICENSE.vectixdb"
    cp third_party/vectixdb/NOTICE "$out/NOTICE.vectixdb"

    # No macOS extended attributes: GNU tar warns about each one.
    xattrs=
    [ "$(uname -s)" = Darwin ] && xattrs=--no-xattrs
    COPYFILE_DISABLE=1 tar $xattrs -czf "$out.tar.gz" -C dist "$name"
    rm -rf "$out"
done

(cd dist && shasum -a 256 lorag_*.tar.gz > SHA256SUMS)
echo "archives and SHA256SUMS in dist/"
