#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Installation (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PACKAGE="${1:-}"
TARGET="${2:-/Library/Extensions}"
BACKUP="$TARGET/macxvm-backup-$(date -u +%Y%m%dT%H%M%SZ)"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the driver is installed in a macOS guest" >&2
    exit 1
fi

if [ -z "$PACKAGE" ]; then
    echo "usage: install.sh PACKAGE [TARGET]" >&2
    exit 2
fi

if [ ! -f "$PACKAGE" ]; then
    echo "the package $PACKAGE is missing" >&2
    exit 1
fi

"$ROOT/driver/installer/verify.sh" "$PACKAGE"

STAGING="$(mktemp -d)"
trap 'rm -rf "$STAGING"' EXIT
tar -xzf "$PACKAGE" -C "$STAGING"

if [ ! -f "$STAGING/manifest.json" ]; then
    echo "the package holds no manifest" >&2
    exit 1
fi

mkdir -p "$BACKUP"
for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
    if [ -e "$TARGET/$item" ]; then
        cp -R "$TARGET/$item" "$BACKUP/"
    fi
done

for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
    if [ -e "$STAGING/$item" ]; then
        rm -rf "$TARGET/$item"
        cp -R "$STAGING/$item" "$TARGET/"
    fi
done

cat >"$TARGET/macxvm-installed-manifest.json" <<EOF
{
  "package": "$PACKAGE",
  "installedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "backup": "$BACKUP",
  "files": $(cd "$STAGING" && find . -type f ! -name manifest.json ! -name manifest.sig ! -name SHA256SUMS | sort | sed 's|^\./||' | python3 -c 'import json,sys; print(json.dumps([line.strip() for line in sys.stdin if line.strip()]))')
}
EOF

echo "the driver is installed in $TARGET"
echo "  backup: $BACKUP"
echo "run 'macxvm gpu NAME --install-driver $PACKAGE --accept-driver-changes' to finish the transaction from the host"
