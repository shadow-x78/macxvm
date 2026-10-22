#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Header Check (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GIT_URL="https://github.com/shadow-x78/macxvm"

failed=0

report() {
    echo "$1" >&2
    failed=1
}

check_source() {
    local file="$1" head
    head="$(head -n 2 "$file")"
    if ! grep -q "^// MacXVM - " <<<"$head"; then
        report "missing the credit banner: ${file#$ROOT/}"
        return
    fi
    if ! grep -q "$GIT_URL" <<<"$head"; then
        report "missing the project URL in the banner: ${file#$ROOT/}"
    fi
}

check_config() {
    local file="$1" head
    head="$(head -n 4 "$file")"
    if ! grep -q "^# MacXVM - " <<<"$head"; then
        report "missing the credit banner: ${file#$ROOT/}"
        return
    fi
    if ! grep -q "$GIT_URL" <<<"$head"; then
        report "missing the project URL in the banner: ${file#$ROOT/}"
    fi
}

check_shell() {
    local file="$1" head
    head="$(head -n 4 "$file")"
    if ! grep -q "^# MacXVM - " <<<"$head"; then
        report "missing the credit banner: ${file#$ROOT/}"
        return
    fi
    if ! grep -q "$GIT_URL" <<<"$head"; then
        report "missing the project URL in the banner: ${file#$ROOT/}"
    fi
}

while IFS= read -r file; do
    check_source "$file"
done < <(find "$ROOT" -name '*.go' -not -path "$ROOT/.git/*" | sort)

while IFS= read -r file; do
    check_shell "$file"
done < <(find "$ROOT" -name '*.sh' -not -path "$ROOT/.git/*" | sort)

while IFS= read -r file; do
    check_config "$file"
done < <(find "$ROOT" -name '*.yml' -not -path "$ROOT/.git/*" | sort)

for file in "$ROOT/.editorconfig" "$ROOT/.gitattributes" "$ROOT/Makefile"; do
    [ -f "$file" ] && check_config "$file"
done

check_source "$ROOT/go.mod"

if [ "$failed" -ne 0 ]; then
    echo "some files miss the credit banner of the project" >&2
    exit 1
fi

echo "every source and configuration file carries the credit banner of the project"
