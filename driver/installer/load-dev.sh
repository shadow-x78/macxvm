#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Development Load (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

BUILD="${1:-}"
IDENTITY="com.macxvm.mxrm"
DEVELOPMENT="com.apple.development.driver-override"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the development load runs inside a macOS guest" >&2
    exit 1
fi

if [ -z "$BUILD" ]; then
    echo "usage: load-dev.sh BUILD_DIR" >&2
    exit 2
fi

if [ ! -d "$BUILD" ]; then
    echo "the build directory $BUILD is missing" >&2
    exit 1
fi

if ! ls "$BUILD"/*.kext >/dev/null 2>&1; then
    echo "the build directory holds no kext" >&2
    exit 1
fi

for kext in "$BUILD"/*.kext; do
    name="$(basename "$kext" .kext)"
    echo "loading $name"
    if ! kextload -C "$(id -u)" "$kext"; then
        echo "the load of $name failed; the recovery display stays available" >&2
        exit 1
    fi
done

if [ ! -f "/Library/Extensions/$IDENTITY.plist" ]; then
    echo "the development identity of the driver is missing; the production package does not inherit it" >&2
    exit 1
fi

echo "the development kexts are loaded"
echo "the auxiliary kernel collection is a separate path and is not touched here"
