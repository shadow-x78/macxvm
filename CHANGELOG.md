# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.8.0] - 2026-10-27

### Added
- `macxvm autoinstall NAME --yes [--accept-license]`: automates OpenCore,
  Recovery Terminal, exact-size disk selection and erase, license (explicit
  flag only), target disk and Install, then stops at the first Setup Assistant
  page before any user account.
- `macxvm finish NAME [--password-stdin] [--apps] [--seal]`: post-Setup key,
  sudoers, Dark, power, slim level 1, Bochs display, app preview/removal,
  verification and optional seal.
- `macxvm gpu NAME [--json]` and `macxvm optimize NAME`: host IOMMU/GPU report
  plus the real macOS display kext, and a stopped-VM switch to Bochs VGA.

### Changed
- The default `cli` profile is now 4 vCPUs / 8192 MiB / 64 GiB. There is one
  default VM; the `xcode` profile remains an explicit option.
- New VMs use `-vga none` plus `bochs-display,vgamem=256M,xres=1920,yres=1080`,
  so macOS loads `AppleBochVGAFB` instead of “No Kext Loaded”.
- Application set defaults are now `default`; `personal` remains accepted as a
  compatibility alias. `apps-default.txt` replaces the old policy file name.

### Audit
- Moved the default macOS machine template, CPU models and pinned asset URLs
  from Go constants into reviewed data resources.
- `list` and `base list` now warn about an unreadable VM instead of hiding it.
- Removed unreferenced installer helpers, the level 3 shell stub, an unused
  screenshot and six unused logo files.
- Hardened installer disk selection against the real `diskutil` fields, Setup
  detection, resume behavior, process ownership, share locking, glob excludes
  and rsync quoting.

### Notes
- `AppleVirtualGraphics.kext` matches only Bochs and Cirrus. There is no Metal
  or 3D acceleration, and NVIDIA RTX passthrough is unavailable on a host with
  no IOMMU groups and no macOS driver for the card.
- `autoinstall` temporarily uses `vmware-svga` because Recovery accepts reliable
  absolute pointer input with it; `finish` switches to Bochs afterwards.

## [v0.7.0] - 2026-10-26

### Added
- CLI profile defaults at the time: 2 vCPUs, 4096 MiB RAM and a 64 GiB logical
  qcow2 disk. `create NAME --base BASE` clones a sealed default base without
  reinstalling; `resize` grows stopped disks and changes stopped VM resources.
- `base bootstrap` with resumable, non-secret stages, local VNC, TOFU SSH trust,
  key-only SSH and validated sudo bootstrap.
- `open`, `start --wait`, readiness states, independent per-VM NVRAM and
  per-version asset manifests with pinned boot references and SHA-256 checks.
- Selective `apps` removal with the default allowlist, dry-run, confirmation,
  `--keep`, SIP/symlink/bundle protection and recoverable-space manifests.
- `scripts/integration.sh` for explicitly opted-in KVM guest testing and a
  benchmark that reports `start_to_ssh_seconds` and cleans up on failure.

### Changed
- `image prepare` uses the same resource defaults as `create`; disks are built
  in staging, converted once to qcow2 and checked before atomic publication.
- `--replace` replaces stopped VMs; `--force` only overrides experimental
  resource limits. Base/overlay deletion and resize protection is enforced.
- SSH uses a per-VM `known_hosts` with a stable VM identity alias and strict
  verification after local TOFU; bootstrap never stores passwords.
- Level 3 is now the selective application engine, not a bulk deletion script.
  Restore covers services and update policy only, never removed applications.

### Fixed
- Interrupted downloads and failed conversions no longer publish final assets.
- Process ownership is checked before stop/kill and after operation locks.
- Missing pid files no longer hide a running VM, and readiness is not inferred
  from a QMP socket alone.

## [v0.6.0] - 2026-10-24

### Added
- `scripts/screenshot.sh NAME [OUT.png]`: takes the screen of a running guest
  over the QMP socket of MacXVM, for fresh screenshots at every stage
- `macxvm start --install` prints the steps of the installation after the
  start, with the erase of the disk as APFS first, because the installer and
  the Startup Disk panel show no target disk before that

### Changed
- the console design follows the reference CLIs: no frames, dim uppercase
  section headers, aligned tables with a header row, one-line banner and
  status symbols in their own colours; the help groups the commands under
  headers and the `list` output is a real table
- every long operation (`slim`, `tools`, `xcode`, `build`, `share`) holds one
  SSH connection for its whole run, so the many remote steps of an operation
  ride one handshake instead of paying one per command
