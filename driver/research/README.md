# MXGPU research notes

This directory holds the behavioural notes of MXGPU. Every note describes what
a component must do; none of them contains source code of another project.

## The clean room rule

1. A note is written from a public specification, from a public header that is
   allowed to be read, from a trace of a guest that MacXVM produced itself, or
   from an experiment on the hardware of the operator.
2. The source of every statement is named in the note itself.
3. No file of Apple, NVIDIA, Mesa or NullMoth is copied into this repository.
   NullMoth is a behavioural reference for the general shape of the problem and
   is never a source.
4. The third party components that were consulted are listed in
   `driver/third_party.lock.json` with their license, their role and the state
   of their provenance.
5. The identifiers of a commercial board are never impersonated. The identity
   of the GPU comes from the measurements of the board.

## The notes

| File | Subject | Source of the behaviour |
|---|---|---|
| `pci-gb206.md` | the PCI layout, the BARs, the reset and the GSP of the GB206 board | the public NVIDIA open kernel modules, the public VFIO documentation, the measurements of the board |
| `xnu-ioaccelerator.md` | the IOKit personalities, the AGDC policy and the IOSurface contract | the public Apple open source distribution of XNU, the load probes of the guests |
| `xnu-ioframebuffer.md` | the framebuffer, the modes and the display hot plug | the public Apple open source distribution of XNU, the load probes of the guests |
| `metal-driver-abi.md` | the Metal objects, the queues and the arguments | the public Metal documentation of Apple, the mock backend of the tests |
| `videotoolbox-abi.md` | the decode and the encode sessions | the public VideoToolbox documentation of Apple, the minimal probes of the guests |

## The probes of the guests

The load probes write their results into `driver/tests/load-probes`. Every
result names the build of macOS, the SDK, the architecture, the class
selectors, the structure layouts, the signing state, the state of SIP, AMFI
and SecureBoot, and the log of the load. A note without such a result stays a
hypothesis and never becomes a claim.
