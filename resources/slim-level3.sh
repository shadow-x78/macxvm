#!/bin/bash
# MacXVM - Level 3 Compatibility Guard (MIT)
# https://github.com/shadow-x78/macxvm
# Destructive bulk deletion was removed. Level 3 is implemented in Go by the
# allowlisted guestapps engine with dry-run, --yes, --keep and SIP checks.
echo "Use: macxvm apps remove NAME --set default --dry-run"
echo "Then: macxvm apps remove NAME --set default --yes"
exit 2
