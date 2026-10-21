#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Header Check (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
#
# Every source file of the project carries the header of the project. This
# script fails when a file misses it.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GIT_URL="https://github.com/shadow-x78/macxvm"

failed=0

check_go() {
    local file="$1"
    local head
    head="$(head -n 2 "$file")"
    if ! grep -q "^// MacXVM - " <<<"$head"; then
        echo "missing header: $file" >&2
        failed=1
        return
    fi
    if ! grep -q "$GIT_URL" <<<"$head"; then
        echo "missing the URL in the header: $file" >&2
        failed=1
    fi
}

check_shell() {
    local file="$1"
    local head
    head="$(head -n 4 "$file")"
    if ! grep -q "^# MacXVM - " <<<"$head"; then
        echo "missing header: $file" >&2
        failed=1
        return
    fi
    if ! grep -q "$GIT_URL" <<<"$head"; then
        echo "missing the URL in the header: $file" >&2
        failed=1
    fi
}

while IFS= read -r file; do
    check_go "$file"
done < <(find "$ROOT" -name '*.go' -not -path "$ROOT/.git/*" | sort)

while IFS= read -r file; do
    check_shell "$file"
done < <(find "$ROOT" -name '*.sh' -not -path "$ROOT/.git/*" | sort)

if [ "$failed" -ne 0 ]; then
    echo "some files miss the header of the project" >&2
    exit 1
fi

echo "every source file carries the header of the project"
