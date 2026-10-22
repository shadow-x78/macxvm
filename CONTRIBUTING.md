<div align="center">

# Contributing to MacXVM

[![Version](https://img.shields.io/badge/version-0.3.0-007AFF?style=flat-square&logo=semver)](CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Table of Contents

- [Day-to-Day Work](#day-to-day-work)
- [The Rules](#rules)
- [Manual QA](#manual-qa)
- [Release Process (maintainers)](#release-process)
- [Verification Checklist](#verification-checklist)

---

Contributions to MacXVM are welcome. Please note that this project is released with a [Contributor Code of Conduct](CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## Day-to-Day Work

- **Branches:** `feature/`, `fix/`, `docs/`, `chore/` prefixes (e.g. `feature/overlay-clone`), branched from `main`.
- **Commits:** `macxvm | v<version> | <type>: <description>` for the work of a release cycle, and `macxvm | v<version> | release: <summary>` for the commit that a release is cut from. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `release`. Every release moves the version, and the tag, the name of the release (`MacXVM v<version>`) and the assets (`macxvm-v<version>-linux-amd64` with its `.sha256`) follow the same number.
- **Style:** every Go file is formatted with `gofmt`, and every file carries the header of the project:

```go
// MacXVM - <module name> (MIT)
// https://github.com/shadow-x78/macxvm
```

- **Toolchain:** Go 1.22 or newer. `make build` produces a static binary for `linux/amd64`.
- **Pull requests:** from a fork, targeting `main`, with the tests green, a `CHANGELOG.md` entry, and the pull request template filled in.

## The Rules

1. **The standard library only.** `go.mod` never gets a `require` line, `CGO_ENABLED=0` is always used and the binary stays static.
2. **No macOS on QEMU details in the code.** No CPU model, no SMC or OSK values, no SMBIOS values, no OpenCore settings, no firmware paths. They belong in the args file of the user.
3. **No Apple software in the repository.** No installer images, no boot loaders, no firmware, no keys.
4. **No placeholders.** No `TODO`, no `panic("not implemented")`, no commented out code, no fake output.
5. **Every subprocess is an argument vector.** `os/exec` never receives a shell command, and every value that comes from the user is quoted for the remote shell.
6. **Every deviation goes into the decisions.** Every deviation from the specification and every ambiguity that had to be resolved goes into [docs/DECISIONS.md](docs/DECISIONS.md).
7. **The documentation is bilingual.** A new guide exists in English and in Arabic, and the README and the doc tables list both.

## Manual QA

The unit tests cover everything that can be tested without a guest. The behaviour that needs a real macOS guest is checked by hand, on a Linux machine with KVM and with the assets of the user:

1. `macxvm create` produces a sparse disk, copies the args file and validates it.
2. `macxvm start --install --vnc` prints the VNC port and the machine boots.
3. The first boot checklist of the README works: user, Remote Login, passwordless sudo, `macxvm ssh-key --install`.
4. `macxvm slim --level 1` and `--level 2 --measure` lower the process count, and SSH and `xcodebuild -version` still work.
5. `macxvm base seal` followed by `macxvm clone` produces an overlay that boots with its own SSH port and MAC address.
6. `macxvm run --from base -- xcodebuild -version` works and leaves no `eph-*` machine behind, also after an interruption.
7. `macxvm build` builds a sample project, and a `cli` machine refuses an Xcode project with exit code 9.

## Release Process (maintainers)

A release is ONE commit containing the pending work plus the version bump:

1. Bump the version in every version home, so that nothing can disagree with the tag:
   - `internal/cmds/cmds.go` (`Version`)
   - the version badge in `README.md`, `README_AR.md`, `CONTRIBUTING.md` and `SECURITY.md`
   - the badge lines of every document in `docs/`
2. Rotate the pending `CHANGELOG.md` entry into a `## [vX.Y.Z] - <date>` block dated today.
3. Pick the bump by the changelog convention: `Added` work is a minor, `Fixed` only is a patch.
4. Verify locally, commit everything as one release commit, and tag it:

```bash
git add -A
git commit -m "macxvm | vX.Y.Z | <type>: one-line summary of the release"
git tag -a vX.Y.Z -m "vX.Y.Z - <one-line summary>"
```

Then push the result:

```bash
git push origin main vX.Y.Z
```

## Verification Checklist

```bash
gofmt -l .
go vet ./...
go test ./...
./scripts/check-headers.sh
make build
```

The binary must stay below 6 MB, and `macxvm doctor` must end with the legal notice.
