<div align="center">

# Architecture Specification - MacXVM

[![Version](https://img.shields.io/badge/version-0.4.1-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="ARCHITECTURE.md">🇬🇧 English</a> · <a href="ARCHITECTURE_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [Overview](#overview)
- [Packages](#packages)
- [Lifecycle](#lifecycle)
- [State Layout](#state-layout)
- [Command Flow](#command-flow)
- [Process Supervision](#process-supervision)
- [Networking](#networking)
- [Project Layout](#project-layout)
- [Design Rules](#design-rules)

---

<a id="overview"></a>
## Overview

MacXVM is a single static Go binary with no daemon. The only long-lived process is QEMU itself; MacXVM starts it, manages it over the QMP socket and exits. An idle host therefore carries no MacXVM process at all.

The tool never describes a macOS machine. The machine definition lives in a user-supplied args file, MacXVM substitutes a handful of placeholders into it, appends its own management flags and starts QEMU with the result. This keeps every macOS-on-QEMU detail (CPU model, SMC keys, SMBIOS values, OpenCore settings, firmware paths) out of the code and out of the repository.

```mermaid
flowchart TD
    USER["macxvm CLI"]
    ARGS["args file (user supplied)"]
    CFG["config.json (per VM)"]
    QEMU["qemu-system-x86_64"]
    QMP["QMP socket (unix)"]
    GUEST["macOS guest"]
    TOOLS["ssh / scp / rsync"]

    USER -->|expand placeholders| ARGS
    ARGS -->|argv| QEMU
    CFG -->|identity, resources| USER
    USER -->|spawn, setsid| QEMU
    USER -->|qmp_capabilities, quit, system_powerdown| QMP
    QMP --> QEMU
    QEMU --> GUEST
    TOOLS -->|hostfwd 127.0.0.1:port| GUEST
    USER --> TOOLS
```

<a id="packages"></a>
## Packages

| Package | Responsibility |
|---------|----------------|
| `cmd/macxvm` | Entry point, subcommand dispatch, panic recovery, exit codes |
| `internal/exitcode` | The error type that carries a CLI exit code |
| `internal/paths` | The state directory and the run directory |
| `internal/names` | The VM name rule and its validation |
| `internal/sizeparse` | Parsing of `--ram` values such as `4096`, `4096M`, `8G` |
| `internal/config` | The persisted description of a VM, atomic writes, identity generation |
| `internal/store` | Registration, listing, cloning, sealing, deletion of VMs |
| `internal/argsfile` | Parsing, validation and expansion of the args file |
| `internal/runstate` | Process detection through the pid file and `/proc`, the operation lock |
| `internal/qmp` | The minimal QEMU Machine Protocol client |
| `internal/qemu` | Preflight, process spawn, QMP socket path, readiness wait, argv assembly |
| `internal/sshx` | The argument builders for SSH, SCP and rsync, `ShellQuote`, the SSH wait |
| `internal/slim` | The slimming policy: the deny list, the label filter, the restore and the measurement |
| `internal/tools` | Command Line Tools, Homebrew and the tooling report |
| `internal/xcode` | Expansion and installation of Xcode inside the guest |
| `internal/project` | Detection of the project kind and the `macxvm.json` file |
| `internal/doctor` | The host checks and the legal notice |
| `internal/hostcheck` | Host facts (RAM, cores, disk, KVM) and the `Runner` seam for subprocesses |
| `internal/cmds` | One file per command, the flag splitter, the usage text |
| `resources` | The guest scripts and the launchd label list, embedded into the binary |

The dependency graph is acyclic and shallow. `paths`, `names`, `sizeparse`, `exitcode`, `hostcheck` and `qmp` are leaves and only use the standard library. `config` builds on `exitcode` and `paths`; `store` on `config`, `paths` and `hostcheck`; `qemu` on `config`, `paths`, `runstate` and `qmp`; `cmds` on everything else.

<a id="lifecycle"></a>
## Lifecycle

```
create ──► start ──► (slim, tools, xcode) ──► stop ──► base seal
              │                                     │
              │                                     ▼
              └──────────────► clone ───────────► overlay VM
                                                    │
                                              build / run
```

- `create` registers the machine, creates a sparse qcow2 disk, validates the args file and writes `config.json`.
- `start` spawns QEMU, waits for the pid file and the QMP socket, and performs the QMP handshake.
- `slim`, `tools` and `xcode` run inside the guest over SSH. They start the machine when it is stopped and stop it again only when they started it.
- `base seal` marks a machine as a base image. A sealed base is never booted and never modified; it is only used as the backing file of overlays.
- `clone` creates an overlay of a sealed base. The overlay costs a few hundred kilobytes and boots in seconds.
- `clone --full` converts the whole disk instead of copying the file, so that the copy is a standalone disk and does not need its base any more.
- `run` creates a throwaway overlay, runs one command in it and removes it afterwards, including when the command is interrupted.

<a id="state-layout"></a>
## State Layout

```
$MACXVM_HOME  (default: $XDG_DATA_HOME/macxvm, else ~/.local/share/macxvm)
├── id_ed25519, id_ed25519.pub     # the SSH key shared by every machine
├── staging/                        # half-built machines, removed on failure
└── vms/<name>/
    ├── config.json                 # the description of the machine
    ├── args.txt                    # a copy of the args file of the user
    ├── disk.qcow2                  # the disk, or an overlay of a base
    ├── nvram.fd                    # only with --nvram-template
    ├── qemu.pid                    # written by QEMU itself
    ├── console.log                 # the output of QEMU
    ├── op.lock                     # serialises start, stop, clone, seal, delete
    └── slim-report.txt             # only after slim --measure

$MACXVM_RUN  (default: $XDG_RUNTIME_DIR/macxvm, else /tmp/macxvm-<uid>)
└── <id>.sock                       # the QMP socket of a machine
```

The run directory holds an unauthenticated QMP socket. MacXVM therefore refuses to use it unless it is a real directory of the current user that no other user may write to.

<a id="command-flow"></a>
## Command Flow

Every command follows the same three steps:

1. **Parse and validate.** The name of the machine is validated before it is ever used to build a path. A name that does not match `^[a-z0-9][a-z0-9._-]{0,62}$` is refused with exit code 2.
2. **Take the operation lock.** `op.lock` is an exclusive, non-blocking `flock`. `start`, `stop`, `clone`, `seal` and `delete` hold it for the whole operation, so two commands can never work on the same machine at the same time.
3. **Work and report.** Errors are returned as `*exitcode.Error` and printed as `macxvm: error: <message>` on stderr. Data goes to stdout, progress goes to stderr.

The flags may be written before or after the name. A helper splits the leading positional out of the argument list before handing the rest to the `flag` package, so both `macxvm create NAME --assets DIR` and `macxvm create --assets DIR NAME` work.

<a id="process-supervision"></a>
## Process Supervision

QEMU is started with `Setsid`, so it outlives the command that started it and never receives a hangup from the terminal. The parent closes the process handle without waiting for it.

The state of a machine is derived from the pid file that QEMU writes and from the command line of that process:

- the pid file is read and parsed;
- `/proc/<pid>/stat` decides whether the process is gone or a zombie;
- `/proc/<pid>/cmdline` must contain `macxvm-<id>`, which makes the check immune to a reused PID.

Because the command line of a process is briefly empty while it starts, the decision "did QEMU exit" is based on the state in `/proc/<pid>/stat` and never on the command line alone.

If a machine does not become ready within 20 seconds, the spawned QEMU is stopped, the pid file and the socket are removed, and the command reports exit code 7. A QEMU that is still alive but lost its pid file is recognised through its command line, so it is never started a second time: two QEMU processes writing one qcow2 image would corrupt it.

<a id="networking"></a>
## Networking

The guest is reached through the user-mode network stack of QEMU (SLIRP). MacXVM substitutes the placeholder `{{NETDEV}}` with:

```
user,id=net0,hostfwd=tcp:127.0.0.1:<port>-:22
```

The port is chosen when the machine is registered, stored in `config.json` and re-checked when the machine starts. If the port is taken in the meantime, a new free port is chosen and written back. This needs no root, no bridge and no IP discovery.

The port of a stopped machine is not reserved, so another local program may take it. This is why the SSH connections use a short `ConnectTimeout` and keepalive options: a program that answers on the port must not be able to hang MacXVM.

<a id="project-layout"></a>
## Project Layout

```
macxvm/
├── cmd/macxvm/                    # the entry point of the binary
├── internal/                      # the packages of the tool
├── resources/                     # the guest scripts and the label list
├── scripts/                       # installation, benchmark, style checks
├── docs/                          # bilingual guides (EN + AR)
├── assets/logo/                   # the logo, the banner and the social preview
├── .github/                       # workflows, issue and pull request templates
├── Makefile                       # build, test, install, clean
├── CHANGELOG.md                   # the release notes
├── CONTRIBUTING.md                # how to contribute
├── CODE_OF_CONDUCT.md             # the rules of the community
├── SECURITY.md                    # how to report a vulnerability
└── README.md, README_AR.md        # the entry points
```

<a id="design-rules"></a>
## Design Rules

1. **The standard library only.** `go.mod` never gets a `require` line, `CGO_ENABLED=0` is always used and the binary stays static.
2. **No macOS on QEMU details in the code.** No CPU model, no SMC or OSK values, no SMBIOS values, no OpenCore settings, no firmware paths.
3. **No Apple software in the repository.** No installer images, no boot loaders, no firmware, no keys. MacXVM launches what the user provides.
4. **No placeholders.** No `TODO`, no `panic("not implemented")`, no commented out code, no fake output.
5. **Reversible slimming.** Only the launchd labels of the label file are touched, a deny list protects SSH and the network, and every change can be undone.
6. **Every subprocess is an argument vector.** `os/exec` never receives a shell command, and every value that comes from the user is quoted for the remote shell.
