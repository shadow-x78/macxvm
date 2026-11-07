#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Package (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DRIVER="$ROOT/driver"
BUILD="$DRIVER/build"
DIST="$DRIVER/dist"
ARTIFACTS="$ROOT/artifacts/mxgpu/package"
KEY_ID="${1:-mxgpu-release-2026}"
SBOM_ONLY=0

for arg in "$@"; do
    if [ "$arg" = "--sbom-only" ]; then
        SBOM_ONLY=1
    fi
done

if [ ! -f "$DRIVER/third_party.lock.json" ]; then
    echo "the lock file of the third party components is missing" >&2
    exit 1
fi

mkdir -p "$ARTIFACTS"

SBOM="$ARTIFACTS/SBOM.json"
cat >"$SBOM" <<EOF
{
  "schema": "mxgpu-sbom-v1",
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "keyId": "$KEY_ID",
  "components": "$(cat "$DRIVER/third_party.lock.json")"
}
EOF

if [ "$SBOM_ONLY" -eq 1 ]; then
    echo "the SBOM is at $SBOM"
    exit 0
fi

if [ ! -d "$BUILD" ]; then
    echo "the driver is not built; run 'make -C driver build' on the builder" >&2
    exit 1
fi

mkdir -p "$DIST"
STAGING="$DIST/mxgpu-staging"
ARCHIVE="$DIST/mxgpu-$KEY_ID.tar.gz"

if [ ! -f "$STAGING/manifest.sig" ]; then
    if [ ! -d "$BUILD" ]; then
        echo "the driver is not built; run 'make -C driver build' on the builder" >&2
        exit 1
    fi
    rm -rf "$STAGING"
    mkdir -p "$STAGING"

    cp -R "$BUILD/." "$STAGING/"
    cp "$SBOM" "$STAGING/SBOM.json"
    cp "$DRIVER/NOTICE" "$STAGING/NOTICE"
    cp "$DRIVER/third_party.lock.json" "$STAGING/third_party.lock.json"
    cp "$DRIVER/installer/install.sh" "$DRIVER/installer/verify.sh" "$DRIVER/installer/uninstall.sh" "$STAGING/"
fi

(
    cd "$STAGING"
    find . -type f ! -name manifest.json ! -name manifest.sig ! -name SHA256SUMS -print0 |
        sort -z |
        xargs -0 shasum -a 256 >SHA256SUMS
)

tar -czf "$ARCHIVE" -C "$STAGING" .

if [ -f "$STAGING/manifest.sig" ]; then
    echo "the signed package is at $ARCHIVE"
else
    echo "the staging directory of the package is at $STAGING"
    echo "run 'driver/installer/sign-release.sh' to sign it and 'driver/installer/package.sh' again to archive it"
fi
