#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DRIVER="$ROOT/driver"
BUILD="$DRIVER/build"
SDKROOT="${SDKROOT:-macosx}"
ARCHS="${ARCHS:-x86_64}"
SBOM_ONLY=0
RUN_TESTS=0

for arg in "$@"; do
    case "$arg" in
        --test)
            RUN_TESTS=1
            ;;
        --sbom)
            SBOM_ONLY=1
            ;;
        --help|-h)
            echo "usage: build.sh [--test] [--sbom]"
            exit 0
            ;;
        *)
            echo "unknown option: $arg" >&2
            exit 2
            ;;
    esac
done

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the MXGPU kexts are built on a macOS builder only; run 'macxvm driver build --builder USER@HOST'" >&2
    exit 1
fi

if [ ! -d "$DRIVER/kexts/MXRM" ]; then
    echo "the kext sources are not in the repository yet" >&2
    exit 1
fi

mkdir -p "$BUILD"

if [ "$SBOM_ONLY" -eq 1 ]; then
    echo "the SBOM is written from driver/third_party.lock.json"
    exec "$DRIVER/installer/package.sh" --sbom-only
fi

if [ ! -d "$(xcrun --sdk "$SDKROOT" --show-sdk-path 2>/dev/null)" ]; then
    echo "the SDK $SDKROOT is not installed on this builder" >&2
    exit 1
fi

export ARCHS SDKROOT

for kext in MXRM MXFB MXAccel MXAGDC; do
    source_dir="$DRIVER/kexts/$kext"
    if [ ! -f "$source_dir/Makefile" ]; then
        echo "the kext $kext has no build rules yet" >&2
        exit 1
    fi
    echo "building $kext for $ARCHS"
    make -C "$source_dir" ARCHS="$ARCHS" SDKROOT="$SDKROOT" BUILD_DIR="$BUILD/$kext"
done

if [ "$RUN_TESTS" -eq 1 ]; then
    ran=0
    for test_dir in "$DRIVER"/tests/*/; do
        if [ -f "$test_dir/Makefile" ]; then
            echo "running the tests of $(basename "$test_dir")"
            make -C "$test_dir" BUILD_DIR="$BUILD/$(basename "$test_dir")"
            ran=$((ran + 1))
        fi
    done
    if [ "$ran" -eq 0 ]; then
        echo "no driver test was executed; the tests of the driver are not in the repository yet" >&2
        exit 1
    fi
fi

echo "the build of the driver finished"
