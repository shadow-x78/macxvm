<div align="center">

# Troubleshooting - MacXVM

[![Version](https://img.shields.io/badge/version-0.5.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="TROUBLESHOOTING.md">🇬🇧 English</a> · <a href="TROUBLESHOOTING_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [Start Here](#start)
- [The Host](#host)
- [The Machine](#machine)
- [The Guest](#guest)
- [The Build](#build)
- [Diagnostic Commands](#diagnostics)

---

<a id="start"></a>
## Start Here

```bash
macxvm doctor
```

The report checks the operating system, `/dev/kvm`, the virtualisation flags of the processor, QEMU, the KVM module, the host tools, the free disk space, the memory of the host and the registered machines. It ends with the legal notice.

<a id="host"></a>
## The Host

**`cannot use /dev/kvm (add your user to the 'kvm' group and log in again)`**

Your user may not use the KVM device. Add yourself to the group and log in again:

```bash
sudo usermod -aG kvm $USER
```

Log out and back in, or start a new session. A new shell is not enough, the group is read when the session starts.

**`MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)`**

Either the host is not Linux x86_64, or QEMU is not installed. On Fedora:

```bash
sudo dnf install qemu-system-x86_64 qemu-img
```

**The doctor warns about `ignore_msrs`.**

Several macOS on KVM guides need the KVM module to ignore the model specific registers. Check the README of the project your boot assets come from. MacXVM never changes this setting for you.

**The doctor warns that QEMU does not select KVM by default.**

Some builds of QEMU only use KVM when the accelerator is named, and a machine that asks for the host CPU needs it. Add `-accel kvm` to the args file of the machine:

```bash
qemu-system-x86_64 \
  -accel kvm \
  -cpu host \
  ...
```

**`run directory path too long; set MACXVM_RUN to a shorter path`**

The QMP socket path must fit into `sun_path`. Point `MACXVM_RUN` at a shorter directory:

```bash
export MACXVM_RUN=/tmp/macxvm
```

<a id="machine"></a>
## The Machine

**`VM 'X' is already running`**

The machine is running. `macxvm status X` shows the state, `macxvm stop X` stops it.

**`'X' is a sealed base image; clone it first (macxvm clone X NEW)`**

A sealed base is a backing file for overlays and is never booted. Clone it and boot the clone.

**`only sealed base images can be overlay-cloned; use --full or seal it first`**

Seal the base with `macxvm base seal X` first, or clone with `--full`, which copies the whole disk.

**`'X' is the base of other VMs (a, b); delete them first`**

The base is still in use. Delete the overlays first, or unseal the base with `macxvm base unseal X`.

**`another macxvm operation is running on 'X'`**

Another `macxvm` command holds the lock of the machine. Wait for it to finish.

**`VM 'X' is not running`**

The machine is stopped. `macxvm start X` starts it. If a QEMU is still alive but lost its pid file, the tool finds it through its command line and stops it anyway.

**The machine starts and stops immediately.**

Read the log of the machine:

```bash
tail -n 40 ~/.local/share/macxvm/vms/X/console.log
```

The most common cause is a wrong path in the args file, or a missing file of the assets directory.

**`QEMU exited before it was ready (see ...)`**

The last lines of the console log are printed above the message. They usually name the argument that QEMU did not accept.

<a id="guest"></a>
## The Guest

**`could not reach 'X' over SSH after 240 s (is Remote Login enabled? see README: First boot checklist)`**

In the guest, open System Settings, General, Sharing and switch on **Remote Login**. The user of the machine must have a password, and the passwordless sudo of the [first boot checklist](../README.md#first-boot-checklist) lets MacXVM run the commands that need root.

**`Command Line Tools are not installed in 'X' (run: macxvm tools install X clt)`**

Install them first:

```bash
macxvm tools install X clt
```

**`this profile has no Xcode; use profile 'xcode'`**

The machine of the profile `cli` has no Xcode. Create a machine of the profile `xcode` for a build of an Xcode project.

**The guest does not answer after the slimming.**

Restore it and reboot the guest:

```bash
macxvm slim X --restore
```

**The guest is slow after the installation of Xcode.**

The first launch of Xcode and the first build fill the caches. This takes a while once, on the disk of the guest.

<a id="build"></a>
## The Build

**`unsupported project`**

The project directory has no `.xcworkspace`, no `.xcodeproj` and no `Package.swift` at its top level. The detection only looks at the top level.

**`the project has several schemes (App, App-Staging), use --scheme`**

The guest was asked for its schemes and found more than one. Choose one with `--scheme`.

**The build does not find the project in the guest.**

The path of the project is part of the name of the work directory. Two different paths of the same project use two different directories, so the caches are not shared. Use the same path every time, or a symlink.

**`the local path '~/.bashrc' must be relative to the project`**

The paths of a fetch must stay inside the project. See [BUILDING.md](BUILDING.md#fetch).

**The build stops at the signing step.**

Code signing needs an unlocked login keychain. Use a simulator destination or `CODE_SIGNING_ALLOWED=NO`.

<a id="diagnostics"></a>
## Diagnostic Commands

```bash
macxvm doctor                 # the state of the host
macxvm list                    # every registered machine
macxvm status X                # the state of one machine
macxvm list --json             # the same, for a script
tail -n 40 ~/.local/share/macxvm/vms/X/console.log   # the output of QEMU
cat ~/.local/share/macxvm/vms/X/config.json           # the description of the machine
```

Inside the guest:

```bash
xcodebuild -version            # is Xcode there?
xcode-select -p                # which developer directory is active?
launchctl print system/com.apple.sshd   # is SSH alive?
df -h                          # how much space is left?
```
