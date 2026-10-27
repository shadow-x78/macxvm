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
run_timed() {
  local label=$1
  shift
  local started=$SECONDS
  if ! "$@" >/dev/null; then
    echo "benchmark failed: $label" >&2
    return 1
  fi
  printf '%s_seconds %s\n' "$label" "$((SECONDS - started))"
}
run_timed clone "$BIN" clone "$BASE" "$N"
run_timed start_to_ssh "$BIN" start "$N" --wait
"$BIN" stop "$N" >/dev/null
"$BIN" delete "$N" --yes >/dev/null
run_timed run_true "$BIN" run --from "$BASE" -- true
