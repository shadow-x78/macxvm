# The framebuffer contract

## Sources

- The public Apple open source distribution of XNU.
- The load probes of the guests and the traces that MacXVM records.

## The facts that the driver may rely on

1. The framebuffer becomes the display of index zero only after the readiness
   of the driver; before that the boot holds the display initialization.
2. The modes come from the measurements of the board and from the EDID of the
   connected display; a mode is never invented.
3. The hot plug of a display triggers a re-read of the EDID and a new set of
   modes, and the content of the surface survives the change.

## Open questions

- The behaviour of the variable refresh rate of the board is measured by the
  matrix of the `T-H3` task.
