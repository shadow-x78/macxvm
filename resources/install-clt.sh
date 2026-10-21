#!/bin/bash
# ─────────────────────────────────────────────
# MacXVM - Command Line Tools Install (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
set -e
PROG=/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress
touch "$PROG"
LABEL=$(softwareupdate -l 2>/dev/null \
  | grep -B1 -E 'Command Line Tools' \
  | grep -E '^\s*\*' \
  | sed -E 's/^\s*\* (Label: )?//' \
  | sort -V | tail -n1)
if [ -z "$LABEL" ]; then
  rm -f "$PROG"
  echo "Could not find a Command Line Tools update. In the guest GUI run: xcode-select --install" >&2
  exit 1
fi
sudo softwareupdate -i "$LABEL" --verbose
rm -f "$PROG"
sudo xcode-select --switch /Library/Developer/CommandLineTools
xcode-select -p
