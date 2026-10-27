#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Benchmark Script (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
set -euo pipefail
BASE="${1:?usage: bench.sh BASE_VM}"
BIN="${MACXVM_BIN:-./bin/macxvm}"
[ -x "$BIN" ] || { echo "missing macxvm binary: $BIN" >&2; exit 2; }
N="bench-$$"
cleanup() {
  "$BIN" stop "$N" >/dev/null 2>&1 || true
  "$BIN" delete "$N" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
seconds() {
  local started=$SECONDS
  "$@" >/dev/null
  echo $((SECONDS - started))
}
printf 'clone_seconds %s\n' "$(seconds "$BIN" clone "$BASE" "$N")"
printf 'start_to_ssh_seconds %s\n' "$(seconds "$BIN" start "$N" --wait)"
"$BIN" stop "$N" >/dev/null
"$BIN" delete "$N" --yes >/dev/null
printf 'run_true_seconds %s\n' "$(seconds "$BIN" run --from "$BASE" -- true)"
