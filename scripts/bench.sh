#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Benchmark Script (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
set -euo pipefail
BASE="${1:?usage: bench.sh BASE_VM}"
t() { local s=$(date +%s); "$@" >/dev/null; echo $(( $(date +%s) - s )); }
N="bench-$$"
echo "clone_seconds      $(t macxvm clone "$BASE" "$N")"
echo "start_to_ssh_secs  $(t bash -c "macxvm start $N && macxvm exec $N -- true")"
macxvm stop "$N" >/dev/null; macxvm delete "$N" --yes >/dev/null
echo "run_true_seconds   $(t macxvm run --from "$BASE" -- true)"
