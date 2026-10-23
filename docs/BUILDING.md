<div align="center">

# Building Projects - MacXVM

[![Version](https://img.shields.io/badge/version-0.5.1-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="BUILDING.md">🇬🇧 English</a> · <a href="BUILDING_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [Overview](#overview)
- [Project Detection](#detection)
- [The macxvm.json File](#project-file)
- [The Work Directory](#workdir)
- [The Sync](#sync)
- [The Build Command](#build-command)
- [Fetching Results](#fetch)
- [Ephemeral Builds](#ephemeral)
- [The Up and Down Workflow](#up-down)
- [Code Signing](#signing)
- [Profiles](#profiles)

---

<a id="overview"></a>
## Overview

`macxvm build` builds a project inside a guest: the project is copied over SSH, the build runs there and the output is streamed back to the terminal. The exit code of the build is the exit code of the command.

```bash
macxvm build --project . --scheme App
macxvm build --vm work --project . --scheme App --configuration Release
macxvm build --ephemeral --from base-xcode --project . --scheme App
```

The machine is only started when it is not running yet, and it is stopped again afterwards, even when the build fails. `--keep` leaves it running.

<a id="detection"></a>
## Project Detection

The kind of the project is detected at the top level of the project directory:

| What is there | Kind | Build |
|--------------|------|-------|
| `*.xcworkspace` | xcode | `xcodebuild -workspace ...` |
| `*.xcodeproj` | xcode | `xcodebuild -project ...` |
| `Package.swift` | swiftPackage | `swift build` |
| nothing of these | n/a | error `unsupported project` |

A `.xcodeproj` inside a `.xcworkspace` is ignored, and a workspace wins over a project.

A project of the `xcode` kind needs a machine of the profile `xcode`. A machine of the profile `cli` has no Xcode, and the build stops with exit code 9:

```
macxvm: error: this profile has no Xcode; use profile 'xcode'
```

<a id="project-file"></a>
## The macxvm.json File

`macxvm.json` in the root of the project holds the defaults:

```json
{
  "version": 1,
  "profile": "xcode",
  "vm": "dev",
  "base": "base-xcode",
  "build": {
    "scheme": "App",
    "configuration": "Debug",
    "destination": "platform=iOS Simulator,name=iPhone 16"
  },
  "sync": { "exclude": ["node_modules"] },
  "fetch": [ { "remote": "build/App.app", "local": "./out/App.app" } ]
}
```

| Key | Meaning |
|-----|---------|
| `version` | required, must be `1` |
| `profile` | `cli` or `xcode` |
| `vm` | the machine the build runs in |
| `base` | the sealed base image the machine is cloned from |
| `build.scheme` | the scheme to build |
| `build.configuration` | `Debug` or `Release` |
| `build.destination` | the destination of the build |
| `sync.exclude` | extra paths that are never sent to the guest |
| `fetch` | the paths that are copied back after a successful build |

Every key except `version` is optional, and unknown keys are ignored. The flags of the command line always win over the file.

`macxvm init` writes a minimal file, with the profile guessed from the project:

```bash
macxvm init          # writes {"version": 1, "profile": "xcode"}
macxvm init --force  # overwrites an existing file
```

<a id="workdir"></a>
## The Work Directory

The project is copied into a directory of the guest that is derived from the path of the project:

```
~/macxvm-work/<first 12 characters of the SHA-1 of the absolute project path>
```

The same project always lands in the same directory, so the derived data and the build cache are reused between the builds.

<a id="sync"></a>
## The Sync

```bash
rsync -a --delete \
  --exclude .git --exclude DerivedData --exclude build --exclude .build \
  -e "ssh <the options of MacXVM>" \
  <project>/ admin@127.0.0.1:~/macxvm-work/<hash>/
```

- `-a` without `-z`: the files are compressed by the disk of the guest anyway, and the compression of the network costs more time than it saves.
- The excluded paths are protected from `--delete`, so the `DerivedData` and the `.build` directory of the guest survive the next sync.
- The excludes of `macxvm.json` are added to the list.

<a id="build-command"></a>
## The Build Command

For an Xcode project:

```
cd ~/macxvm-work/<hash> && xcodebuild \
  [-workspace <workspace> | -project <project>] \
  -scheme <scheme> -configuration <configuration> \
  [-destination <destination>] \
  -jobs <cpus> \
  -derivedDataPath ~/macxvm-work/<hash>/DerivedData \
  COMPILER_INDEX_STORE_ENABLE=NO \
  [ONLY_ACTIVE_ARCH=YES] \
  build [the extra arguments]
```

- `-jobs` defaults to the number of CPUs of the guest.
- `COMPILER_INDEX_STORE_ENABLE=NO` skips the index of the compiler, which is the biggest waste of time in a build that nobody reads.
- `ONLY_ACTIVE_ARCH=YES` is added for a `Debug` build, so that only the current architecture is compiled.
- Without `--scheme`, the guest is asked for its schemes. A project with exactly one scheme uses it, a project with several schemes stops and asks for `--scheme`.

For a Swift package:

```
cd ~/macxvm-work/<hash> && swift build -c <configuration> --jobs <cpus> [the extra arguments]
```

The extra arguments of the command line come after a `--`:

```bash
macxvm build --project . --scheme App -- -quiet
```

<a id="fetch"></a>
## Fetching Results

```bash
macxvm build --project . --scheme App --fetch build/App.app=./out/App.app
```

The result is copied back with `scp -r` after a successful build. Both paths must be relative and must stay inside the project: a `fetch` list of a cloned repository may not write anywhere else on the host.

The `fetch` list of `macxvm.json` is honoured as well.

<a id="ephemeral"></a>
## Ephemeral Builds

```bash
macxvm build --ephemeral --from base-xcode --project . --scheme App
```

The build runs in a throwaway overlay of the sealed base image. The overlay is created, started, used and removed again, also when the build fails or when the command is interrupted. The machine uses the fast and unsafe cache mode, which is only allowed for a throwaway machine.

`--keep` keeps the machine, so that you can look at it.

<a id="up-down"></a>
## The Up and Down Workflow

```bash
macxvm up      # clones the base if needed, starts the machine, waits for SSH
macxvm down    # stops the machine gracefully
macxvm down --delete   # stops it and removes the overlay
```

`macxvm up` creates the machine from the `base` of `macxvm.json` when it does not exist yet, and starts it. `macxvm down --delete` only removes a machine that was cloned from that base; anything else needs `--yes`.

<a id="signing"></a>
## Code Signing

Code signing over SSH needs an unlocked login keychain. For a build that does not need a signature, use a simulator destination or `CODE_SIGNING_ALLOWED=NO`:

```bash
macxvm build --project . --scheme App --destination "platform=iOS Simulator,name=iPhone 16"
```

To unlock the keychain of the guest, run this in the guest:

```bash
security unlock-keychain
```

The guest is x86_64, so a simulator build produces an x86_64 simulator. A build for a device compiles normally, but it cannot be run in the simulator of the guest.

<a id="profiles"></a>
## Profiles

| Profile | Contents | `swift build` | `xcodebuild` | iOS SDK |
|---------|----------|---------------|--------------|---------|
| `cli` | slim macOS + Command Line Tools | yes | no (exit 9) | no |
| `xcode` | slim macOS + Command Line Tools + Xcode | yes | yes | yes, software rendered |

The Command Line Tools contain the macOS SDK only. The profile `cli` is therefore for Swift packages, Homebrew, git and scripts.
