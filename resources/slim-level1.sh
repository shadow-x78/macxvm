#!/bin/bash
# ─────────────────────────────────────────────
# MacXVM - Guest Slimming Level 1 (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────
sudo mdutil -a -i off || true
sudo tmutil disable || true
sudo pmset -a displaysleep 0 sleep 0 disksleep 0 || true
defaults write com.apple.universalaccess reduceMotion -bool true || true
defaults write com.apple.universalaccess reduceTransparency -bool true || true
defaults write NSGlobalDomain NSAutomaticWindowAnimationsEnabled -bool false || true
defaults write com.apple.dock launchanim -bool false || true
defaults write com.apple.dock autohide -bool true || true
defaults write com.apple.CrashReporter DialogType none || true
defaults -currentHost write com.apple.screensaver idleTime 0 || true
sudo softwareupdate --schedule off || true
sudo defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticCheckEnabled -bool false || true
sudo defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticDownload -bool false || true
killall Dock || true
echo "slim level 1 done"
