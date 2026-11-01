# MacXVM - The MXGPU driver package

MXGPU is the driver that MacXVM plans for a passed through NVIDIA board on
macOS. This page describes the parts that exist in the repository today: the
package format, the trust roots, the build on a macOS builder, the
installation transaction and the state of the work.

**The feature is experimental.** The kernel extensions, the Metal plug-in, the
AIR translator and the video components are not written yet. Nothing in this
page claims a working Metal path, and the gates of the plan are still open.
The state of every task is recorded in
[artifacts/mxgpu/execution-ledger.json](../artifacts/mxgpu/execution-ledger.json).

## The pieces of the driver

| Component | Responsibility |
|---|---|
| `MXRM.kext` | the PCI function, the BARs, the DMA, the interrupts, the GSP, the memory, the channels, the fences, the power and the reset |
| `MXKMS` | the modes, the heads, the vblank and the suspend of the modeset, inside `MXRM.kext` |
| `MXFB.kext` | the frame buffer, the EDID, the high resolution, the HDR, the hot plug and the cursor |
| `MXAccel.kext` | the `IOAccelerator`, the `IOGPU`, the IOSurface, the residency and the completion |
| `MXAGDC.kext` | the policy of the device control, without faking the identity of a commercial card |
| `MXMTLDriver.bundle` | the Metal objects, the queues, the encoders, the resources and the fences |
| `MXAIR` | the verifier of the AIR shaders and the lowering into SPIR-V |
| `NVK` on XNU | the Vulkan and the NAK on top of `MXRM` |
| `MXVideo` | the decode and the encode of the guest |

The sources live under `driver/`. The notes that describe the behaviour of
each component live in `driver/research`, and the components that were
consulted are listed in `driver/third_party.lock.json`.

## The clean room

- No Apple source, no NVIDIA firmware and no code of another driver is copied
  into this repository.
- The notes name the source of every statement: a public specification, a
  public header that may be read, a trace of a guest that MacXVM produced, or
  an experiment on the hardware of the operator.
- The components of the lock file are references until their exact revision
  and their digest are written in it. The gate of the clean room stays open
  until then.
- The identifiers of a commercial board are never impersonated. The identity
  of the GPU comes from the measurements of the board.

## The package format

A package is a gzip archive that holds:

| File | Meaning |
|---|---|
| `manifest.json` | the format, the key identity, the target, and every file with its size and its SHA256 |
| `manifest.sig` | the Ed25519 signature of the canonical form of the manifest |
| `SHA256SUMS` | the hashes of the files of the package |
| `SBOM.json` | the third party components with their license and their provenance |
| `NOTICE` | the notice of the redistribution |
| `third_party.lock.json` | the lock file of the components that were consulted |

The signature covers the canonical form of the manifest, not the archive, so
the archive can be rebuilt without a new key. The canonical form sorts the
files and the keys of the manifest and rejects a duplicate key, a floating
point number and trailing data.

The verifier refuses a package that holds a path that leaves the archive, an
absolute path, a link, a special file, a duplicate path, too many files, a
file that is too large or an archive that expands too much. It refuses a file
whose size or whose hash differs from the manifest, and a manifest whose
signature does not match the release key.

## The trust roots

| Key | Where | Use |
|---|---|---|
| the release key | `resources/mxgpu-release-public-key.txt` | the signature of the packages |
| the host backup key | `~/.macxvm/keys/backup-key` | the signature of the manifest of the backup of the host |
| the witness key of the migration | the operator | the record of the ownership during a migration |

The private keys live outside the repository, root only, and they never enter
a package. The public key of the release is the one that the verifier of
MacXVM uses.

## Building on a macOS builder

The kexts are built on macOS; they are never built on Linux.

```bash
macxvm driver build --builder user@macos-builder
```

The command archives the `driver/` directory with its lock file, sends it over
SSH to a temporary directory, checks that the archive arrived unchanged, runs
`make build`, `make test` and `make package` on the builder, fetches the
package into `artifacts/mxgpu/package/` and verifies its signature.

On the builder itself:

```bash
make -C driver build      # the kexts, for ARCHS=x86_64 and SDKROOT=macosx
make -C driver test       # the tests of the builder
make -C driver test-host  # the checks that need no macOS framework
make -C driver package    # the package and its SBOM
```

`make -C driver test-host` runs on Linux and in the CI: it checks the lock
file of the third party components and the credit banner of every source of
the driver.

## Installing in a guest

```bash
macxvm gpu NAME --install-driver PACKAGE --accept-driver-changes
```

The transaction does the following:

1. It verifies the signature of the package and the hash of every file.
2. It backs up the NVRAM, the installed manifest, the three stage files and
   the OpenCore images of the machine.
3. It writes a signed manifest of the backup, so that the rollback restores
   exactly the state that the installation found.
4. It copies the components into the guest and writes the installed manifest.
5. It records a probe report; a machine is only `gpu-ready` when that report
   is successful.

The rollback restores the backup that matches the manifest:

```bash
macxvm gpu NAME --remove-driver
macxvm start NAME --display recovery
```

The recovery stage boots with the boot argument `-mxoff`, so the driver does
not touch the board, and the display of the guest falls back to `bochs-display`
over VNC.

## The state of the work

| Gate | State |
|---|---|
| G0, the clean room and the SBOM | open: the digests of the provisioned artifacts are missing |
| G1, the ownership of the board | open: the cycles on the physical board are missing |
| G2 to G7, the driver and the features | open: the sources are not written |
| G9, the package and the installation | open: no package was signed and installed in a guest |

The ledger of the plan lists the state of every task with its evidence.
