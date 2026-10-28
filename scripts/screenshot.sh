#!/bin/sh
# ─────────────────────────────────────────────
# MacXVM - Guest Screenshot (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

set -eu

NAME="${1:?usage: scripts/screenshot.sh NAME [OUT.png]}"
OUT="${2:-assets/screenshots/${NAME}.png}"

HOME_DIR="${MACXVM_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/macxvm}"
RUN_DIR="${MACXVM_RUN:-${XDG_RUNTIME_DIR:+$XDG_RUNTIME_DIR/macxvm}}"
RUN_DIR="${RUN_DIR:-/tmp/macxvm-$(id -u)}"

VM_DIR="$HOME_DIR/vms/$NAME"
[ -d "$VM_DIR" ] || { echo "macxvm: no machine '$NAME' ($VM_DIR)" >&2; exit 1; }
[ -f "$VM_DIR/qemu.pid" ] || { echo "macxvm: '$NAME' is not running" >&2; exit 1; }
PID=$(cat "$VM_DIR/qemu.pid")
kill -0 "$PID" 2>/dev/null || { echo "macxvm: '$NAME' is not running" >&2; exit 1; }

ID=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["id"])' "$VM_DIR/config.json")
SOCK="$RUN_DIR/$ID.sock"
[ -S "$SOCK" ] || { echo "macxvm: no QMP socket $SOCK" >&2; exit 1; }

RAW=$(mktemp /tmp/macxvm-shot-XXXXXX.ppm)
trap 'rm -f "$RAW"' EXIT

python3 - "$SOCK" "$RAW" << 'EOF'
import json, socket, sys

sock, out = sys.argv[1], sys.argv[2]
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect(sock)
f = s.makefile("rwb")
f.readline()  # the greeting of QMP
f.write(json.dumps({"execute": "qmp_capabilities"}).encode() + b"\n")
f.flush()
f.readline()
f.write(json.dumps({"execute": "screendump", "arguments": {"filename": out}}).encode() + b"\n")
f.flush()
while True:
    msg = json.loads(f.readline())
    if "return" in msg or "error" in msg:
        if "error" in msg:
            sys.exit("macxvm: the screen dump failed: %s" % msg["error"].get("desc", ""))
        break
s.close()
EOF

mkdir -p "$(dirname "$OUT")"
if command -v ffmpeg >/dev/null 2>&1; then
    ffmpeg -y -loglevel error -i "$RAW" "$OUT"
elif python3 -c "import PIL" 2>/dev/null; then
    python3 - "$RAW" "$OUT" << 'EOF'
import sys
from PIL import Image
Image.open(sys.argv[1]).convert("RGB").save(sys.argv[2])
EOF
else
    mv "$RAW" "${OUT%.png}.ppm"
    echo "macxvm: no ffmpeg and no PIL, wrote ${OUT%.png}.ppm" >&2
    exit 0
fi
echo "macxvm: wrote $OUT"
