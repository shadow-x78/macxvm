#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Package Verification (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE="${1:-}"
PUBLIC_KEY="$ROOT/resources/mxgpu-release-public-key.txt"

if [ -z "$SOURCE" ]; then
    echo "usage: verify.sh PACKAGE" >&2
    exit 2
fi

if [ ! -e "$SOURCE" ]; then
    echo "the package $SOURCE is missing" >&2
    exit 1
fi

if [ -x "$ROOT/bin/macxvm" ] && [ -f "$SOURCE" ]; then
    if [ ! -f "$PUBLIC_KEY" ]; then
        echo "the public key of the release is missing" >&2
        exit 1
    fi
    exec "$ROOT/bin/macxvm" driver verify "$SOURCE"
fi

STAGING=""
if [ -d "$SOURCE" ]; then
    STAGING="$SOURCE"
else
    STAGING="$(mktemp -d)"
    trap 'rm -rf "$STAGING"' EXIT
    tar -xzf "$SOURCE" -C "$STAGING"
fi

if [ ! -f "$STAGING/manifest.json" ]; then
    echo "the package holds no manifest" >&2
    exit 1
fi

python3 - "$STAGING" <<'PY'
import hashlib, json, os, sys

staging = sys.argv[1]
with open(os.path.join(staging, "manifest.json"), encoding="utf-8") as handle:
    manifest = json.load(handle)
problems = []
for item in manifest.get("files", []):
    path = os.path.join(staging, item["path"])
    if not os.path.isfile(path):
        problems.append("the file %s is missing" % item["path"])
        continue
    with open(path, "rb") as handle:
        raw = handle.read()
    if len(raw) != item.get("size", -1):
        problems.append("the file %s has the wrong size" % item["path"])
    if hashlib.sha256(raw).hexdigest() != item.get("sha256", ""):
        problems.append("the file %s does not match its hash" % item["path"])
for problem in problems:
    print("the package is not intact: %s" % problem, file=sys.stderr)
if problems:
    sys.exit(1)
print("the package holds the %d files of its manifest" % len(manifest.get("files", [])))
PY
