#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Probe Runtime Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

# Builds a QEMU that carries the probe device of driver/qemu/apple-gfx-probe.c.
# The device announces the identity of the Apple paravirtual graphics adapter
# and logs every register access of the guest, so the binding of the driver of
# macOS can be measured without the framework of Apple, which exists on macOS
# hosts only.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE="${MXGPU_QEMU_VERSION:-10.2.2}"
WORK="${MXGPU_QEMU_WORK:-$HOME/.local/share/macxvm/qemu-work}"
RUNTIME="${MXGPU_RUNTIME:-$HOME/.local/share/macxvm/reims}"
PATCH="$ROOT/driver/qemu/qemu-apple-gfx-probe.patch"

if [ ! -d "$WORK" ]; then
    mkdir -p "$WORK"
fi

cd "$WORK"

if [ ! -d "qemu-$SOURCE" ]; then
    echo "fetching the source of QEMU $SOURCE..."
    curl -fsSL -o "qemu-$SOURCE.tar.xz" "https://download.qemu.org/qemu-$SOURCE.tar.xz"
    tar -xf "qemu-$SOURCE.tar.xz"
fi

cd "qemu-$SOURCE"

if ! grep -q "apple-gfx-probe" hw/display/meson.build 2>/dev/null; then
    echo "applying the patch of the probe device..."
    patch -p1 <"$PATCH"
fi

mkdir -p build
cd build

if [ ! -f build.ninja ]; then
    echo "configuring the build..."
    ../configure \
        --target-list=x86_64-softmmu \
        --enable-slirp \
        --disable-werror \
        --disable-docs \
        --disable-tools \
        --disable-guest-agent
fi

echo "building the QEMU of the probe..."
ninja qemu-system-x86_64

echo "installing the runtime..."
mkdir -p "$RUNTIME/bin" "$RUNTIME/share"
install -m 0755 qemu-system-x86_64 "$RUNTIME/bin/qemu-system-x86_64"

# The data of the build is a set of links to the tree of the build, so the
# copies are dereferenced: the runtime outlives the tree of the build.
BIOS="$WORK/qemu-$SOURCE/pc-bios"
rm -f "$RUNTIME/bin/"*.bin "$RUNTIME/bin/"*.rom 2>/dev/null || true
for rom in "$BIOS"/*.bin "$BIOS"/*.rom; do
    [ -e "$rom" ] || continue
    cp -L "$rom" "$RUNTIME/bin/" 2>/dev/null || true
done

if [ -d "$BIOS/keymaps" ]; then
    rm -rf "$RUNTIME/bin/keymaps"
    cp -rL "$BIOS/keymaps" "$RUNTIME/bin/"
fi

echo
echo "the runtime of the probe is at $RUNTIME"
echo "  qemu: $RUNTIME/bin/qemu-system-x86_64"
echo "  use it with: macxvm create NAME --gpu-backend reims --reims-runtime $RUNTIME"
