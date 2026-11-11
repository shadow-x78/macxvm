# The Apple paravirtual GPU on the RTX 5060 — plan of the first gate

## Goal

Make a macOS guest under QEMU load the **Apple paravirtual GPU driver** and
report an `IOAccelerator`, on the reference host that owns one NVIDIA
RTX 5060 (GB206, `10de:2d05`) and no other display device.

This is **gate G1c of the existing plan** (`.kilo/plans/1793364599068-reims-gpu-native-plan.md`),
scoped to what can be measured before any Metal work. It does not claim
rendering, performance, or Metal support.

## What the repository already contains

The plan is not greenfield. These pieces exist and are the foundation:

| Piece | Where | State |
|---|---|---|
| `reims` backend with `DriverAppleParavirt` | `internal/config/gpu.go` | implemented, validated |
| `ReimsRuntime`, `ReimsQEMUBinary`, `ReimsResolution`, `ReimsROM` | `internal/config/gpu.go`, `internal/argsfile/argsfile.go` | implemented, files validated |
| QEMU binary override for the backend | `internal/qemu/qemu.go` `BinaryFor` | implemented |
| Args templates that emit `apple-gfx-pci` | `resources/args-template-reims-{install,daily}.txt` | implemented, gated by `#when REIMS_ROM` |
| Kext map proving the identity | `internal/kextmap/kextmap.go` | `AppleParavirtGPU` → `0x106b:0xeeee` |
| Research notes | `driver/research/*.md` | written; **no measurements yet** |
| Load probes | `driver/tests/load-probes/` | **empty** |

The templates already emit:

```
-device apple-gfx-pci,romfile={{REIMS_ROM}},resolution={{REIMS_RESOLUTION}}
```

`apple-gfx-pci` is **not** a device of upstream QEMU. It exists only in a
custom QEMU that has to be built and named by `ReimsRuntime`. That build is the
first task.

## Constraints that shaped this plan

1. **Legal.** macOS in a VM on non-Apple hardware breaks Apple's licence. The
   plan records measurements; it does not distribute Apple software. The user
   owns this decision.
2. **Two independent blockers on this host.** No IOMMU groups and no
   `vfio-pci` (so the `vfio` backend cannot work), and macOS has no driver for
   GB206. The paravirtual identity avoids both: the guest never sees NVIDIA.
3. **No macOS hosted runner.** The CI job that asked for `macos-14` is
   cancelled in two seconds on this account (decision A107). Anything that
   needs macOS runs on the operator's machine, not in CI.
4. **Measurement, not claim.** `driver/research/README.md` states that a note
   without a probe result stays a hypothesis. The deliverable of this gate is
   a probe result, not prose.
5. **Clean room.** No Apple, NVIDIA, Mesa or NullMoth source enters the
   repository (`driver/third_party.lock.json`). The QEMU device is written from
   the public QEMU device API and from traces the operator records.

## The one open technical question

`AppleParavirtGPU.kext` binds the PCI identity `0x106b:0xeeee` and publishes an
`IOAccelerator` with the Metal plugin `AppleParavirtGPUMetal`. What is **not**
known from any public source is the **BAR contract**: which registers the guest
reads and writes during `start`, and what the driver expects back.

Everything after the first load depends on that contract. The plan therefore
splits at exactly that point.

## Tasks

Each task is atomic and has a check that fails loudly. Run them in order.

### D0 — Record the host baseline

1. Run `macxvm doctor` and `macxvm gpu --passthrough` and save both outputs to
   `artifacts/mxgpu/d0/`. They are the "before" evidence.
2. Record `lspci -nn`, `/proc/cmdline`, the QEMU version and the Go version in
   the same directory.

**Verify:** the four files exist and name this host.
**Rollback:** none, the task only writes under `artifacts/`.

### D1 — Build a QEMU that knows `apple-gfx-pci`

1. Take the QEMU source of the same version the host runs (10.2.x) and add a
   device `apple-gfx-pci` under `hw/display/`.
2. The device declares, and nothing more:
   - PCI vendor `0x106b`, device `0xeeee`, class `0x030000`.
   - An optional `romfile` property (the option ROM the guest expects) and a
     `resolution` property, matching the two placeholders the templates already
     pass.
   - Two BARs: BAR0 a 4 KiB register window (`MemoryRegionOps` with read/write
     handlers that log and return zero), BAR2 a shared memory window.
3. Register the device in `hw/display/meson.build` and in the
   `qemu-system-x86_64` build.
4. Install it as `<runtime>/bin/qemu-system-x86_64` and expose
   `<runtime>/share/reims.rom` through `ReimsROMPath`.

**Verify:**
```bash
"$RUNTIME/bin/qemu-system-x86_64" -device help | grep -q apple-gfx-pci
"$RUNTIME/bin/qemu-system-x86_64" -device apple-gfx-pci,help
```
Both commands must succeed and the second must list `romfile` and `resolution`.
**Rollback:** remove the runtime directory; the host QEMU is untouched.

**Risk:** if the guest requires a valid option ROM to bind at all, the device
will not bind with an empty ROM. Task D3 answers that with a measurement
instead of a guess.

### D2 — Prepare a guest that can be measured

1. On the host: `macxvm image prepare probe --profile cli --macos sonoma --cpus 8 --ram 6G`.
   The RAM figure is chosen because the host had about 6.5 GiB available when
   this plan was written; raise it only when the host has free memory.
