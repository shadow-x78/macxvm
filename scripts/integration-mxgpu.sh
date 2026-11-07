#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Hardware Integration (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

if [ "${MACXVM_MXGPU_INTEGRATION:-0}" != "1" ]; then
    echo "the hardware integration needs MACXVM_MXGPU_INTEGRATION=1" >&2
    exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="${1:-}"
GPU="${MXGPU_GPU_BDF:-0000:01:00.0}"
AUDIO="${MXGPU_AUDIO_BDF:-0000:01:00.1}"
HOST_GPU="${MXGPU_HOST_GPU_BDF:-0000:02:00.0}"
NAME="mxgpu-probe-$(date -u +%Y%m%dT%H%M%SZ)"
BIN="$ROOT/bin/macxvm"

if [ -z "$BASE" ]; then
    echo "usage: integration-mxgpu.sh SEALED_BASE" >&2
    exit 2
fi

if [ ! -x "$BIN" ]; then
    echo "the binary of macxvm is not built; run 'make build'" >&2
    exit 1
fi

if ! test -e "/sys/bus/pci/devices/$GPU"; then
    echo "the board $GPU is not in the host" >&2
    exit 1
fi

cleanup() {
    "$BIN" stop "$NAME" --force >/dev/null 2>&1 || true
    "$BIN" gpu --unbind-vfio --gpu "$GPU" --restore-host >/dev/null 2>&1 || true
    "$BIN" delete "$NAME" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM

"$BIN" create "$NAME" --base "$BASE"

"$BIN" gpu --prepare-vfio --gpu "$GPU" --audio "$AUDIO" --host-gpu "$HOST_GPU" --accept-host-changes

for stage in install daily recovery; do
    echo "starting the stage $stage"
    "$BIN" start "$NAME" --display "$stage" --vnc
    "$BIN" status "$NAME" --json
    "$BIN" stop "$NAME"
done

echo "the hardware integration of the MXGPU path finished"
