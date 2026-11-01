# The IOKit contract of the accelerator

## Sources

- The public Apple open source distribution of XNU. Only the interface of the
  classes is described here; no header is copied into this repository.
- The load probes that MacXVM builds for each supported build of macOS. The
  results live in `driver/tests/load-probes`.

## The facts that the driver may rely on

1. The accelerator publishes a personality per class and per property, and the
   properties of a personality are matched in a fixed order.
2. The policy of the device control decides which buffers of the guest are
   allowed to reach the hardware. The policy is read from the guest, never
   guessed from a commercial card.
3. The surface of an IOSurface belongs to the process that created it; a
   second process sees the same buffer only when the owner shares it.
4. The residency of a buffer and the cache policy of the hardware are
   described by the accelerator, not by the kernel extension.

## Open questions

- The exact behaviour of the policy on Tahoe is measured by the matrix of the
  `T-C2a` task before the final configuration is written.
