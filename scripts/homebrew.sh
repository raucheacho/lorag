#!/bin/sh
# Prints the Homebrew formula of a release, from the archives' checksums:
#   scripts/homebrew.sh 0.1.0 dist/SHA256SUMS > lorag.rb
# The formula goes to the tap repository (Formula/lorag.rb).
set -eu

[ $# -eq 2 ] || { echo "usage: $0 <version> <SHA256SUMS>" >&2; exit 2; }
version=${1#v}
sums=$2
repo=${LORAG_REPO:-raucheacho/lorag}
base="https://github.com/$repo/releases/download/v$version"

sha() {
    s=$(awk -v f="lorag_${version}_$1.tar.gz" '$2 == f { print $1 }' "$sums")
    [ -n "$s" ] || { echo "no checksum for $1 in $sums" >&2; exit 1; }
    echo "$s"
}

cat <<EOF
class Lorag < Formula
  desc "Local memory and search for AI agents, over MCP"
  homepage "https://github.com/$repo"
  license "MIT"

  on_macos do
    on_arm do
      url "$base/lorag_${version}_darwin_arm64.tar.gz"
      sha256 "$(sha darwin_arm64)"
    end
    on_intel do
      url "$base/lorag_${version}_darwin_amd64.tar.gz"
      sha256 "$(sha darwin_amd64)"
    end
  end

  on_linux do
    on_arm do
      url "$base/lorag_${version}_linux_arm64.tar.gz"
      sha256 "$(sha linux_arm64)"
    end
    on_intel do
      url "$base/lorag_${version}_linux_amd64.tar.gz"
      sha256 "$(sha linux_amd64)"
    end
  end

  def install
    bin.install "lorag", "loragctl"
    prefix.install_metafiles
    # install_metafiles already took LICENSE.vectixdb; NOTICE.vectixdb is left.
    prefix.install Dir["*.vectixdb"]
  end

  service do
    run [opt_bin/"lorag", "server"]
    keep_alive true
    # pdftotext, ffmpeg, whisper-cli and tesseract are found on this PATH.
    environment_variables PATH: std_service_path_env
    log_path var/"log/lorag.log"
    error_log_path var/"log/lorag.log"
  end

  def caveats
    <<~TEXT
      Lorag needs Ollama for embeddings (https://ollama.com):
        ollama pull bge-m3

      Optional, for PDF pages, audio, video and OCR:
        brew install poppler ffmpeg whisper-cpp tesseract

      Data is stored in ~/.lorag/data. The Studio is on http://localhost:8080.
    TEXT
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/lorag version")
    assert_match version.to_s, shell_output("#{bin}/loragctl version")
  end
end
EOF
