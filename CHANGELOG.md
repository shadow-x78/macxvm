# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.2] - 2026-10-22

### Added
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
- feat: `macxvm image prepare` writes a GUID partition table on the disk of the
  machine, so that the firmware and the installer of Apple see the disk. A disk
  of QEMU starts as a file of zeros and the installer does not list it

### Changed
- the binary is 7.32 MB now, because the command `image prepare` needs a client
  of HTTP and the arithmetic of big numbers. The limit of the CI moves from
  6 MB to 8 MB

### Fixed
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