2. `macxvm autoinstall probe --yes --accept-license`, then complete Setup
   Assistant in the viewer (`macxvm open probe`), then
   `macxvm finish probe --apps`.
3. Do **not** seal the machine; the probe machine must stay mutable.

**Verify:** `macxvm exec probe -- sw_vers` prints the version and
`macxvm exec probe -- system_profiler SPDisplaysDataType` shows the Bochs
display. This is the control state: a guest that boots and reports no
accelerator.

**Rollback:** `macxvm delete probe --yes`.

### D3 — Register the paravirtual identity and probe it

This is the gate. It answers the open question.

1. Create the machine with the backend the code already supports:

```bash
macxvm create rt-probe --args-file <the args file of the probe machine> \
  --assets <the assets of the probe machine> \
  --gpu-backend reims --reims-runtime "$RUNTIME" --reims-resolution 1920x1080
```

2. Start it and capture, inside the guest:

```bash
macxvm exec rt-probe -- sh -c 'ioreg -l -w0 > /tmp/ioreg.txt; kextstat | grep -i paravirt'
macxvm exec rt-probe -- sh -c 'system_profiler SPDisplaysDataType'
macxvm exec rt-probe -- sh -c 'log show --last 5m --predicate "senderImagePath CONTAINS AppleParavirt"'
```

3. Record the device trace on the host: run the custom QEMU with
   `-trace apple_gfx_*` (or the logging the D1 handlers already do) and keep
   the BAR access log in `artifacts/mxgpu/d3/`.

**Decision rule, written before the measurement:**

| Observation | Conclusion | Next task |
|---|---|---|
| `AppleParavirtGPU` appears in `kextstat` **and** an `IOAccelerator` appears in `ioreg` | the identity is enough to bind | D4 |
| The kext loads but no `IOAccelerator` is published | the BAR contract is required | D5 |
| The kext does not load at all | the ROM or a personality property is missing | D6 |
| The guest panics or hangs on boot | the identity alone breaks the boot path | stop, restore the `bochs-display` args file, report |

**Rollback:** `macxvm delete rt-probe --yes`; the probe machine is untouched.

**This is the point where the plan branches.** Nothing after it should be
started before D3 produces one of the four rows above.

### D4 — Only if the driver binds: publish the accelerator

Fill the BAR0 read handlers with the values the trace shows the guest expects,
then re-run D3 until `IOAccelerator` appears. Record every value in
`driver/research/xnu-ioaccelerator.md` with the trace line that justifies it.

### D5 — Only if the BAR contract is required: map it

1. Replay the recorded BAR accesses against the device, one register at a time.
2. For each register, record: offset, width, value written, value read back,
   and what the guest did next.
3. Write the map into `driver/research/xnu-ioaccelerator.md` and
   `driver/research/xnu-ioframebuffer.md` as facts with their trace lines.

This task produces knowledge, not a working accelerator. Its output is the
input of every later gate.

### D6 — Only if the kext does not load: find the missing property

Compare the `IOKitPersonalities` entry of `AppleParavirtGPU.kext` (read in the
guest with `plutil -p`) against the identity the device declares. Adjust the
device, not the kext: the kext is signed and cannot be modified.

## Out of scope

These are deliberately not part of this plan, and no output of it may claim
them:

- Any Metal rendering. That is gate G4 and needs `MXAIR` (AIR → SPIR-V) and
  `NVK`, per the existing plan.
- Any performance measurement or comparison.
- Any change to the `driver/` kext sources, which do not exist yet.
- Migration, HDR, multi-head, USB passthrough on this path.
- CI: nothing here can run on the hosted runners.

## Risks

| Risk | Effect | Mitigation |
|---|---|---|
| The device cannot bind without the real Apple ROM | D3 fails at the first row | the ROM path is a property; a measured empty ROM result still answers the question |
| The guest needs registers the trace cannot explain | D5 grows without bound | stop after the first 100 registers and report the shape of the gap |
| The host has no IOMMU and no `vfio-pci` | irrelevant to this path, which never passes a real device | the plan does not use `vfio` |
| The account cannot start a macOS runner | no automated evidence | all measurements are recorded on the operator's host under `artifacts/mxgpu/` |
| Apple licence breach | legal exposure for the operator | stated in `docs/GPU.md` and in `macxvm doctor`; the operator decides |

## Validation

The gate passes only when **all** of the following are true:

1. `qemu-system-x86_64 -device help` from the runtime lists `apple-gfx-pci`.
2. The probe machine still boots and reports the Bochs display with the
   unmodified args file (the control state is not broken).
3. The `reims` machine starts, and the guest log, `kextstat` and `ioreg` are
   captured in `artifacts/mxgpu/d3/`.
4. One of the four rows of the decision rule is recorded with its evidence.
5. `driver/research/` gains no claim that is not backed by a trace or a probe
   result.
6. `go vet ./...` and `go test ./...` still pass; no source change is required
   for this gate, and if one becomes necessary it is its own task.

## Open question for the operator

**Do you accept the Apple licence exposure of running this measurement on the
i5-14400F host, and do you want the probe machine created on that host now?**

- Recommended: yes, because the whole gate is a measurement on your own
  hardware and the machine is deleted afterwards.
- The alternative is to run D1 and D2 only, and to stop before D3.
