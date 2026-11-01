#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Removal (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

TARGET="${1:-/Library/Extensions}"
BACKUP="${2:-}"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the driver is removed from a macOS guest" >&2
    exit 1
fi

for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
    if [ -e "$TARGET/$item" ]; then
        rm -rf "$TARGET/$item"
        echo "removed $item"
    fi
done

if [ -f "$TARGET/macxvm-installed-manifest.json" ]; then
    rm -f "$TARGET/macxvm-installed-manifest.json"
fi

if [ -n "$BACKUP" ] && [ -d "$BACKUP" ]; then
    for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
        if [ -e "$BACKUP/$item" ]; then
            cp -R "$BACKUP/$item" "$TARGET/"
            echo "restored $item"
        fi
    done
fi

echo "the driver is removed"
