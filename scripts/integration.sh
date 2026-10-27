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
N="integration-$$"
cleanup() {
  macxvm stop "$N" >/dev/null 2>&1 || true
  macxvm delete "$N" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
[[ -e /dev/kvm ]] || { echo "/dev/kvm is unavailable" >&2; exit 6; }
command -v qemu-system-x86_64 >/dev/null
command -v qemu-img >/dev/null
macxvm create "$N" --base "$BASE"
macxvm start "$N" --wait
macxvm exec "$N" -- sudo -n true
macxvm exec "$N" -- test -d /System/Applications/Safari.app
macxvm exec "$N" -- test -d /System/Applications/Utilities.app
macxvm exec "$N" -- defaults read NSGlobalDomain AppleInterfaceStyle
macxvm exec "$N" -- df -h /
macxvm stop "$N"
macxvm start "$N" --wait
macxvm exec "$N" -- sudo -n true
macxvm stop "$N"
macxvm delete "$N" --yes
trap - EXIT INT TERM
echo "integration passed: $N was cloned, booted, checked, restarted and deleted"
