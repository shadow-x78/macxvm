# MacXVM - The migration of a machine

This page describes the migration of a virtual machine to another host: the
compatibility check, the live migration, the cancel and the rollback. The
migration works with both GPU backends, `vfio` and `reims`.

**The feature is experimental.** There is no complete driver yet and no
migration result on real hardware. The state of every task is recorded in
[artifacts/mxgpu/execution-ledger.json](../artifacts/mxgpu/execution-ledger.json).

## The commands

```bash
macxvm migrate NAME --to USER@HOST --check    # a read only report
macxvm migrate NAME --to USER@HOST --live     # the real migration
macxvm migrate NAME --cancel                  # cancel a running session
```

`--check` stops nothing and changes nothing. `--live` runs the full
compatibility check again inside the command before any stop and never relies
on an old check. `--cancel` refuses to work when there is no valid session or
when the session is stale.

## What the command checks

| Check | What it prevents |
|---|---|
| the model of the card and its subsystem identity | a destination with another card |
| the version of the firmware | an incompatible pair of the firmware and the GSP |
| the version of the driver | a destination with another driver |
| the type of the QEMU machine | a destination with an unsupported machine |
| the flags of the processor | a processor without the needed flags |
| the image of OpenCore and the NVRAM | a destination that does not hold the same image |
| the address of the card in the destination | a destination without an address for the card |
| the identity of a USB device | a USB device without an identity in the destination |

An empty field of the destination is not a reason to refuse: a missing fact on
the other side is not a difference. Only a confirmed difference is a reason.

## The plan of the migration

The plan lives in `<vmdir>/migration/migration-plan.json` and holds:

- the identity of the source and of the destination.
- the model and the subsystem identity of the card, the version of the
  firmware and the version of the driver.
- the map of the devices: the slot of the guest in the source and the address
  or the identity of the device in the destination.
- the map of the identity: the serial, the `MLB`, the `UUID` and the `MAC`,
  which stay the same in the guest so that the machine keeps its identity.
- the chain of the disks, one node per disk with its base path and its format.
- the state of the network and of the port that is forwarded.

The digest of the plan does not depend on the time of its generation, so it
stays the same after any recheck.

`--live` sends the plan to the destination next to the checkpoint of the
guest, in the directory of the migration of the destination:
`${MACXVM_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/macxvm}/vms/NAME/migration`.
The expression follows the environment of the destination, so the destination
reads the plan in the same place where it keeps its own plan. The destination
reads the plan from the path of `--plan` and refuses a plan that does not
match the session of the migration.

## The state of the session

| State | Meaning |
|---|---|
| `prepared` | the plan is ready and nothing started |
| `pre-copy` | the disks are copied and the source still runs |
| `source-stopped` | the source was stopped after the last write |
| `final-state-sent` | the final state was sent |
| `destination-validated` | the destination validated the restore |
| `source-fenced` | the source was fenced |
| `committed` | the commit record was written and the destination is the owner |
| `destination-active` | the destination is the owner and the machine runs |
| `cancelled` | the session was cancelled |
| `failed` | the session failed |

A cancelled, failed or committed session accepts no cancel. Once the commit
record is written the destination is the owner and there is no automatic
return to the source.

## The rollback

1. Read `migration/session.json` and its state, and never guess the owner.
2. If no commit record was written, stop the destination and do not resume its
   disk.
3. If the ownership of the source is certain, resume the source when it was
   stopped.
4. If the ownership or the liveness of the destination is uncertain, keep both
   sides stopped and ask for a reconciliation.
5. Delete the transient state only after the fencing is proven, and run the
   check again before any new attempt.

## The ownership of the card during a migration

A passed through card cannot be held by two sides at the same time. MacXVM
takes an exclusive lock on the addresses of the card and on its `IOMMU`
groups before it starts the machine, and it hands the file to QEMU so that the
lock stays held until QEMU exits. A migration to another host does not move
the lock; the destination takes its own lock after it proves that the source
no longer owns the card.

## What is not tested yet

- A migration on real hardware with `vfio`.
- A migration on real hardware with `reims` in the shared mode.
- The resume of a disk after a failure in the middle of the copy.
- A migration between two hosts with a different version of the definition.
