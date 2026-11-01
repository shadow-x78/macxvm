# The tests of the driver

Every directory holds one field of the matrix of the plan:

| Directory | Field |
|---|---|
| `abi` | the sizes, the versions, the invalid pointers and the death of a client |
| `load-probes` | the load of the minimal components on every supported build of macOS |
| `metal` | the buffers, the textures, the heaps, the blits, the fences and the events |
| `golden` | the images that the display and the Metal paths must produce |
| `stress` | the long mixed workload |
| `power` | the sleep and the wake with their hashes |
| `migration` | the checkpoint, the cancel and the rollback |

Every result is written as a JSON file that names the build of macOS, the
SDK, the architecture, the driver version and the hash of the artifacts that
were used. A test without such a file is not a result.

The tests that need a guest run inside the probe machines of the `T-A0` task;
the tests that need no guest run on the builder with `make -C driver test`.
