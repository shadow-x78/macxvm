#!/bin/bash
# ─────────────────────────────────────────────
# MacXVM - Guest Slimming Level 3 (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
APPS="/Applications"
KEEP="Safari.app Xcode.app Utilities.app"
removed=0
for app in "$APPS"/*.app; do
  name=$(basename "$app")
  case " $KEEP " in
    *" $name "*) continue;;
  esac
  case "$name" in
    Safari.app|Xcode.app|Xcode-beta.app|Utilities.app) continue;;
    *) ;;
  esac
  sudo rm -rf "$app" && removed=$((removed + 1))
done
sudo rm -rf "$APPS/Xcode-beta.app"
sudo rm -rf /Library/Updates/* /System/Library/AssetsV2/com_apple_MobileAsset_SoftwareUpdate* 2>/dev/null || true
sudo rm -rf /Library/Developer/CoreSimulator/Profiles/Runtimes/* 2>/dev/null || true
sudo rm -rf /Library/Developer/CoreSimulator/Caches/* 2>/dev/null || true
rm -rf "$HOME/Library/Developer/Xcode/DerivedData" "$HOME/Library/Developer/Xcode/iOS DeviceSupport" 2>/dev/null || true
rm -rf "$HOME/Library/Developer/CoreSimulator/Devices" 2>/dev/null || true
rm -rf "$HOME/Library/Developer/Xcode/UserData/Previews" 2>/dev/null || true
rm -rf "$HOME/Library/Caches"/* "$HOME/Library/Logs"/* "$HOME/Library/Containers/com.apple.mail" 2>/dev/null || true
rm -rf "$HOME/Library/Mail"/* 2>/dev/null || true
sudo rm -rf /private/var/log/* /private/var/folders/* 2>/dev/null || true
sudo rm -rf /Library/Logs/* /Library/Caches/* 2>/dev/null || true
sudo find /Applications -name "*.app" -maxdepth 2 2>/dev/null | wc -l | xargs echo "the applications that are left:"
echo "slim level 3 done: $removed applications removed"
