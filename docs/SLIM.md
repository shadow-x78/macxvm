<div align="center">

# Slimming a macOS Guest - MacXVM

[![Version](https://img.shields.io/badge/version-0.2.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="SLIM.md">🇬🇧 English</a> · <a href="SLIM_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [Goal](#goal)
- [Level 1](#level-1)
- [Level 2](#level-2)
- [The Deny List](#deny-list)
- [What Is Never Touched](#never)
- [Restoring](#restore)
- [Measuring](#measure)
- [Acceptance](#acceptance)
- [Troubleshooting](#troubleshooting)

---

<a id="goal"></a>
## Goal

A macOS guest that idles at 30 % of the processor is useless for building. The slimming levels of MacXVM cut the idle cost of the guest down: fewer background services, no indexing, no animations, no automatic updates.

Both levels run **inside the guest** over SSH, need no root on the host and are reversible. The guest keeps working: SSH, the network and `xcodebuild` must survive.

```bash
macxvm slim demo --level 1
macxvm slim demo --level 2 --measure
macxvm slim demo --restore
```

<a id="level-1"></a>
## Level 1 (safe, the default)

Level 1 only changes settings. It never stops a service and never removes a file:

| Change | Effect |
|--------|--------|
| `mdutil -a -i off` | no Spotlight indexing, no background indexing of the disk |
| `tmutil disable` | no Time Machine, no snapshot churn |
| `pmset` sleep timers to 0 | the guest never puts the display or the disks to sleep |
| `reduceMotion`, `reduceTransparency` | fewer animations, less work for the compositor |
| `NSAutomaticWindowAnimationsEnabled` | no window animations |
| `dock` without launch animation | the Dock starts instantly |
| `CrashReporter DialogType none` | no crash dialogs that block a session |
| `softwareupdate --schedule off` | no automatic system updates |
| `AutomaticCheckEnabled`, `AutomaticDownload` | macOS never checks for updates on its own |

The automatic updates stay off on purpose: a macOS system update inside such a guest can break booting. Do not run system updates in the guest.

The script lives in `resources/slim-level1.sh` and is embedded into the binary.

<a id="level-2"></a>
## Level 2 (aggressive, optional)

Level 2 does everything of level 1 and disables the launchd jobs of `resources/slim-labels.txt`:

| Domain | Label | What it does |
|--------|-------|--------------|
| `gui` | `com.apple.photoanalysisd` | analysis of the photo library |
| `gui` | `com.apple.mediaanalysisd` | analysis of the media library |
| `gui` | `com.apple.suggestd` | suggestions of the system |
| `gui` | `com.apple.knowledge-agent` | the knowledge assistant |
| `gui` | `com.apple.parsecd` | parsing of CDs and DVDs |
| `gui` | `com.apple.assistantd` | the assistant |
| `gui` | `com.apple.Siri.agent` | Siri |
| `gui` | `com.apple.siriknowledged` | the knowledge of Siri |
| `gui` | `com.apple.helpd` | the help viewer |
| `gui` | `com.apple.tipsd` | the tips of macOS |
| `system` | `com.apple.analyticsd` | the analytics of the system |

A job that does not exist on your version of macOS is skipped, so the list stays valid across releases. The list is embedded into the binary and filtered by the policy of the tool before it is written to the guest.

You may extend the file, but the deny list still applies to what you add.

<a id="deny-list"></a>
## The Deny List

The list of the protected labels is enforced in the code, not in the file. A label that starts with one of these prefixes is never disabled:

```
com.apple.dt.                  com.apple.opendirectoryd
com.apple.CoreSimulator        com.apple.syslogd
com.openssh                     com.apple.diskarbitrationd
com.apple.launchd              com.apple.coreservices
com.apple.WindowServer         com.apple.xpc
com.apple.loginwindow           com.apple.sshd
com.apple.mDNSResponder
com.apple.networkd
com.apple.configd
com.apple.securityd
com.apple.trustd
```

These are the services that keep SSH, the network, the window server, the keychain and the system itself alive. The comparison ignores the case, so `com.apple.sshd` and `com.apple.SSHD` are both protected.

When a label is dropped, the tool says so:

```
macxvm: warning: skipping the protected label gui com.apple.sshd
```

<a id="never"></a>
## What Is Never Touched

- The system volume is never modified.
- SIP is never changed, `csrutil` is never used.
- No system application is removed.
- No file of the guest is deleted by the slimming.
- Only the labels of the label file are ever disabled.

The level 1 settings are not undone by `--restore`, because keeping the automatic updates off is what protects the boot of the guest. The restore undoes the disabled jobs and turns Spotlight back on.

<a id="restore"></a>
## Restoring

```bash
macxvm slim demo --restore
```

The restore reads `~/.macxvm-slim-disabled` in the guest, enables every job that is listed there, deletes the file and turns Spotlight on again with `sudo mdutil -a -i on`. The guest has to be rebooted afterwards, so that the jobs start again.

<a id="measure"></a>
## Measuring

```bash
macxvm slim demo --level 2 --measure
```

The measurement runs before and after the slimming, with 30 seconds of idle time in between, and stores both results in `slim-report.txt` of the machine:

```
=== before ===
processes: 412
top -l 2 -n 8 -o cpu -stats pid,command,cpu,mem | tail -n 12
...
vm_stat | head -6
...
memory_pressure | tail -4
...

=== after ===
processes: 380
...
```

The number of processes in the first line is the quickest way to see the effect.

<a id="acceptance"></a>
## Acceptance

A slimming is accepted when all of this holds:

- the number of processes is lower than before,
- SSH still works,
- `xcodebuild -version` still works,
- a small project still builds.

If a label breaks something, remove it from `resources/slim-labels.txt` and note the change in [DECISIONS.md](DECISIONS.md).
<a id="troubleshooting"></a>
## Troubleshooting

**The guest does not answer after the slimming.**
Restore it with `macxvm slim demo --restore`, reboot the guest and try again. If the SSH connection is broken, the job that broke it can be enabled from a terminal of the guest:

```bash
sudo launchctl enable system/com.apple.sshd
```

**A label is missing in the list of the guest.**
A label that does not exist on your version of macOS is skipped silently. Check the list in the guest with `launchctl print system/<label>`.

**The measurement shows no difference.**
The measurements need an idle guest. Wait until the guest is settled, then measure again. The `--measure` flag already waits 30 seconds between the two measurements.
