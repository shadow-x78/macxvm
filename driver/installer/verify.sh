#!/usr/bin/env bash
# ─────────────────────────────────────────────
# MacXVM - MXGPU Package Verification (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -euo pipefail

SOURCE="${1:-}"
TRUST_ROOT="${MACXVM_TRUST_ROOT:-}"

if [ -z "$SOURCE" ]; then
    echo "usage: verify.sh PACKAGE" >&2
    exit 2
fi

if [ ! -e "$SOURCE" ]; then
    echo "the package $SOURCE is missing" >&2
    exit 1
fi

if [ -z "$TRUST_ROOT" ] || [ ! -f "$TRUST_ROOT" ]; then
    echo "the trust root of the release is missing; the host writes it before it installs a package" >&2
    exit 1
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

if [ ! -f "$STAGING/manifest.sig" ]; then
    echo "the package holds no signature" >&2
    exit 1
fi

python3 - "$STAGING" "$TRUST_ROOT" <<'PY'
import hashlib, json, os, sys

P = 2 ** 255 - 19
L = 2 ** 252 + 27742317777372353535851937790883648493
D = -121665 * pow(121666, P - 2, P) % P
SQRT_M1 = pow(2, (P - 1) // 4, P)


def inv(value):
    return pow(value, P - 2, P)


def decode_point(raw):
    if len(raw) != 32:
        raise ValueError("a point is 32 bytes")
    value = int.from_bytes(raw, "little")
    y = value & ((1 << 255) - 1)
    sign = value >> 255
    if y >= P:
        raise ValueError("the point is out of range")
    x2 = (y * y - 1) * inv(D * y * y + 1) % P
    x = pow(x2, (P + 3) // 8, P)
    if (x * x - x2) % P != 0:
        x = x * SQRT_M1 % P
    if (x * x - x2) % P != 0:
        raise ValueError("the point is not on the curve")
    if x == 0 and sign == 1:
        raise ValueError("the point has an invalid sign")
    if x & 1 != sign:
        x = P - x
    return (x, y)


def add(first, second):
    x1, y1 = first
    x2, y2 = second
    factor = D * x1 * x2 * y1 * y2 % P
    x3 = (x1 * y2 + x2 * y1) * inv(1 + factor) % P
    y3 = (y1 * y2 + x1 * x2) * inv(1 - factor) % P
    return (x3, y3)


def mul(scalar, point):
    result = (0, 1)
    while scalar:
        if scalar & 1:
            result = add(result, point)
        point = add(point, point)
        scalar >>= 1
    return result


def base_point():
    return decode_point((4 * inv(5) % P).to_bytes(32, "little"))


def verify(public, signature, message):
    if len(public) != 32 or len(signature) != 64:
        return False
    try:
        a = decode_point(public)
        r = decode_point(signature[:32])
    except ValueError:
        return False
    s = int.from_bytes(signature[32:], "little")
    if s >= L:
        return False
    k = int.from_bytes(hashlib.sha512(signature[:32] + public + message).digest(), "little") % L
    return mul(s, base_point()) == add(r, mul(k, a))


def canonical(value):
    if isinstance(value, dict):
        parts = []
        for key in sorted(value):
            parts.append(json.dumps(key, ensure_ascii=False).encode("utf-8") + b":" + canonical(value[key]))
        return b"{" + b",".join(parts) + b"}"
    if isinstance(value, list):
        return b"[" + b",".join(canonical(item) for item in value) + b"]"
    if isinstance(value, bool):
        return b"true" if value else b"false"
    if value is None:
        return b"null"
    if isinstance(value, int):
        return str(value).encode("ascii")
    if isinstance(value, float):
        raise ValueError("the manifest holds a number that is not an integer")
    return json.dumps(value, ensure_ascii=False).encode("utf-8")


staging, trust_root = sys.argv[1], sys.argv[2]
with open(trust_root, encoding="utf-8") as handle:
    key_text = "".join(line.strip() for line in handle if line.strip() and not line.lstrip().startswith("#"))
try:
    public = bytes.fromhex(key_text)
except ValueError:
    print("the trust root of the release is not hexadecimal", file=sys.stderr)
    sys.exit(1)

with open(os.path.join(staging, "manifest.json"), "rb") as handle:
    raw = handle.read()
with open(os.path.join(staging, "manifest.sig"), "rb") as handle:
    signature = handle.read()

try:
    manifest = json.loads(raw)
    message = b"MXGPU-PACKAGE-v1\x00" + canonical(manifest)
except (ValueError, TypeError) as problem:
    print("the manifest of the package is not valid JSON: %s" % problem, file=sys.stderr)
    sys.exit(1)

if not verify(public, signature, message):
    print("the signature of the package does not match the release key", file=sys.stderr)
    sys.exit(1)

problems = []
for item in manifest.get("files", []):
    path = os.path.join(staging, item["path"])
    if not os.path.isfile(path):
        problems.append("the file %s is missing" % item["path"])
        continue
    with open(path, "rb") as handle:
        content = handle.read()
    if len(content) != item.get("size", -1):
        problems.append("the file %s has the wrong size" % item["path"])
    if hashlib.sha256(content).hexdigest() != item.get("sha256", ""):
        problems.append("the file %s does not match its hash" % item["path"])
for problem in problems:
    print("the package is not intact: %s" % problem, file=sys.stderr)
if problems:
    sys.exit(1)
print("the package holds the %d files of its manifest and its signature matches the release key" % len(manifest.get("files", [])))
PY
