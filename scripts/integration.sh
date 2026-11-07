#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Optional KVM Integration Test (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
set -euo pipefail
if [[ "${MACXVM_INTEGRATION:-0}" != 1 ]]; then
  echo "refusing to run: set MACXVM_INTEGRATION=1 to opt in to KVM guest changes" >&2
  exit 2
fi
BASE="${1:?usage: MACXVM_INTEGRATION=1 integration.sh SEALED_BASE}"
BIN="${MACXVM_BIN:-./bin/macxvm}"
[ -x "$BIN" ] || { echo "missing macxvm binary: $BIN" >&2; exit 2; }
N="integration-$$"
cleanup() {
  "$BIN" stop "$N" >/dev/null 2>&1 || true
  "$BIN" delete "$N" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM
[[ -e /dev/kvm ]] || { echo "/dev/kvm is unavailable" >&2; exit 6; }
command -v qemu-system-x86_64 >/dev/null || { echo "qemu-system-x86_64 is missing" >&2; exit 6; }
command -v qemu-img >/dev/null || { echo "qemu-img is missing" >&2; exit 6; }
"$BIN" create "$N" --base "$BASE"
"$BIN" start "$N" --wait
"$BIN" exec "$N" -- sudo -n true
"$BIN" exec "$N" -- test -d /System/Applications/Safari.app
"$BIN" exec "$N" -- test -d /System/Applications/Utilities.app
"$BIN" exec "$N" -- defaults read NSGlobalDomain AppleInterfaceStyle || echo "the guest is in the light mode"
"$BIN" exec "$N" -- df -h /
"$BIN" stop "$N"
"$BIN" start "$N" --wait
"$BIN" exec "$N" -- sudo -n true
"$BIN" stop "$N"
"$BIN" delete "$N" --yes
trap - EXIT INT TERM
echo "integration passed: $N was cloned, booted, checked, restarted and deleted"
