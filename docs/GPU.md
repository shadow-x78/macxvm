# MacXVM - Display and GPU

This page records what the guest actually reports, the reverse engineering behind
the default display, and the limits that cannot be engineered around.

## What macOS sees today

Run `macxvm gpu NAME` while the guest is running. On this project's reference
host the guest reports:

| Field | Value |
|---|---|
| Vendor ID | `0x1234` |
| Device ID | `0x1111` |
| Class | `AppleBochVGAFB` |
| Kext | `AppleBochVGAFB` inside `AppleVirtualGraphics.kext` |
| Bus | PCIe, x1 |
| Framebuffer | `IOFBMemorySize = 8294400` bytes |
| Framebuffer / 4 bytes per pixel | 2,073,600 pixels = exactly 1920x1080 |

`8294400 = 1920 x 1080 x 4`, so macOS is not reporting a crippled 7 MB card; it
is reporting one full 1080p frame buffer. The `VRAM (Total): 7 MB` line in
System Information is that single frame, not the 256 MB QEMU aperture.

## Reverse engineering of the guest drivers

The kext that drives a virtual display on macOS is
`/System/Library/Extensions/AppleVirtualGraphics.kext`. Its personality list
matches only two PCI pairs:

| Vendor | Device | Driver class |
|---|---|---|
| `0x1234` | `0x1111` | `AppleBochVGAFB` |
| `0x1013` | `0x0000` | `AppleCirrusGD5446FB` |

Nothing else. There is no `virtio-gpu` driver in macOS: the virtio GPU type
`0x10` is not among the ten device types that `AppleVirtIO.kext` claims.
`AppleParavirtGPU.kext` is a different story: it binds the PCI identity
`0x106b:0xeeee` straight to `IOPCIDevice` and provides an `IOAccelerator`
with the Metal plugin `AppleParavirtGPUMetal`. No QEMU device has that
identity, so the guest cannot use it yet.

That is why the default is `-vga none` plus `bochs-display`: it is the only
virtual device for which macOS has a real frame buffer driver. The earlier
`vmware-svga` device produced `No Kext Loaded`, meaning no driver at all.

The full map of the guest kexts, with the native accelerators that macOS can
drive and the measured effect of using them, is in
[ACCELERATION.md](ACCELERATION.md). Run `macxvm accel NAME --re` to read the
map of your own guest.

## Why there is no Metal or 3D

`AppleBochVGAFB` is a 2D frame buffer driver. It exposes no `IOAccelerator`
service for a GPU, so Metal, Core Animation GPU compositing and the iOS Simulator
all fall back to software rendering on the CPU. This is a driver capability
limit, not a configuration mistake, and no QEMU flag changes it.

## Why NVIDIA passthrough is not a solution here

`macxvm gpu NAME` reports the host facts. On the reference host:

```text
0000:01:00.0 VGA compatible controller: NVIDIA Corporation GB206 [GeForce RTX 5060]
IOMMU groups: 0
```

Two independent blockers:

1. **No IOMMU.** There are zero IOMMU groups and no `intel_iommu=on` or
   `iommu=pt` on the kernel command line, and the CPU has no integrated GPU
   (the `F` suffix means graphics are disabled). Passing a device through needs
   VT-d enabled in firmware, `intel_iommu=on iommu=pt` on the kernel command
   line and `vfio-pci` bound to the card. MacXVM can do the last two steps:
   `macxvm gpu NAME --enable-iommu` and `macxvm gpu NAME --bind-vfio ADDRESS`.
2. **No macOS driver.** Even with IOMMU working, macOS has no driver for any
   NVIDIA GeForce card. Apple ended the support in 2019 and the last drivers
   stop at Kepler. A passed through card would appear as an unknown PCI
   device with no acceleration, and it would remove the only working display
   device of the host.

The host can be made passthrough ready, and MacXVM does that on request, but
a passed through NVIDIA card never becomes a performance path for macOS. The
only acceleration target that macOS itself claims is Apple's paravirtual GPU
at `0x106b:0xeeee`, which needs a new QEMU device.

## What actually makes the guest faster

Measured on the reference host with the guest running the same workload:

| Change | Effect |
|---|---|
| `+avx2,+fma,+bmi1,+bmi2,+sha-ni` in the CPU model | the guest CPUID now advertises AVX2, FMA, BMI1/2 and SHA |
| 4 to 8 vCPU | a parallel workload dropped from 18.4 s to 6.3 s, about 2.9x |
| 8 to 12 GiB RAM | no guest swap at idle; `vm_stat` shows zero swapins |
| `slim --level 1` | reduces motion, transparency, window animations, Dock animation |
| 1920x1080 to 1440x900 | `WindowServer` idle dropped from 7.5% to 2.5% CPU |
| AHCI to NVMe for the system disk | write 694 to 984 MiB/s, read 6.6 to 10.1 GiB/s, TRIM on |
| `-cpu host` | **fails to boot** on this host; do not use it |

The default is now 1440x900 because the software renderer blits every pixel on
the CPU. Measured on the reference host, `WindowServer` at idle fell from 7.5% to
2.5% CPU. Use `macxvm optimize NAME --width 1920 --height 1080` if you prefer the
sharper mode; the setting only applies to a stopped VM.

`macxvm accel NAME --apply` moves the system disk to NVMe and exposes the
entropy and the balloon, because macOS ships drivers for all three. The
measurements and the map behind the decision are in
[ACCELERATION.md](ACCELERATION.md).

## Commands

```bash
macxvm gpu macos                     # host IOMMU/GPU plus the real guest kext
macxvm gpu macos --passthrough       # what the host still needs for a passthrough
macxvm gpu macos --enable-iommu      # turn VT-d on in the kernel command line
macxvm optimize macos --check        # is the Bochs path already in place?
macxvm optimize macos                # stopped VM, 1440x900
macxvm optimize macos --width 1920 --height 1080
macxvm accel macos --re              # the reverse engineering map of the guest
macxvm accel macos --apply           # stopped VM: native accelerators
```

## Honest summary

- macOS does recognise the display: `AppleBochVGAFB` is loaded and reported.
- It is a 2D frame buffer with no Metal, by driver design.
- NVIDIA passthrough is blocked by a missing macOS driver, and the missing
  IOMMU of the host is fixable with one command and a reboot.
- The real wins are vCPU count, RAM, CPU instruction flags, NVMe and reduced
  UI work.
