#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Installation (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

SOURCE="${1:-}"
TARGET="${2:-/Library/Extensions}"
BACKUP_ROOT="/var/root/macxvm-backups"
BACKUP="$BACKUP_ROOT/$(date -u +%Y%m%dT%H%M%SZ)"
INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the driver is installed in a macOS guest" >&2
    exit 1
fi

if [ -z "$SOURCE" ]; then
    echo "usage: install.sh PACKAGE [TARGET]" >&2
    exit 2
fi

STAGING=""
if [ -d "$SOURCE" ]; then
    STAGING="$SOURCE"
elif [ -f "$SOURCE" ]; then
    STAGING="$(mktemp -d)"
    trap 'rm -rf "$STAGING"' EXIT
    tar -xzf "$SOURCE" -C "$STAGING"
else
    echo "the package $SOURCE is missing" >&2
    exit 1
fi

if [ ! -f "$STAGING/manifest.json" ]; then
    echo "the package holds no manifest" >&2
    exit 1
fi

if [ ! -f "$INSTALLER_DIR/verify.sh" ]; then
    echo "the package holds no verify.sh; the installation refuses a package that it cannot check" >&2
    exit 1
fi

/bin/sh "$INSTALLER_DIR/verify.sh" "$STAGING"

mkdir -p "$BACKUP"
for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
    if [ -e "$TARGET/$item" ]; then
        cp -R "$TARGET/$item" "$BACKUP/"
    fi
done

for item in MXRM.kext MXFB.kext MXAccel.kext MXAGDC.kext MXMTLDriver.bundle; do
    rm -rf "$TARGET/$item"
    if [ -e "$STAGING/$item" ]; then
        cp -R "$STAGING/$item" "$TARGET/"
    fi
done

python3 - "$SOURCE" "$BACKUP" "$STAGING" "$TARGET" >"$TARGET/macxvm-installed-manifest.json" <<'PY'
import json, os, plistlib, sys, time

source, backup, staging, target = sys.argv[1:5]
files = []
for root, dirs, names in os.walk(staging):
    dirs.sort()
    for name in sorted(names):
        path = os.path.join(root, name)
        rel = os.path.relpath(path, staging).replace(os.sep, "/")
        if rel in ("manifest.json", "manifest.sig", "SHA256SUMS"):
            continue
        files.append(rel)
version = ""
plist = os.path.join(target, "MXRM.kext", "Contents", "Info.plist")
if os.path.isfile(plist):
    with open(plist, "rb") as handle:
        version = str(plistlib.load(handle).get("CFBundleVersion", ""))
print(json.dumps({
    "package": source,
    "installedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "backup": backup,
    "driverVersion": version,
    "files": files,
}, indent=2, sort_keys=True))
PY

echo "the driver is installed in $TARGET"
echo "  backup: $BACKUP"
echo "run 'macxvm gpu NAME --install-driver PACKAGE --accept-driver-changes' to finish the transaction from the host"
