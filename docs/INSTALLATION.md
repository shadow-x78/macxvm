<div align="center">

# Installation Guide - MacXVM

[![Version](https://img.shields.io/badge/version-0.3.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="INSTALLATION.md">🇬🇧 English</a> · <a href="INSTALLATION_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [Requirements](#requirements)
- [Build and Install](#build)
- [Manual Installation](#manual)
- [Verifying](#verify)
- [The State Directory](#state)
- [Updating](#updating)
- [Uninstalling](#uninstall)
- [Distribution](#distribution)

---

<a id="requirements"></a>
## Requirements

| Requirement | Detail |
|-------------|--------|
| Host | Linux x86_64 with hardware virtualisation (Intel VT-x or AMD-V) |
| KVM | `/dev/kvm` usable by your user, `qemu-system-x86_64` 6.0 or newer, `qemu-img` |
| Host tools | `ssh`, `scp`, `rsync`, `ssh-keygen` |
| Go | 1.22 or newer, only to build MacXVM |
| Guest | x86_64 macOS ≤ 26, installed by you |

MacXVM is one static binary without a daemon. It needs nothing on the host beyond the tools above, and it never runs as root.

```bash
macxvm doctor
```

<a id="build"></a>
## Build and Install

```bash
git clone https://github.com/shadow-x78/macxvm.git ~/MacXVM
cd ~/MacXVM && ./scripts/install.sh
```

The script builds the binary and installs it into `~/.local/bin`. Set `PREFIX` to install it elsewhere:

```bash
PREFIX=/usr/local/bin ./scripts/install.sh
```

<a id="manual"></a>
## Manual Installation

```bash
git clone https://github.com/shadow-x78/macxvm.git && cd macxvm
make build
make install
```

| Target | What it does |
|--------|--------------|
| `make build` | builds a static binary for `linux/amd64` into `bin/macxvm` |
| `make test` | runs `go vet ./...` and `go test ./...` |
| `make install` | builds and installs the binary into `~/.local/bin` |
| `make clean` | removes `bin/` |

The binary is about 4 MB and is built with `CGO_ENABLED=0`, so it needs no C library on the target host.

<a id="verify"></a>
## Verifying

```bash
macxvm version
macxvm doctor
```

`macxvm doctor` checks the operating system, `/dev/kvm`, the virtualisation flags of the processor, QEMU, the KVM module, the host tools, the free disk space, the memory of the host and the registered machines. It ends with the legal notice, because running macOS under QEMU/KVM on a PC is a matter for you and not for the tool.

<a id="state"></a>
## The State Directory

Everything MacXVM knows lives in two directories:

| Directory | Default | Content |
|-----------|---------|---------|
| `$MACXVM_HOME` | `$XDG_DATA_HOME/macxvm`, else `~/.local/share/macxvm` | the machines, their disks, the configuration, the SSH key |
| `$MACXVM_RUN` | `$XDG_RUNTIME_DIR/macxvm`, else `/tmp/macxvm-<uid>` | the QMP sockets of the running machines |

Both can be changed with the environment variables of the same name. The run directory must be a real directory of your user that no other user may write to, because the QMP socket inside it is not authenticated.

```bash
export MACXVM_HOME=/srv/macxvm
export MACXVM_RUN=/run/user/$(id -u)/macxvm
```

<a id="updating"></a>
## Updating

```bash
cd ~/MacXVM
git pull
./scripts/install.sh
```

The machines themselves live in the state directory and are not touched by an update.

<a id="uninstall"></a>
## Uninstalling

```bash
rm -f ~/.local/bin/macxvm
rm -rf ~/.local/share/macxvm
```

The second command removes every registered machine, every disk and the SSH key. Copy the state directory somewhere else first if you want to keep the machines.

<a id="distribution"></a>
## Distribution

MacXVM has no package repository yet. The release page carries the static binary for `linux/amd64`, and a distribution package would be built from the same `make build` target.

A distribution must keep these properties:

- the binary is static and built with `CGO_ENABLED=0`,
- the release is built for `linux/amd64`,
- no Apple software, no firmware, no boot loader and no keys are ever included,
- the legal notice of the README and of `macxvm doctor` stays untouched.
