#!/bin/bash
# bazNTMS agent — macOS pkg uretimi (Faz 7.3).
# Kullanim (macOS runner'da):
#   deploy/macos/build-pkg.sh <exe> <out-pkg> <version>
#   deploy/macos/build-pkg.sh bazntms-agent-darwin-arm64 bazntms-agent-arm64.pkg 0.2.0
# Imza: MACOS_DEVELOPER_ID imza kimligi varsa pkgbuild --sign kullanir.
set -eu

EXE="${1:?kullanim: build-pkg.sh <exe> <out-pkg> <version>}"
OUT="${2:?kullanim: build-pkg.sh <exe> <out-pkg> <version>}"
VERSION="${3:?kullanim: build-pkg.sh <exe> <out-pkg> <version>}"

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(mktemp -d)"
trap 'rm -rf "$ROOT"' EXIT

mkdir -p "$ROOT/pkgroot/usr/local/bin" "$ROOT/pkgroot/usr/local/share/bazntms" \
    "$ROOT/pkgroot/Library/LaunchDaemons" "$ROOT/scripts"
cp "$EXE" "$ROOT/pkgroot/usr/local/bin/bazntms-agent"
chmod 755 "$ROOT/pkgroot/usr/local/bin/bazntms-agent"
cp "$HERE/../config/bazntms-agent.yml.example" "$ROOT/pkgroot/usr/local/share/bazntms/"
cp "$HERE/local.bazntms.agent.plist" "$ROOT/pkgroot/Library/LaunchDaemons/"
cp "$HERE/postinstall" "$ROOT/scripts/postinstall"
chmod +x "$ROOT/scripts/postinstall"

SIGN_ARGS=()
if [ -n "${MACOS_DEVELOPER_ID:-}" ]; then
    SIGN_ARGS=(--sign "$MACOS_DEVELOPER_ID")
fi

pkgbuild ${SIGN_ARGS[@]+"${SIGN_ARGS[@]}"} \
    --root "$ROOT/pkgroot" \
    --scripts "$ROOT/scripts" \
    --identifier "local.bazntms.agent" \
    --version "$VERSION" \
    --install-location "/" \
    "$OUT"

echo "[pkg] uretildi: $OUT"
