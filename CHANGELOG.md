# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- fix: a trailing backslash at the end of a line of the args file is dropped
  instead of becoming an argument, so that the example of the README and the
  boot script of another project work as they are
- fix: the SSH step of a stop is bounded on its own, so that a guest that does
  not answer no longer uses the whole timeout before the other steps run

### Added
- feat: `macxvm doctor` reports whether QEMU can use the KVM accelerator and
  whether it selects KVM on its own, and it names the accelerator to add when
  it does not

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
