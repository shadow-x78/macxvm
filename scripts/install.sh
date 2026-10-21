#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - Install Script (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
#
# Builds MacXVM and installs it into ~/.local/bin.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local/bin}"

if ! command -v go >/dev/null 2>&1; then
    echo "go is not in PATH, install Go 1.22 or newer first" >&2
    exit 1
fi

echo "building macxvm..."
cd "$ROOT"
make build

echo "installing into $PREFIX..."
mkdir -p "$PREFIX"
install -m 0755 bin/macxvm "$PREFIX/macxvm"

echo "installed: $PREFIX/macxvm"
echo
echo "next steps:"
echo "  1. macxvm doctor"
echo "  2. macxvm create demo --args-file ./args.txt --assets ./assets"
