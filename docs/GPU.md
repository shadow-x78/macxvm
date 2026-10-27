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

Nothing else. There is no `virtio-gpu` kext in macOS: the only VirtIO families
shipped are `AppleVirtIO`, `AppleVirtIOStorage`, `AppleVirtIOBiometrics` and
`AppleVirtIONeuralEngineDevice`. `AppleParavirtIOSurface.kext` exists but belongs
to Apple's own Virtualization.framework on a macOS host, so it is not usable
from Linux QEMU.

That is why the default is `-vga none` plus `bochs-display`: it is the only
virtual device for which macOS has a real frame buffer driver. The earlier
`vmware-svga` device produced `No Kext Loaded`, meaning no driver at all.

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
   line and `vfio-pci` bound to the card.
2. **No macOS driver.** Even with IOMMU working, macOS has no driver for the
   RTX 5060 (Blackwell). NVIDIA's last macOS drivers stop at Kepler. A passed
   through card would appear as an unknown PCI device with no acceleration, and
   it would remove the only working display device.

Passthrough is therefore not a performance path on this hardware, and MacXVM
does not pretend otherwise.

## What actually makes the guest faster

Measured on the reference host with the guest running the same workload:

| Change | Effect |
|---|---|
| `+avx2,+fma,+bmi1,+bmi2,+sha-ni` in the CPU model | the guest CPUID now advertises AVX2, FMA, BMI1/2 and SHA |
| 4 to 8 vCPU | a parallel workload dropped from 18.4 s to 6.3 s, about 2.9x |
| 8 to 12 GiB RAM | no guest swap at idle; `vm_stat` shows zero swapins |
| `slim --level 1` | reduces motion, transparency, window animations, Dock animation |
| 1920x1080 frame buffer | the default; a higher resolution means more pixels for the software renderer to blit |
| `-cpu host` | **fails to boot** on this host; do not use it |

`macxvm optimize NAME --width 1440 --height 900` trades sharpness for fewer
pixels if the guest still feels slow. The setting only applies to a stopped VM.

## Commands

```bash
macxvm gpu macos                     # host IOMMU/GPU plus the real guest kext
macxvm optimize macos --check        # is the Bochs path already in place?
macxvm optimize macos                # stopped VM, 1920x1080
macxvm optimize macos --width 1440 --height 900
```

## Honest summary

- macOS does recognise the display: `AppleBochVGAFB` is loaded and reported.
- It is a 2D frame buffer with no Metal, by driver design.
- NVIDIA passthrough is blocked by missing IOMMU and a missing macOS driver.
- The real wins are vCPU count, RAM, CPU instruction flags and reduced UI work.
