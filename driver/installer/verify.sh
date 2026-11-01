#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Package Verification (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PACKAGE="${1:-}"
PUBLIC_KEY="$ROOT/resources/mxgpu-release-public-key.txt"

if [ -z "$PACKAGE" ]; then
    echo "usage: verify.sh PACKAGE" >&2
    exit 2
fi

if [ ! -f "$PACKAGE" ]; then
    echo "the package $PACKAGE is missing" >&2
    exit 1
fi

if [ ! -f "$PUBLIC_KEY" ]; then
    echo "the public key of the release is missing" >&2
    exit 1
fi

if [ -x "$ROOT/bin/macxvm" ]; then
    exec "$ROOT/bin/macxvm" driver verify "$PACKAGE"
fi

echo "the binary of macxvm is not built; run 'make build' and retry" >&2
exit 1