- the history carries one commit style: every subject reads
  `macxvm | v<version> | <type>: <summary>`; the forty-nine older subjects
  that wrote `0.2.2` without the `v`, or no version at all, were rewritten to
  the same style and the tags follow them

### Fixed
- the partition of the prepared disk starts and ends on the 1 MiB boundary,
  the layout the partitioning tools of the big systems write, and `sgdisk`
  reports no problem on it

## [v0.5.3] - 2026-10-23

### Changed
- the mark of the help is smaller, seven lines, with the version, the tagline
  and the usage beside it, and the cards are 66 columns wide
- the checks of `macxvm doctor` carry short names, so that every line of the
  report holds one check

## [v0.5.2] - 2026-10-23

### Fixed
- fix: the mark of the help is the logo of the banner: the screen with its
  bezel, the window of the guest inside it and the stand below, in the blue and
  the purple of the logo
- fix: the code carries no comment except the credit header and the directives
  of the toolchain, and the configuration files carry the credit header only
- fix: the guest of the example is dark everywhere: the login window and the
  desktop are black, and Terminal, Safari and Xcode use their dark theme. A
  LaunchAgent of the guest puts the black wallpaper back at every login,
  because macOS 14 resets a wallpaper that was set outside System Settings

## [v0.5.1] - 2026-10-23

### Fixed
- fix: the mark of the help is the logo of the project, a ring in the blue of
  macOS with a window of the guest on its lower right edge, and the colours of
  the console come from the logo, not from another project
- fix: every card of the console is 74 columns wide, with a border on both
  sides, and a line that is too long is wrapped at the column of its value
- fix: `macxvm start` on a machine that is already running says so and exits
  with 0, instead of failing
- fix: `macxvm share sync` and `share push` work on every shared folder when
  no `--id` names one, and a folder that cannot be copied does not stop the
  others

## [v0.5.0] - 2026-10-23

### Added
- feat: the help opens with the mark of the project and lists the commands in
  the groups Machines, Boot assets, The guest, Projects and This host, with a
  card frame and the colours of the terminal. The colours are turned off by
  `NO_COLOR` and by a terminal that is not a TTY
- feat: `macxvm doctor` and `macxvm list` use the same frame and the same badges

### Changed
- the tagline of the tool carries no em dash, and the sources carry none

## [v0.4.1] - 2026-10-23

### Fixed
- fix: a share leaves out the caches, the build trees, the repositories of git
  and the images of a machine, and it always leaves out the state directory of
  MacXVM, so that a copy of a home directory cannot copy the disk of the
  machine into the guest
- fix: `share sync` and `share push` start a machine that is stopped

### Added
- feat: a share measures the folder and the free space of the guest, and it
  refuses a copy that does not fit, because a copy that fills the disk of the
  guest leaves a machine that does not boot
- feat: `share add --exclude PATTERN` adds a pattern to leave out

## [v0.4.0] - 2026-10-23

### Added
- feat: `macxvm share` keeps a folder of the host in step with a folder of the
  guest, both ways, over the SSH of the machine. `share sync --watch` repeats
  the sync, `share push` and `share pull` copy one way, and `--delete` makes a
  deletion explicit

## [v0.3.1] - 2026-10-23

### Changed
- the desktop and the login window of the guest of the example are black, and
  the guest uses the dark appearance, because a guest without a GPU draws the
  video wallpaper of Sonoma as a white surface
- the screenshots of the README show the dark guest

## [v0.3.0] - 2026-10-22

### Added
- feat: the guest of the example comes with the dark appearance, and the
  background of the login window is the black colour of the system instead of
  a wallpaper
- feat: `macxvm image prepare` fetches the installer of Apple and the OpenCore
  boot loader, writes an args file for macOS and registers the machine. The
  chunk list and the signature of Apple are checked before the image is used,
  and the image is converted into a raw disk with `dmg2img`
- feat: `macxvm image link` prints the addresses of the boot assets without
  downloading anything. No Apple software is ever stored in the repository or
  in a release of the project
- feat: the slim level 3 removes the applications that a build machine does not
  need, the leftovers of the installation, the updates, the caches, the logs
  and the simulator runtimes. Safari and Xcode stay
- feat: the args file of the tool carries the flags that make the guest fast:
  the timer of the guest without the lag, the table of ACPI that macOS needs,
  the shape of the processors and the model of the processor of the host
- feat: `macxvm image prepare --cpu host` uses the processor of the host, which
  is the fastest, and `macxvm xcode install` leaves out the platform runtime
  unless it is asked for
- feat: `macxvm xcode install` runs the first launch of Xcode with `sudo`, which
  the installation of the additional components needs. Without it the install
  failed at the last step with exit code 75
