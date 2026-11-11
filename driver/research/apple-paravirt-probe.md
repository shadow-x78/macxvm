# The Apple paravirtual graphics probe

## What this note covers

Whether macOS binds `AppleParavirtGPU.kext` to a PCI function that announces
the identity `0x106b:0xeeee`, and what the guest does with it.

## Sources

- The public Apple open source distribution of XNU, for the shape of the IOKit
  personalities.
- The load probes of the guests that MacXVM produces. The results live in
  `driver/tests/load-probes`.
- The measurements of the host, in `artifacts/mxgpu/d0` and
  `artifacts/mxgpu/d3`.

## The fact that the measurement settles

The identity alone is not enough. The guest enumerated the function and left
it alone.

The probe device presents the identity `0x106b:0xeeee`, the class
`0x0380` and one BAR of 4096 bytes. The guest read the PCI configuration
space of the function, which the QMP query of the bus confirms:

```text
slot 4 | vendor 0x106b | device 0xeeee | class 0x0380 (display controller)
       | BAR0 memory, 4096 bytes
```

macOS Recovery, the environment that installs the system, made **no** access
to that BAR. The log of the guest holds the identity line of the device and
nothing else:

```text
apple-gfx-probe: the identity 106b:eeee is presented to the guest, with a 1920 x 1080 display mode
```

So the driver did not bind. The kext of Apple needs more than the identity:
it needs the option ROM and the register contract of the device.

## What this tells the next task

The device of Apple in the QEMU of upstream answers the MMIO of the guest with
the `ParavirtualizedGraphics` framework of the host, which is a closed
library that only exists on macOS. The upstream device therefore cannot be
built on Linux, and it cannot be read as a specification of the protocol,
because the protocol lives in that library.

The register contract has to come from the traces of a guest that already
binds the driver. Such a guest needs a macOS host, because the framework that
answers the registers is the piece that is missing on Linux.

The measurements of the identity are therefore the first step, and they are
not enough. The second step needs the traces of a macOS host.

## The open questions

- Whether the option ROM of the framework is a part of the binding, or only of
  the early boot of the display.
- Which registers the driver writes before it publishes the accelerator.
- Whether the driver refuses the function because of the missing ROM, or
  because the BAR answers zero.

## The probe device

The device that carries the measurement is in
`driver/qemu/apple-gfx-probe.c`. It writes the identity, it answers every
register with zero, and it logs every access. It renders nothing.
