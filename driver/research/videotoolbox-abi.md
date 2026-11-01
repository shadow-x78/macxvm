# The VideoToolbox contract

## Sources

- The public VideoToolbox documentation of Apple.
- The minimal decode and encode probes of the `T-D2c` task.

## The facts that the driver may rely on

1. The decode and the encode sessions are created by a real client process and
   they register their service with the framework of the guest.
2. The frames that leave the decoder carry the format that the client asked
   for; the bitstream of the encoder is checked against the source frames.
3. The service of the encoder runs in its own process on the supported
   releases; the driver never assumes that it runs inside the client.

## Open questions

- The behaviour of the encoder of Tahoe is measured by the probes before the
  feature is announced.
