# MacXVM - Native Acceleration

This page records the reverse engineering of the guest drivers, the native
accelerators that macOS can drive, the measured effect of switching a machine
to them, and the acceleration targets that are still open.

## The guest kexts are the contract

macOS can only drive a virtual device when the guest ships a driver for it.
The drivers are kernel extensions, and every extension publishes its matching
rules in `Contents/Info.plist` under `IOKitPersonalities`. Those rules name
the PCI identities, the PCI class codes and the virtio device types that the
driver claims.

MacXVM reads that file from the running guest:

```bash
macxvm accel macos --re          # the map of the running guest
macxvm accel macos               # the map against the machine
macxvm accel macos --apply       # switch the machine to the native path
```

The command reads `/System/Library/Extensions/<kext>.kext/Contents/Info.plist`
in the guest, converts it to XML with `plutil`, and parses the personalities.
The evidence below comes from macOS Sonoma 14.8 (build 23J619) on the
reference host.

## The complete map

### PCI identities that macOS claims

| PCI identity | Driver | Kext | What it provides |
|---|---|---|---|
| `0x1234:0x1111` | `AppleBochVGAFB` | `AppleVirtualGraphics` | 2D frame buffer |
| `0x1013:0x0000` | `AppleCirrusGD5446FB` | `AppleVirtualGraphics` | 2D frame buffer |
| `0x106b:0xeeee` | `AppleParavirtGPUControl` | `AppleParavirtGPU` | **Metal GPU**, `IOAccelerator` and `IOFramebuffer` |
| `0x106b:0x1a00` | `AppleVirtIOStorageDevice` | `AppleVirtIOStorage` | Apple block storage |
| `0x106b:0x1a03` | `AppleVideoToolboxParavirtualizationDriver` | `AppleVideoToolboxParavirtualization` | video decode and encode |
| `0x106b:0x2002` | `AppleEmbeddedNVMeController` | `IONVMeFamily` | Apple NVMe |

### PCI class codes that macOS claims

| Class code | Driver | Kext | What it provides |
|---|---|---|---|
| `0x01080200` | `IONVMeController` | `IONVMeFamily` | **NVMe storage**, any vendor |

The `GenericNVMeSSD` personality of `IONVMeFamily` matches the class code
`0x01080200&0xffffff00`, so macOS claims every standard NVMe controller,
including the one of QEMU.

### virtio device types that macOS claims

The transport is the standard virtio one: the personality
`AppleVirtIOPCITransport` matches the PCI vendor `0x1af4`, and the drivers
below match the virtio device type.

| Virtio type | Driver | What it provides |
|---|---|---|
| `0x01` | `AppleVirtIONetwork` | ethernet |
| `0x02` | `AppleVirtIOBlockStorageDevice` | block storage |
| `0x03` | `AppleVirtIOConsole` | serial console |
| `0x04` | `AppleVirtIOEntropy` | entropy |
| `0x05` | `AppleVirtIOBalloon` | memory balloon |
| `0x09` | `AppleVirtIO9P` | 9p share |
| `0x12` (18) | `AppleVirtIOInput` | keyboard, mouse, trackpad |
| `0x13` (19) | `AppleVirtIOSocket` | vsock socket |
| `0x19` (25) | `AppleVirtIOSound` | audio |
| `0x1a` (26) | `AppleVirtIOFS` | virtio-fs share |

The virtio GPU type `0x10` (16) is **not** in the list. macOS ships no driver
for a virtio GPU, so `virtio-gpu` in QEMU is invisible to the guest. The
display therefore stays on `bochs-display`, which is the frame buffer that
`AppleBochVGAFB` claims.

## What the map changed

The project already used `virtio-net` and knew that `vmware-svga` has no
driver. The map adds the facts that were missing:

1. **NVMe is a native path.** `IONVMeFamily` claims every NVMe controller, so
   the system disk does not need the AHCI stack of macOS.
2. **The entropy and the balloon have drivers.** `AppleVirtIOEntropy` and
   `AppleVirtIOBalloon` are available, so both devices can be exposed.
3. **Apple ships a Metal capable paravirtual GPU.** The identity is
   `0x106b:0xeeee` and the driver is `AppleParavirtGPUControl`, with the Metal
   plugin `AppleParavirtGPUMetal`. QEMU has no device with that identity, so
   the guest cannot use it yet. It is not a virtio device: the personality
   binds it straight to `IOPCIDevice`, and the companion kexts match the
   device names `paravirtualizedgraphics,gpu` and
   `paravirtualizedgraphics,iosurface`.
