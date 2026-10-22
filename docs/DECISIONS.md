<div align="center">

# Decisions Log - MacXVM

[![Version](https://img.shields.io/badge/version-0.1.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Table of Contents

- [Confirmed with the project owner](#owner)
- [Spec ambiguities and deviations](#deviations)
- [Repository and documentation](#repository)

---

<a id="owner"></a>
## Confirmed with the project owner

Every deviation from the specification and every ambiguity that had to be resolved while implementing the MacXVM implementation specification v4. New entries are appended as the work proceeds.

| # | Decision | Reason |
|---|---|---|
| U1 | Module path `github.com/shadow-x78/macxvm`; LICENSE holder `shadow-x78` | Spec section 7 says `github.com/OWNER/macxvm` |
| U2 | Go toolchain installed by the owner (`>= 1.22`); `go *` allowed in the shell | Build, vet and test gates must be real, not asserted |
| U3 | The repository ships automated tests only; macOS-guest acceptance is a manual checklist in the README | The build host has no macOS KVM assets (no OpenCore/EFI, no installer image, no args file) |

<a id="deviations"></a>
## Spec ambiguities and deviations

| # | Ambiguity | Resolution |
|---|---|---|
| A1 | Spec section 6 places the four guest files in root `resources/`, but spec section 7 embeds them from `internal/resources`. `//go:embed` cannot reach a parent directory | Keep the spec section 6 tree exactly: `resources/` holds the four files, and `resources/embed.go` (`package resources`) embeds them. Import path is `github.com/shadow-x78/macxvm/resources` |
| A2 | Spec section 17 wants a `Runner` interface for every subprocess, but spec section 6 lists no exec package | One canonical interface `hostcheck.Runner` in `internal/hostcheck/runner.go`; every package that shells out takes it as a parameter, so tests assert argv without executing |
| A3 | `--ram` suffix grammar | Accept exactly `\d+` (MiB) or `\d+[GM]` (case-insensitive). `8GB`, `8GiB`, `1.5G`, `8K` are rejected with exit 2 |
| A4 | `config.json` without a `schemaVersion` key | Decodes as version 1 (tolerant). Only `schemaVersion > 1` is rejected with `config was created by a newer macxvm` |
| A5 | VM lifecycle for `slim` and `xcode install` | Same rule as spec sections 11.10 and 11.12: start the VM if stopped, and stop it afterwards only if the command started it, unless `--keep` |
| A6 | `macxvm.json` `sync.exclude` and `fetch` | Honoured by `build` in addition to the built-in excludes (`.git`, `DerivedData`, `build`, `.build`) |
| A7 | `xcode install --platform` | Accepts `iOS` only (default `iOS`); any other value is rejected with exit 2 |
| A8 | Project detection | Top level only. A `.xcodeproj` nested inside an `.xcworkspace` is ignored; `.xcworkspace` wins over `.xcodeproj` |
| A9 | `doctor` exit code | 1 when any check fails; warnings do not change the exit code |
| A10 | `#install` prefix | Exactly one space: `#install ` |
| A11 | Host protection | Checked at `create`, `start` and `run`: `--ram` above 75 % of host RAM, or `--cpus` above host cores minus one, is refused unless `--force`; RAM above 50 % warns |
| A12 | `delete` of a running VM | Exit 1, `VM 'X' is running (stop it first)` |
| A13 | `base seal` of a running VM | Exit 4, `VM 'X' is already running` |
| A14 | `unseal` or `delete` while overlays exist | Exit 1, `'X' is the base of other VMs (NAMES); delete them first` |
| A15 | `list` and `status` table columns | `RAM` is MiB, `DISK` is GiB (raw integers) |
| A16 | `--` handling | Go's `flag` package stops at the first positional, so `NAME -- cmd` leaves `--` in the remainder; every command strips exactly one leading `--` after the positional |
| A17 | Flag ordering | The spec's usage puts `NAME` before the flags, so `cmds.SplitArgs` splits the leading positional out of the argument list before handing the rest to `flag`. Bool flags are detected through the `FlagSet`, so `--flag value NAME` works too |
| A18 | `Makefile` `install` target | Adds `mkdir -p $(HOME)/.local/bin` before the install step; the spec's Makefile would fail when the directory does not exist |
| A19 | Version | `var Version = "0.1.0"` in `cmd/macxvm/main.go`; `macxvm version` prints `macxvm 0.1.0` |
| A20 | `go.mod` | `go 1.22` language level; any toolchain from 1.22 upwards works. The build host uses Go 1.27.3 |
| A21 | Release artefacts | `linux/amd64` only, built with `CGO_ENABLED=0`. No extra release workflow: spec section 17 allows a single CI job only |
| A22 | Guest assets | The repository never ships, downloads or embeds Apple software, firmware, keys or boot loaders. The machine definition, bootloader, firmware and installer image come from the user |
| A23 | Order of the lines of the args file | The `#install` lines are appended after the normal lines, so that the normal machine definition is not overridden by the installation steps |
| A24 | A missing assets directory | Exit 2 in `create` (the argument is wrong) and exit 1 in `start` (the stored directory is gone) |
| A25 | Detecting that QEMU exited early | The command line of a process is briefly empty while it starts, so the decision is based on `/proc/<pid>/stat` (`runstate.ProcessGone`): a missing or zombie process means the machine is gone. The command line is only used to recognise the process of the machine, which makes the check immune to a reused PID |
| A26 | `store.SelectPrunable` | It takes the running check as an argument, so the selection of the temporary machines can be tested without a running QEMU |
| A27 | The exit code of a remote command | `macxvm exec` and `macxvm run` pass the exit code of the remote command through. The error carries no message, so nothing is printed in that case |
| A28 | Waiting for the guest | `sshx.WaitForSSH` waits 240 seconds by default, as the specification says, and every command that needs the guest uses it |
| A29 | Size of the code | The implementation has about 5,200 lines of Go without the tests, above the 4,500 lines of the target. The 25 commands of the specification each need their own file, and the legal notice, the doctor and the argument helpers take their share. The code stays small in behaviour: one binary, no daemon, no background process |
| A30 | The `run` command | A temporary machine is removed when the command ends, when it fails and when the command is interrupted with SIGINT or SIGTERM |
| A31 | The SSH options | The options of the specification are used, plus `ServerAliveInterval=5` and `ServerAliveCountMax=2`. Without them a guest that stops answering keeps the SSH client waiting forever, and the graceful step of `stop` would never end |
| A32 | The paths of a fetch | The remote and the local path of a fetch must be relative and must stay inside the project. The `fetch` list of `macxvm.json` belongs to the repository, so it may not write anywhere on the host |
| A33 | The creation of a virtual machine | The new machine is built in a staging directory and replaces the old one only when it is complete. A failure never destroys what is already there |
| A34 | The run directory | The directory that holds the QMP socket must be a real directory of the current user that no other user may write to. The QMP socket is not authenticated |
| A35 | The release platform | The `build` target of the Makefile pins `GOOS=linux GOARCH=amd64`, because the tool only runs on Linux x86_64 |
| A36 | A machine that did not start | A QEMU process whose command line contains the marker of the machine is recognised even without a pid file, so that it is not started a second time and can still be stopped |
| A37 | The argument builders of the SSH commands | `sshx.SSHArgs`, `sshx.SCPArgs` and `sshx.RSyncArgs` return the arguments of the program, not the name of the program. The name is added by the caller, so that the program is not passed twice |

<a id="repository"></a>
## Repository and documentation

| # | Decision | Reason |
|---|---|---|
| A38 | The name of the project | One spelling everywhere: the display name is `MacXVM`, the folder is `MacXVM`, the binary, the state directory and the environment variables are `macxvm`, and the module path is `github.com/shadow-x78/macxvm` |
| A39 | The decisions log | It moved from the root of the repository to `docs/DECISIONS.md`, so that every document of the project lives in the same place and carries the same header. The content is unchanged |
| A40 | The colours of the project | The macOS palette: system blue `#007AFF`, indigo `#5E5CE6`, graphite `#1D1D1F`, and the text colours `#F5F5F7` and `#A1A1A6`. The logo, the banner, the social preview and the badges use them |
| A41 | The documentation is bilingual | Every guide exists in English and in Arabic, and the README has a language switcher, like the other projects of the owner |
| A42 | The file headers | Every source file carries the header of the project, and `scripts/check-headers.sh` fails when one misses it. The CI runs it |
| A43 | The license | MIT, as the specification states. MacXVM ships no Apple software and links no copyleft library, so a permissive license is the right one. The license tab of the repository stays empty until the first push, because GitHub reads the license from the committed file |
