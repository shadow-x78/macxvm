<div align="center">

# Decisions Log - MacXVM

[![Version](https://img.shields.io/badge/version-0.9.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Table of Contents

- [Confirmed with the project owner](#owner)
- [Spec ambiguities and deviations](#deviations)
- [Repository and documentation](#repository)
- [The audit of the project](#audit)

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
| A19 | Version | `Version = "0.9.0"` in `internal/cmds/cmds.go`; release builds pin the same value with ldflags |
| A20 | `go.mod` | `go 1.22` language level; any toolchain from 1.22 upwards works. The build host uses Go 1.27.3 |
| A21 | Release artefacts | `linux/amd64` only, built with `CGO_ENABLED=0`. No extra release workflow: spec section 17 allows a single CI job only |
| A22 | Guest assets | The repository never ships, downloads or embeds Apple software, firmware, keys or boot loaders. The machine definition, bootloader, firmware and installer image come from the user |
| A23 | Order of the lines of the args file | The `#install` lines are appended after the normal lines, so that the normal machine definition is not overridden by the installation steps |
| A24 | A missing assets directory | Exit 2 in `create` (the argument is wrong) and exit 1 in `start` (the stored directory is gone) |
| A25 | Detecting that QEMU exited early | The command line of a process is briefly empty while it starts, so the decision is based on `/proc/<pid>/stat` (`runstate.ProcessGone`): a missing or zombie process means the machine is gone. The command line is only used to recognise the process of the machine, which makes the check immune to a reused PID |
| A26 | `store.SelectPrunable` | It takes the running check as an argument, so the selection of the temporary machines can be tested without a running QEMU |
| A27 | The exit code of a remote command | `macxvm exec` and `macxvm run` pass the exit code of the remote command through. The error carries no message, so nothing is printed in that case |
| A28 | Waiting for the guest | `sshx.WaitForSSH` waits up to 240 seconds for key-only SSH; `open` intentionally never waits, and bootstrap uses a longer configurable manual-install wait |
| A29 | Size of the code | The implementation has about 5,200 lines of Go without the tests, above the 4,500 lines of the target. The 25 commands of the specification each need their own file, and the legal notice, the doctor and the argument helpers take their share. The code stays small in behaviour: one binary, no daemon, no background process |
| A30 | The `run` command | A temporary machine is removed when the command ends, when it fails and when the command is interrupted with SIGINT or SIGTERM |
| A31 | The SSH options | Each VM has a `known_hosts` with `HostKeyAlias=macxvm-<id>`, local TOFU while its marked QEMU is running, then strict verification and `BatchMode=yes`; `GlobalKnownHostsFile=/dev/null` only isolates the host's global file, not the per-VM trust |
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

<a id="audit"></a>
## The audit of the project

The whole project was audited in October 2026. The findings, the evidence and the verification of every change are in the report of the audit. The decisions below are the ones that the audit produced and that a reader of the code cannot guess.

| # | Decision | Reason |
|---|---|---|
| A44 | The full copy of a machine | `store.Clone` converts the disk with `qemu-img convert -O qcow2` instead of copying the file. A copy of a qcow2 overlay keeps the pointer to its base in the header, so the copy would break as soon as the base was deleted |
| A45 | The name of a stored configuration | `config.Load` refuses a name that is not a valid VM name. The name that the user types was always validated, but the name that is stored in `config.json` was not, and every path of the state directory is built from it |
| A46 | The order of the checks of a temporary machine | The memory size and the host protection are checked before the clone, so that a rejected value leaves no machine behind |
| A47 | The exit code of the host protection | The refusal of the host protection carries the usage code, like every other wrong argument |
| A48 | The console log of a machine | It is written with the mode 0600 and it is truncated on every start, so that it holds the messages of the last start only and does not grow without a bound |
| A49 | The public key in the guest | It is appended only when it is not in `~/.ssh/authorized_keys` yet, so that a second `ssh-key --install` does not leave a duplicate |
| A50 | The comments of the source files | The comments of the Go files and of the shell scripts were removed during the audit. The credit header at the top of a file and the tool directives such as `//go:embed` stay, because the first one is the credit of the project and the second one is read by the compiler |
| A51 | The comments of the configuration files | The `Makefile`, `.gitignore`, `.gitattributes` and the files of `.github` carry the header block of the project and the section titles. `scripts/check-headers.sh` checks `.gitignore` as well, because the file carries the header from now on |
| A52 | The version 0.2.0 | The audit added a check to `macxvm doctor`, so the version moves to 0.2.0. A new capability of the command line is a minor release, and the fixes that came with it are a patch release |
| A53 | The tags and the releases | Every version of the changelog gets a tag and a release on GitHub, with the binary `macxvm-v<version>-linux-amd64` of the tag. The release notes are the section of the changelog |
| A54 | The network of the guest | The `-device` line of the args file points at the network of QEMU with `netdev=net0`, and the example names the accelerator with `-accel kvm`. The card of the example stayed outside the network, so the port of SSH was never forwarded, and a QEMU that does not choose KVM on its own fell back to the software emulation of the processor. Both were found by running the tool on a host |
| A55 | The boot assets of a guest | `macxvm image prepare` fetches the installer of Apple and the OpenCore boot loader and writes an args file for macOS. The repository still ships no Apple software, and nothing is downloaded without the command of the user. The decision A22 stays true, with one addition: the tool can fetch the assets when it is asked to |
| A56 | The sources of the assets | The installer comes from the recovery service of Apple, the same one that the tool `macrecovery` of OpenCorePkg uses. The chunk list and the signature of Apple are checked before the image is used, because the service of Apple answers over plain HTTP. The boot loader and the firmware of QEMU come from the repository of OSX-KVM, which holds the pair that belongs together |
| A57 | The image of the installer | The service of Apple hands a DMG, and QEMU cannot boot a DMG. The tool converts it with `dmg2img` into a raw disk. The tool does not implement the format of the DMG itself, because the converter is a small program that every distribution of Linux already has or can install |
| A58 | The size of the binary | The limit of the CI moves from 6 MB to 8 MB. The command `image prepare` needs a client of HTTP and the arithmetic of big numbers for the signature of Apple, and both grow the binary by about three megabytes. The alternative, a hand written client of HTTP and a program of the shell for the downloads, costs more than the megabytes and is less safe |
| A60 | The shared folder | `macxvm share` copies a folder both ways with `rsync` over the SSH of the machine. macOS has no driver for the filesystems that QEMU can share without a server (`virtiofs` and `9p`), and a share of SMB or NFS would need a service and root on the host. The copy needs neither, and it works on the machine that the tool already has |
| A61 | The deletions of a shared folder | A two-way sync never deletes a file, because a file that one side does not have can be a new file of the other side, and `rsync --delete` in both directions would delete it before the other direction sees it. A deletion is copied only with `push --delete` or `pull --delete`, which names the direction |
| A59 | The appearance of the guest | The guest of the example uses the dark appearance, and the background of the login window is the black colour of the system, so that a guest that cannot draw the video wallpaper of Sonoma does not show a white screen. The look is set in the guest, not in the tool, because the tool does not touch the system of a guest |
| A62 | The SSH connection of an operation | The long operations (`slim`, `tools`, `xcode`, `build`, `share`) hold one SSH connection for their whole run through `sshx.Begin` and `sshx.End`, with `ControlMaster=auto` and a short `ControlPersist=30`, so the sequential commands of an operation ride one handshake. The control socket lives in the run directory of the tool, the master is closed when the operation returns, and a master that an interrupted operation leaves behind dies within thirty seconds. The tool still runs no daemon |
| A63 | The layout of the prepared disk | The partition of a disk that `macxvm image prepare` writes starts at the 1 MiB boundary and ends before one, the layout that the partitioning tools of the big systems write, so that `sgdisk -v` reports no problem on it. A disk too small for the boundary keeps the first-34-sectors layout |
| A64 | The steps of the installation in the output | `macxvm start --install` prints the erase of the disk as APFS before the reinstall, because the installer of macOS and the Startup Disk panel list no target disk before the disk holds an APFS volume. The screen of the guest says so and the tool repeats it, so that the first installation does not stop at an empty disk list |
| A65 | The screenshot script | `scripts/screenshot.sh` takes the screen of a running guest over the QMP socket of the tool with `python3` only, so that the screenshots of the README can be refreshed at every stage of the installation. The fallbacks of the state and the run directory mirror `internal/paths` exactly |
| A66 | Default resources | `cli` defaults are 4 vCPUs / 8192 MiB / 64 GiB logical qcow2; `xcode` remains an explicit 4 / 8192 / 100 alternative. These are logical resources, not APFS free-space or GUI-performance promises |
| A67 | Create and replace | `image prepare` and `create` share resource resolution and host/KVM checks. `--replace` is the destructive replacement opt-in; `--force` is only for experimental resources |
| A68 | Asset authenticity | Boot references are pinned and SHA-256 checked. Apple recovery uses RSA-signed chunklists; digest-only method 2 is rejected. Legacy unversioned Apple files are not assigned to a new macOS version |
| A69 | Atomic disks and clones | GPT is written to a sparse staging raw disk, converted once to qcow2, checked and atomically published. Clone validates disk/NVRAM/args expansion and never leaves a partial destination |
| A70 | Bootstrap trust | Passwords are accepted only through hidden input or stdin and sent through stdin to SSH/sshpass; no secret is persisted. Sudoers is validated with `visudo` and `sudo -n`, not installed with an unverified echo |
| A71 | Readiness and process safety | QMP and SSH readiness are distinct. Every operation reloads config after `op.lock`; exact QEMU markers are rechecked before signals and stale snapshots are rejected |
| A85 | CI job layout | The push-triggered `ci` workflow contains only the `test` job, so a normal run shows a single result. The KVM test is a separate `integration` workflow with `workflow_dispatch` and a required `base` input; it never appears as a skipped job beside a push run. |
| A84 | Automatic viewer | `start --vnc`, `autoinstall` and `finish` open the first available viewer after the VM is ready. `--no-open` and `MACXVM_NO_OPEN=1` disable it, and a missing viewer is a warning, never a failed start. |
| A72 | VNC | VNC is loopback-only and allocated under a global lock. `open` uses the real listener and does not wait for SSH; a running headless VM needs explicit `--restart` |
| A73 | Selective applications | Level 3 and `apps` use one fixed default allowlist, dry-run, confirmation, keep rules, bundle/SIP/symlink/file-flag checks and manifests. Mail, Messages and FaceTime are dock-only; no bulk deletion of caches/logs/user data is allowed |
| A74 | Rollback | Restore means services and update policy only. Removed applications are rolled back by deleting a clone and creating another from the sealed base |
| A76 | Fully automated install | `autoinstall` uses QMP screendumps + tesseract OCR, a fixed read-only seed ISO for exact-size disk selection, and vncdo pointer input. It stops at the first Setup Assistant page; the Apple license needs `--accept-license`, and no password is typed. |
| A77 | Post-Setup automation | `finish` performs key/sudo/Dark/slim/Bochs/apps/verify/seal after the user creates the account and enables Remote Login. It never claims to restore removed applications. |
| A78 | Display reverse engineering | `AppleVirtualGraphics.kext` matches Bochs (`0x1234:0x1111`) and Cirrus only. The default is `-vga none` + `bochs-display` so macOS loads `AppleBochVGAFB`; Metal/3D and RTX passthrough are out of scope. The full map of the guest kexts, and the paravirtual GPU that macOS does claim, are in A88 and A90 |
| A86 | CPU flags and defaults | The default CPU model exposes AVX2, FMA, BMI1/2 and SHA-NI, and `cli` defaults to 6 vCPU / 12 GiB. `-cpu host` was measured and does not boot macOS on the reference host, so it stays documented as unsupported. |
| A87 | Default resolution | The default Bochs mode is 1440x900 because the frame buffer is blitted by the CPU; idle `WindowServer` dropped from 7.5% to 2.5%. `optimize --width/--height` accepts 1024x640 to 3840x2160. |
| A83 | Installer media | The shared `BaseSystem.img` is mounted with `readonly=on`, and `argsfile.Validate` rejects a writable one. A live install proved that a writable image is modified by the guest and breaks the manifest for every VM; it is restored deterministically from the signature-verified DMG. |
| A79 | macOS template location | The default machine definition and CPU models are data in `resources/args-template.txt` and `resources/cpu-models.txt`, not constants in Go source. Pinned download URLs live in `resources/boot-assets.txt`. This satisfies the contribution policy while keeping `image prepare` reproducible. |
| A80 | Corrupt VM visibility | `store.Scan` returns readable VMs plus per-directory problems. `list` and `base list` print a warning to stderr and keep stdout valid, so a broken VM is visible without breaking JSON consumers. |
| A81 | Unused assets | The unreferenced level 3 stub, disk-erased screenshot and six unreferenced logo files were deleted. The two README banners remain. |
| A82 | CI action versions | `actions/checkout@v7` and `actions/setup-go@v7` resolve to v7.0.1 and v7.0.0; the major-version pins are valid. |
| A75 | Release and integration | v0.9.0 uses matching ldflags, and v0.9.0 is the first tag after v0.6.0; it supersedes the unreleased v0.7.0/v0.8.0 milestones. The `ci` workflow runs formatting, headers, shell syntax, vet, tests and build on every push; KVM integration lives in the separate `integration` workflow, runs only by manual dispatch on a self-hosted runner, and needs `MACXVM_INTEGRATION=1` plus a named sealed base |
| A88 | The source of truth for the supported devices | The IOKit personalities of the guest kexts, not the names of the kexts. `macxvm accel NAME --re` reads `/System/Library/Extensions/<kext>.kext/Contents/Info.plist` from the running guest, converts it with `plutil` and parses it with a small XML property list reader, so the project keeps zero dependencies and the report stays evidence |
| A89 | The system disk on NVMe | `IONVMeFamily` claims the PCI class code `0x01080200`, so macOS drives every standard NVMe controller, and `macxvm accel NAME --apply` moves the system disk from `ide-hd` to `nvme`. Measured on the reference host: 694 to 984 MiB/s on a 512 MiB write with `fsync`, 6.6 to 10.1 GiB/s on the read, and TRIM on. The boot loader disk stays on AHCI, so the boot path of the firmware does not change, and the APFS volume keeps its identifier |
| A90 | The display stays on the 2D frame buffer | macOS claims no virtio GPU: the virtio device type `0x10` is absent from `AppleVirtIO.kext`. The only Metal capable identity that macOS claims is Apple's own `0x106b:0xeeee`, which binds straight to `IOPCIDevice` and is not a virtio device, so no existing QEMU device can be reused for it. The target is recorded as an open item instead of being faked with a wrong device |
| A91 | The passthrough on the host | The host side is made ready on request: `--enable-iommu` writes `intel_iommu=on iommu=pt` to every kernel entry with `grubby`, and `--bind-vfio` unbinds the card and binds it to `vfio-pci`. Both need root and, for the first one, a reboot. The tool refuses to bind the only display device of the host, because that would remove the desktop of the host. The guest side stays a documented limit: macOS has no driver for any NVIDIA GeForce card |
| A92 | The virtio devices that the tool exposes | Only the three that macOS drives natively and that QEMU implements well: the entropy, the balloon and the network. The 9p, the fs, the sound and the vsock devices exist in the map of the guest, but macOS has no tool to mount a 9p or an fs share from the command line, and the QEMU implementations of the sound and the vsock devices do not match the protocol that Apple implements |
| A93 | The size gate of the CI | The limit moves from 8,388,608 to 10,485,760 bytes. The binary of the v0.10.0 cycle is 8,196,256 bytes, so the old gate left 192,304 bytes and the next feature that added a few kilobytes would have failed the build for a reason that has nothing to do with the quality of the change. The decision A54 raised the same limit once, from 6 MB to 8 MB |
| A94 | The forbidden flags of an args file | The validator rejects the flags that MacXVM owns with one dash and with two dashes, so `--display` cannot slip past a check that rejects `-display`. QEMU rejects the double dash form itself, so the change only makes the message of the tool early and clear |
