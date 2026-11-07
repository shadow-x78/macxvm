#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Release Signing (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DRIVER="$ROOT/driver"
DIST="$DRIVER/dist"
STAGING="$DIST/mxgpu-staging"
KEY_FILE="${MXGPU_SIGNING_KEY:-$HOME/.macxvm/keys/mxgpu-release-2026}"

if [ ! -d "$STAGING" ]; then
    echo "the staging directory of the package is missing; run 'driver/installer/package.sh' first" >&2
    exit 1
fi

if [ ! -f "$KEY_FILE" ]; then
    echo "the signing key $KEY_FILE is missing; the private key never enters the repository" >&2
    exit 1
fi

if [ "$(uname -s)" != "Darwin" ]; then
    echo "the release is signed on a macOS builder only" >&2
    exit 1
fi

MANIFEST="$STAGING/manifest.json"
python3 - "$STAGING" "$MANIFEST" <<'PY'
import hashlib, json, os, sys, time

staging, manifest_path = sys.argv[1], sys.argv[2]
files = []
for root, dirs, names in os.walk(staging):
    dirs.sort()
    for name in sorted(names):
        path = os.path.join(root, name)
        rel = os.path.relpath(path, staging).replace(os.sep, "/")
        if rel in ("manifest.json", "manifest.sig", "SHA256SUMS"):
            continue
        with open(path, "rb") as handle:
            raw = handle.read()
        files.append({
            "path": rel,
            "size": len(raw),
            "sha256": hashlib.sha256(raw).hexdigest(),
            "mode": "%04o" % (os.stat(path).st_mode & 0o777),
        })
files.sort(key=lambda item: item["path"])

def digest(name):
    with open(os.path.join(staging, name), "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()

manifest = {
    "formatVersion": 1,
    "keyId": os.environ.get("MXGPU_KEY_ID", "mxgpu-release-2026"),
    "createdAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "target": {
        "os": os.environ.get("MXGPU_TARGET_OS", "macos"),
        "build": os.environ.get("MXGPU_TARGET_BUILD", ""),
        "arch": os.environ.get("MXGPU_TARGET_ARCH", "x86_64"),
        "sdk": os.environ.get("MXGPU_TARGET_SDK", ""),
        "driverAbi": os.environ.get("MXGPU_DRIVER_ABI", ""),
    },
    "files": files,
    "sbomSha256": digest("SBOM.json"),
    "noticeSha256": digest("NOTICE"),
}
with open(manifest_path, "w") as handle:
    json.dump(manifest, handle, indent=2, sort_keys=True)
    handle.write("\n")
print("the manifest holds %d files" % len(files))
PY

CANONICAL="$DIST/manifest.canonical.json"
python3 - "$MANIFEST" "$CANONICAL" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as handle:
    manifest = json.load(handle)
raw = json.dumps(manifest, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
with open(sys.argv[2], "wb") as handle:
    handle.write(raw)
PY

SEPARATED="$DIST/manifest.separated"
python3 - "$CANONICAL" "$SEPARATED" <<'PY'
import sys
with open(sys.argv[1], "rb") as handle:
    canonical = handle.read()
with open(sys.argv[2], "wb") as handle:
    handle.write(b"MXGPU-PACKAGE-v1\x00")
    handle.write(canonical)
PY

SIGNATURE="$STAGING/manifest.sig"
openssl pkeyutl -sign -inkey "$KEY_FILE" -rawin -in "$SEPARATED" -out "$SIGNATURE"
rm -f "$SEPARATED" "$CANONICAL"

(
    cd "$STAGING"
    find . -type f ! -name manifest.json ! -name manifest.sig ! -name SHA256SUMS -print0 |
        sort -z |
        xargs -0 shasum -a 256 >SHA256SUMS
)

echo "the package is signed"
echo "  manifest: $MANIFEST"
echo "  signature: $SIGNATURE"
