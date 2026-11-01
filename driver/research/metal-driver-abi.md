# The Metal contract of the plug-in

## Sources

- The public Metal documentation of Apple.
- The mock backend of the tests of the `T-M1a` task.

## The facts that the driver may rely on

1. The plug-in is discovered through the Metal framework of the guest and not
   through a private path.
2. Every object of the plug-in owns its resources and releases them when the
   last reference disappears; a client that dies does not leave the hardware
   in a busy state.
3. The queues of a client are ordered; the fences of a client order the work
   of the other clients only through the shared memory of the client.
4. The shader of a client is translated by the AIR verifier before it reaches
   the hardware; a shader that the verifier refuses is reported as an error
   and never executed.

## Open questions

- The tier two features of the argument buffers are enabled only after the
  measurements of the `T-M2` task.