4. **macOS ships Intel display drivers.** `AppleIntelKBLGraphicsFramebuffer`
   claims the Intel identities `0x8086:0x5912`, `0x5916`, `0x591b`, `0x591c`,
   `0x591e`, `0x5923`, `0x5926`, `0x5927` and `0x87c0`. They need the real
   Intel hardware behind them, so they are not a path for a host with an
   NVIDIA card.

## The measured effect

Measured on the reference host with the same guest and the same workload.
The numbers are the output of `dd` inside the guest.

| Path | Write, 512 MiB with `fsync` | Read, 512 MiB |
|---|---|---|
| AHCI (`ide-hd`, before) | 727,538,340 B/s = 694 MiB/s | 6,963,396,568 B/s = 6.6 GiB/s |
| NVMe (`nvme`, after) | 1,032,100,690 B/s = **984 MiB/s** | 10,796,149,292 B/s = **10.1 GiB/s** |

That is 1.42x on the write and 1.55x on the read. The guest confirms the
driver and the feature:

```text
$ system_profiler SPNVMeDataType
NVMExpress:
    Generic SSD Controller:
        QEMU NVMe Ctrl:
          TRIM Support: Yes
          Serial Number: macxvm
          BSD Name: disk0
```

`kextstat` shows `com.apple.iokit.IONVMeFamily (2.1.0)` loaded, and the disk
is `disk0` with its GPT partition map intact. The volume is the same volume:
the APFS container and the EFI partition moved with the disk.

`macxvm accel macos --apply` performs exactly this change:

- the system disk moves from `-device ide-hd` to `-device nvme,drive=...`;
- `-device virtio-rng-pci` is added for `AppleVirtIOEntropy`;
- `-device virtio-balloon-pci` is added for `AppleVirtIOBalloon`;
- the boot loader disk stays on AHCI, so the boot path of the firmware does
  not change;
- `args.txt` is copied to `args.txt.bak-accel-<time>` before the write.

## The graphics card on this hardware

The host has one display device, an NVIDIA GeForce RTX 5060, and the guest is
macOS. macOS has no driver for any NVIDIA GeForce card: Apple ended the
support in 2019 and the last drivers stop at Kepler. A passed through card
would therefore appear in the guest as an unknown PCI device with no
acceleration, and it would remove the only working display of the host.

MacXVM still makes the host ready for a passthrough, because that is the part
that the host controls:

```bash
macxvm gpu macos --passthrough        # what the host still needs
macxvm gpu macos --enable-iommu       # turn VT-d on in the kernel command line
macxvm gpu macos --bind-vfio 0000:01:00.0   # hand the card to vfio-pci
```

`--enable-iommu` adds `intel_iommu=on iommu=pt` to every kernel entry with
`grubby` and asks for a reboot. `--bind-vfio` unloads the NVIDIA modules,
unbinds the card and binds it to `vfio-pci`. The command refuses to bind the
only display device of the host, because that would remove the desktop of the
host itself.

The guest side of a passthrough is a separate question, and the answer is in
the kexts: macOS claims `0x106b:0xeeee` for its own paravirtual GPU and
nothing else that can accelerate. The open target is therefore to implement
that device in QEMU, with the protocol that `AppleParavirtGPU` speaks. It is
not virtio-gpu, so no existing QEMU device can be reused for it.

## Commands

```bash
macxvm accel macos --re               # the reverse engineering map of the guest
macxvm accel macos                    # the map against the machine
macxvm accel macos --check            # the report, with a failure when a gain is missing
macxvm accel macos --apply            # stopped VM: switch to the native accelerators
macxvm accel macos --json             # the same report as JSON
macxvm gpu macos --passthrough        # the readiness of the host for a passthrough
```

## Honest summary

- The kexts of the guest are the source of truth, and MacXVM reads them.
- macOS drives NVMe natively, and the system disk now uses it: 1.42x on the
  write, 1.55x on the read, with TRIM.
- macOS drives the standard virtio transport for ten device types, and the
  entropy and the balloon are exposed now.
- macOS has no virtio-gpu driver, so the display stays on the 2D frame buffer
  of Bochs.
- The only Metal capable device that macOS claims is Apple's own
  `0x106b:0xeeee`, which no QEMU device implements yet.
- The NVIDIA card of this host cannot accelerate macOS, whatever the host
  does.
