# The GB206 board

## What this note covers

The PCI layout of the board, the size of its BARs, its reset behaviour and the
pair of the resource manager and the GSP firmware.

## Sources

- The public documentation of the VFIO interface of the Linux kernel:
  `https://docs.kernel.org/driver-api/vfio.html`.
- The public description of the behaviour of the resource manager in the
  open kernel modules of NVIDIA. Only the description is read; no code and no
  firmware are copied.
- The measurements of the board that MacXVM itself produces with the probes of
  the `T-D1` task. The raw results live in `artifacts/mxgpu/d1`.

## The facts that the driver may rely on

1. The board exposes one display function and one audio function.
2. The audio function shares the identity of the card and lives in its own
   IOMMU group; a group that holds a device that the host still needs is
   refused by the host tooling.
3. The BARs are measured, never assumed. The aperture of the guest comes from
   the measurement of the board and from the enabled state of ReBAR; the
   values of a demonstration are not used in a shipped configuration.
4. The reset of the board is only performed while no client holds the device.
   A guest that dies leaves the board to the host after the rebind.

## Open questions

- The exact relation between the reset method of the board and the firmware
  that must be loaded first is not settled yet; the probe of the `T-D1` task
  measures it before the driver relies on it.
- The version of the resource manager and the version of the GSP firmware that
  belong together are pinned by the provenance bundle of the `T-D1a` task.
