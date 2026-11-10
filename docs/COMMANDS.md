# Command Reference

Every command of `macxvm`, with the flags it accepts. The [README](../README.md)
shows the short list of the commands you will actually use; this page is the
reference.

Run `macxvm <command> --help` for the same information from the tool itself,
and `macxvm` alone for the grouped list.

---

## Contents

- [Machines](#machines)
- [Boot assets](#assets)
- [The guest](#guest)
- [Projects](#projects)
- [The host](#host)
- [Exit codes](#exit-codes)

---

<a id="machines"></a>
## Machines

| Command | What it does |
|---------|--------------|
| `macxvm create NAME (--base BASE \| --args-file FILE)` | register a virtual machine |
| `macxvm list [--json]` | list the registered machines |
| `macxvm status NAME [--json]` | show the state of a machine |
| `macxvm start NAME [--display install\|daily\|recovery] [--vnc] [--wait] [--force]` | start the machine |
| `macxvm stop NAME [--force] [--timeout SECONDS]` | stop the machine |
| `macxvm open NAME [--restart]` | open the loopback VNC display without waiting for SSH |
| `macxvm resize NAME [--cpus N] [--ram SIZE] [--disk GiB] [--force]` | resize a stopped machine; the disk only grows |
| `macxvm autoinstall NAME --yes [--accept-license] [--timeout MINUTES]` | install macOS through Recovery and stop at the first Setup Assistant page |
| `macxvm finish NAME [--password-stdin] [--apps] [--keep APP ...] [--seal]` | after Setup Assistant: SSH key, sudo, Dark mode, slim, display, optional seal |
| `macxvm optimize NAME [--check] [--width N] [--height N]` | switch a stopped machine to the Bochs display |
| `macxvm accel NAME [--check] [--apply] [--re] [--json]` | compare the machine with the devices macOS drives natively |
| `macxvm gpu [NAME] [--status] [--json]` | report the GPU state of the host or of a machine |
| `macxvm migrate NAME --to USER@HOST [--check\|--live] [--dest NAME]` | check or run the migration of a machine |
| `macxvm migrate NAME --cancel` | cancel a running migration |
| `macxvm delete NAME [--yes]` | delete a machine |
| `macxvm clone SRC DST [--full]` | clone a machine; by default as an overlay of a sealed base |
| `macxvm compact NAME` | shrink the disk of a stopped machine |
| `macxvm prune [--all]` | delete the leftovers of temporary machines |

### Creating a machine

```bash
macxvm create NAME --base BASE
macxvm create NAME --args-file FILE [--assets DIR] [--nvram-template FILE]
```

`--base` and `--args-file` are the two ways to describe a machine, and they
cannot be combined with the flags that change the hardware of the machine:
`--cpus`, `--ram`, `--disk`, `--profile`, `--cpu`, `--gpu-backend`, `--gpu`,
`--audio`, `--host-gpu` and the rest. A clone of a base inherits the hardware
of the base.

| Flag | Meaning |
|------|---------|
| `--profile cli\|xcode` | the resource profile: `cli` is 6 vCPU, 12 GiB and a 64 GiB disk; `xcode` is 4 vCPU, 8 GiB and a 100 GiB disk |
| `--cpus N` | the number of virtual CPUs |
| `--ram SIZE` | the memory, with a suffix: `8G`, `12GiB`, `16384M` |
| `--disk GiB` | the size of the logical disk |
| `--user NAME` | the local administrator of the guest, `admin` by default |
| `--cpu MODEL` | the processor model of the guest, `skylake` by default |
| `--macos-version V` | the macOS version, taken from the assets when it is not given |
| `--replace` | replace a stopped machine of the same name |
| `--force` | override the host limits; never replaces an existing machine |

### The GPU and the display

These are the experimental commands that give the guest a real graphics card.

```bash
macxvm gpu                                      # the host: IOMMU, the cards, the display
macxvm gpu NAME                                 # a machine: the driver, the kext, the readiness
macxvm gpu NAME --status                        # the same, as a short report
macxvm gpu NAME --passthrough                   # what the host still needs for a passthrough
macxvm gpu NAME --enable-iommu                  # turn VT-d on in the kernel command line
macxvm gpu --prepare-vfio --gpu BDF --audio BDF --host-gpu BDF --accept-host-changes
macxvm gpu --unbind-vfio --gpu BDF --restore-host
macxvm gpu NAME --install-driver PACKAGE --accept-driver-changes
macxvm gpu NAME --remove-driver
macxvm gpu NAME --recover
```

The details, the measured numbers and the limits of the passthrough are in
[GPU.md](GPU.md).

### The driver package

```bash
macxvm driver verify PACKAGE         # check the signature and the hashes
macxvm driver info PACKAGE [--json]   # show the contents of a package
macxvm driver build --builder USER@HOST
```

The format, the trust roots and the build of the package are in
[DRIVER.md](DRIVER.md).

<a id="assets"></a>
## Boot assets

| Command | What it does |
|---------|--------------|
| `macxvm image link` | print where the boot assets come from, without downloading anything |
| `macxvm image prepare NAME [--macos VERSION] [--profile cli\|xcode]` | fetch the boot assets and register the machine |

```bash
macxvm image prepare macos --profile cli
```

The tool downloads about 1 GB into the state directory: the macOS installer
from the recovery service of Apple, the OpenCore boot loader and the firmware
of QEMU. Every file is checked against the hash of its manifest before it is
used, and nothing of Apple is stored in this repository.

The supported macOS names are `high-sierra`, `mojave`, `catalina`, `big-sur`,
`monterey`, `ventura`, `sonoma`, `sequoia` and `tahoe`.

<a id="guest"></a>
## The guest

| Command | What it does |
|---------|--------------|
| `macxvm ssh NAME` | an interactive SSH session |
| `macxvm ssh-key [--install NAME]` | show the SSH key, or install it in a guest |
| `macxvm exec NAME -- CMD ...` | run a command; the exit code is passed through |
| `macxvm cp SRC DST` | copy files; a guest path is `NAME:/path` |
| `macxvm share add NAME --host DIR [--guest DIR]` | share a folder between the host and the guest |
| `macxvm share list NAME` | show the shared folders |
| `macxvm share sync NAME [--id ID] [--watch]` | copy both ways and keep the newer file |
| `macxvm share push NAME [--id ID] [--delete]` | copy the host folder into the guest |
| `macxvm share pull NAME [--id ID] [--delete]` | copy the guest folder onto the host |
| `macxvm share remove NAME --id ID` | stop sharing a folder and keep the files |
| `macxvm slim NAME [--level 1\|2\|3] [--keep APP ...] [--dry-run] [--yes] [--measure] [--restore]` | trim the machine |
| `macxvm apps list NAME` | show the applications of the guest |
| `macxvm apps remove NAME --set default [--dry-run] [--yes]` | remove the allowlisted applications |
| `macxvm tools install NAME clt\|brew` | install the Command Line Tools or Homebrew |
| `macxvm tools brew NAME PKG ...` | install Homebrew packages |
| `macxvm tools list NAME` | show the installed tools |
| `macxvm xcode install NAME --xip PATH [--platform iOS]` | install Xcode |
| `macxvm xcode clean NAME` | remove the caches of Xcode |

```bash
macxvm exec macos -- swift --version
macxvm exec macos -- sh -c 'cd /tmp && ls'
macxvm cp ./local.txt macos:/Users/admin/remote.txt
macxvm cp macos:/Users/admin/remote.txt ./local.txt
```

The two-way sync of `share` keeps the newer file of the two and never deletes a
file, because a file that one side does not have may be a new file of the
other side. A deletion is copied only when `push` or `pull` runs with
`--delete`.

<a id="projects"></a>
## Projects

| Command | What it does |
|---------|--------------|
| `macxvm init [--profile P] [--force]` | write a `macxvm.json` in the project directory |
| `macxvm up` | create and start the machine of the project |
| `macxvm down [--delete] [--yes]` | stop the machine of the project |
| `macxvm build [--vm NAME] [--project DIR] [--scheme S] [--configuration Debug\|Release] [--destination D] [--ephemeral --from BASE] [--keep] [--fetch REMOTE=LOCAL] [--jobs N] [-- ARGS]` | build a project in a guest |
| `macxvm run --from BASE [--cpus N] [--ram SIZE] [--project DIR] [--fetch REMOTE=LOCAL] [--keep] -- CMD ...` | run a command in a temporary machine |

```bash
macxvm init --profile cli
macxvm up
macxvm build --scheme MyApp
macxvm down --delete --yes
```

The project file and the build flow are in [BUILDING.md](BUILDING.md).

<a id="host"></a>
## The host

| Command | What it does |
|---------|--------------|
| `macxvm doctor` | check whether this host can run macOS guests |
| `macxvm version` | print the version |
| `macxvm completion bash\|zsh` | print the shell completion script |

```bash
macxvm doctor
macxvm completion zsh > ~/.zsh/completions/_macxvm
```

<a id="exit-codes"></a>
## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | error |
| 2 | wrong usage |
| 3 | not found |
| 4 | already running |
| 5 | not running |
| 6 | unsupported host |
| 7 | timeout |
| 8 | invalid args file |
| 9 | profile or tooling mismatch |

Every error message on stderr names the command that fixes it.

---

Every document of this project exists in English and in Arabic: the
[Arabic reference](COMMANDS_AR.md) mirrors this page.
