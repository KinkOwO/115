# Synopsis read state — confirmed baseline (attempt 2/3, 2026-09-23)

2026-09-23, current authoritative `client/DFO.exe.i64`, read-only session
`a7cc79e4`. The user confirmed live behavior succeeds: opening the synopsis
shows it once, and paging or closing no longer causes it to reopen. No client
patch is required. Attempt 1/3 sent an empty CMD2079 response and failed live
acceptance; it remains reverted.

## Corrected registration and state chain

The earlier TODO confused CMD2310 (ARTIFICIAL_GOD_WEAPON_UPGRADE) with NOTI2310
(SYNOPSIS_TABLE_INFO). `sub_145265880` is the former's callback and says nothing
about synopsis state. The authoritative NOTI registration is `sub_14000B900`:

- `0x14000b93f`: `sub_14599D5D0(..., 2310, sub_140F0E410, 0)`.
- `sub_140F0E410`: read u8 at `0x140f0e456`, LE i16 count at
  `0x140f0e45f`, then LE i32 IDs at `0x140f0e48b`. Count is compared as signed;
  negative IDs are discarded. The first byte is consumed but not branched on.
- `sub_140F0F7E0`: clear both sets at manager offsets 64 and 80; insert all
  supplied IDs into offset 64, then call `sub_140F0F000`.
- `sub_140F0F000`: rebuild offset 80 with eligible synopsis IDs absent from
  offset 64. Offset 96 records whether this unread set is nonempty.
- `sub_140F0EC50`: task-specific popup predicate checks offset 96, resolves
  the task's synopsis, and tests membership in offset 80.
- `sub_146967E40` at `0x146967ece`: if that predicate is true, resolve the
  synopsis ID and open UI **659** at `0x146967ef5`, then return before normal
  task detail initialization. UI **1199** is the task-detail path intercepted
  by this check; the earlier description calling 1199 the synopsis popup was
  imprecise.
- Closing the synopsis invokes `sub_141A38510`, which can reselect the task
  through `sub_1419B5BA0` / `sub_1419B24D0`. Without a read-state update the
  same predicate remains true. This static chain explains the observed cycle;
  the candidate still needs manual live acceptance.

## Request and candidate

`sub_140F0EEB0` sends CMD2079 only for an ID in the unread set. Its stack
buffer has 13 unwritten prefix bytes followed by an i32 synopsis ID; the
captured decrypted body is 24 bytes after transport padding. Player-labelled
open, next-page and X events at 13:10:36, 13:12:06 and 13:12:47 UTC all report
ID 2 at offset 13. The changing prefix is not an interaction-kind field.

The candidate records the reported ID in the character JSON `synopsis_read`
set using the existing row-locked `CommitCharacterEvent` transaction and a
stable receipt key per synopsis. It sends the entire committed set as NOTI2310
after saving; a replay resends the current full set. No new table or destructive
migration is needed. Older saves without the key represent an empty read set;
unrelated save fields are preserved. At login the snapshot follows quest lists
so the client's unread rebuild sees current quest eligibility.

For the first observed ID 2 the complete payload is `00 0100 02000000`.
There is no speculative CMD2079 success body or extra UI packet.

Tests cover player-supplied request bytes, signed count/ID limits, exact native
reader layout, corrupt input rejection, save-field preservation, accumulating
read IDs, duplicate reports, and login packet order. The user confirmed that
opening Declaration of War shows the synopsis once and paging/closing no longer
reopens it. This behavior is the confirmed baseline. The user deferred testing
the other dungeon TODO items until a later live test window.

Rollback: revert the attempt's server files and rebuild the source candidate.
The additional JSON key/receipt rows are inert under the previous code and do
not require deleting player state.
