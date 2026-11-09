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
STAMP_FILE="$DIST/mxgpu-staging.stamp"
ARCHIVE="$DIST/mxgpu-$KEY_ID.tar.gz"

build_stamp() {
    find "$BUILD" -type f -print0 |
        sort -z |
        xargs -0 shasum -a 256 |
        shasum -a 256 |
        awk '{print $1}'
}

STAMP="$(build_stamp)"

if [ -f "$STAGING/manifest.sig" ]; then
    if [ ! -f "$STAMP_FILE" ] || [ "$(cat "$STAMP_FILE")" != "$STAMP" ]; then
        echo "the staging directory holds the signature of a different build" >&2
        echo "remove $STAGING before the package of a new build, or run 'make -C driver clean'" >&2
        exit 1
    fi
else
    rm -rf "$STAGING"
    mkdir -p "$STAGING"

    cp -R "$BUILD/." "$STAGING/"
    cp "$SBOM" "$STAGING/SBOM.json"
    cp "$DRIVER/NOTICE" "$STAGING/NOTICE"
    cp "$DRIVER/third_party.lock.json" "$STAGING/third_party.lock.json"
    cp "$DRIVER/installer/install.sh" "$DRIVER/installer/verify.sh" "$DRIVER/installer/uninstall.sh" "$STAGING/"
    printf '%s\n' "$STAMP" >"$STAMP_FILE"
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
