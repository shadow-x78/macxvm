#!/bin/bash
# ─────────────────────────────────────────────
# MacXVM - Guest Slimming Level 2 (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
UIDN=$(id -u)
LOG="$HOME/.macxvm-slim-disabled"; : > "$LOG"
while read -r dom label; do
  [ -z "$dom" ] && continue
  case "$dom" in \#*) continue;; esac
  if [ "$dom" = "gui" ]; then target="gui/$UIDN/$label"; S=""; else target="system/$label"; S="sudo"; fi
  if launchctl print "$target" >/dev/null 2>&1; then
    $S launchctl bootout "$target" 2>/dev/null || true
    $S launchctl disable "$target" || true
    echo "$dom $label" >> "$LOG"
  fi
done < /tmp/slim-labels.txt
echo "slim level 2 done: $(wc -l < "$LOG") jobs disabled"