- feat: `macxvm image prepare` writes a GUID partition table on the disk of the
  machine, so that the firmware and the installer of Apple see the disk. A disk
  of QEMU starts as a file of zeros and the installer does not list it

### Changed
- the binary is 7.20 MB now, because the command `image prepare` needs a client
  of HTTP and the arithmetic of big numbers. The limit of the CI moves from
  6 MB to 8 MB

### Fixed
- fix: the request to the recovery service of Apple sends the parameters
  separated by lines, which is the form the service accepts. The form of the
  query string was refused with "Request parameters malformed", so the tool
  could never fetch an installer by itself
- fix: the example of the args file attaches the network card to the network of
  QEMU with `netdev=net0`. Without it the card stays outside the network, the
  port of SSH is never forwarded, and every command that needs the guest fails
- fix: the example names the accelerator of QEMU with `-accel kvm`, so that a
  machine that asks for the host processor does not fall back to the software
  emulation of the processor

## [v0.2.1] - 2026-10-22

### Changed
- the version of the tool and the badges of the documents are 0.2.1

## [v0.2.0] - 2026-10-22

### Fixed
- fix: a trailing backslash at the end of a line of the args file is dropped
  instead of becoming an argument, so that the example of the README and the
  boot script of another project work as they are
- fix: the SSH step of a stop is bounded on its own, so that a guest that does
  not answer no longer uses the whole timeout before the other steps run
- fix: the full copy of a machine is a standalone disk, so that it keeps
  working when the base image is deleted
- fix: a configuration whose name leaves the state directory is refused, so
  that a broken file cannot make a delete reach outside of it
- fix: a rejected memory size or a refused host no longer leaves a temporary
  virtual machine behind
- fix: the refusal of the host protection exits with the usage code, like
  every other wrong argument
- fix: the public key is added to the guest only once
- fix: the console log of a machine holds the messages of the last start only
- fix: the check of the KVM accelerator of `macxvm doctor` does not depend on
  the host that runs the tests. The check of the binary and the probe of the
  accelerator are replaced by the tests, so that the suite passes on a host
  without QEMU

### Changed
- the console log of a machine is written with the mode 0600
- `macxvm down --delete` names the base of the project file in its message

### Added
- feat: `macxvm doctor` reports whether QEMU can use the KVM accelerator and
  whether it selects KVM on its own, and it names the accelerator to add when
  it does not

### Security
- the name of a stored configuration is validated when it is read, so that a
  broken or hand-edited file cannot make a delete reach outside of the state
  directory

## [v0.1.0] - 2026-10-22

### Added
- feat: the first release of MacXVM, a launcher for slim macOS guests on a Linux host

MacXVM registers, starts, stops, clones and deletes macOS guests that run under
QEMU/KVM, and it manages the tools inside them. The machine definition is never
built into the tool: the user brings an args file, MacXVM substitutes a handful
of placeholders into it and starts QEMU with the result.

- `macxvm create` registers a machine, creates a sparse qcow2 disk, validates the
  args file and writes the configuration.
- `macxvm start` and `macxvm stop` manage the QEMU process, over the QMP
  socket and, when possible, over SSH.
- `macxvm clone` and `macxvm base seal` provide sealed base images with qcow2
  overlays: a fresh machine costs a few hundred kilobytes and a second.
- `macxvm run` builds a throwaway machine, runs one command in it and removes it
  again, including when the command is interrupted.
- `macxvm slim` trims the guest in two levels, with a deny list that protects
  SSH and the network, and a restore that undoes the change.
- `macxvm tools` installs the Command Line Tools and Homebrew, `macxvm xcode`
  installs Xcode inside the guest.
- `macxvm build` copies the project over SSH and runs `xcodebuild` or
  `swift build` in the guest, with the derived data kept between the builds.
- `macxvm doctor` checks the host and ends with the legal notice.
- `macxvm init`, `macxvm up` and `macxvm down` drive a project from its
  `macxvm.json` file.

### Security
- The QMP socket lives in a directory that is checked for ownership and mode,
  because the protocol itself is not authenticated.
- A machine that does not become ready is stopped again, so that a retry can
  never start a second QEMU on the same disk.
- The paths of a fetch must stay inside the project, so that a project file of
  a cloned repository cannot write anywhere else on the host.
- The arguments of the SSH commands are built as vectors, and every value that
  comes from the user is quoted for the remote shell.

### Documentation
- The documentation is bilingual: every guide exists in English and in Arabic.
- The legal notice, the host requirements and the limits of the project are
  stated in the README, in `macxvm doctor` and in `SECURITY.md`.
