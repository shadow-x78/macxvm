#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Benchmark Script (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
set -euo pipefail
BASE="${1:?usage: bench.sh BASE_VM}"
N="bench-$$"
cleanup() {
  macxvm stop "$N" >/dev/null 2>&1 || true
  macxvm delete "$N" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
seconds() {
  local started=$SECONDS
  "$@" >/dev/null
  echo $((SECONDS - started))
}
printf 'clone_seconds %s\n' "$(seconds macxvm clone "$BASE" "$N")"
printf 'start_to_ssh_seconds %s\n' "$(seconds macxvm start "$N" --wait)"
macxvm stop "$N" >/dev/null
macxvm delete "$N" --yes >/dev/null
printf 'run_true_seconds %s\n' "$(seconds macxvm run --from "$BASE" -- true)"
