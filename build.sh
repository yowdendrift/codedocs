#!/bin/sh
# Builds the codedocs binaries. Needs Go 1.22+.
#   ./build.sh          this machine only, into skills/codedocs/bin
#   ./build.sh all      Windows, macOS (Intel + Apple silicon), Linux (x64 + arm64), into skills/codedocs/bin
#   ./build.sh dist     all five, named as release assets, into dist/ with SHA256SUMS
set -e
cd "$(dirname "$0")"
v=$(tr -d ' \r\n' < skills/codedocs/VERSION)
targets="windows/amd64 darwin/arm64 darwin/amd64 linux/amd64 linux/arm64"
build() { # GOOS GOARCH OUTPUT
  (cd tool && CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w -X main.version=$v" -o "../$3" .)
  echo "built $3 ($v)"
}
local_name() { if [ "$1" = windows ]; then echo codedocs.exe; else echo "codedocs-$1-$2"; fi; }

case "$1" in
  dist)
    rm -rf dist && mkdir dist
    for t in $targets; do
      os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=.exe
      build "$os" "$arch" "dist/codedocs-$os-$arch$ext"
    done
    (cd dist && if command -v sha256sum >/dev/null 2>&1; then sha256sum codedocs-*; else shasum -a 256 codedocs-*; fi > SHA256SUMS)
    echo "wrote dist/SHA256SUMS"
    ;;
  *)
    # Stop any running viewers first (a running binary can't be replaced on Windows).
    CODEDOCS_NO_DOWNLOAD=1 sh skills/codedocs/bin/codedocs stop --all >/dev/null 2>&1 || true
    if [ "$1" = all ]; then list=$targets; else list="$(go env GOOS)/$(go env GOARCH)"; fi
    for t in $list; do
      os=${t%/*}; arch=${t#*/}
      build "$os" "$arch" "skills/codedocs/bin/$(local_name "$os" "$arch")"
    done
    ;;
esac
