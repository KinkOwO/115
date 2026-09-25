# `[seeking]` monster quest items — confirmed baseline 2026-09-25

## Breakpoint

- In session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_174022_205850_next37`, character 11 completed quest 3587 at `10:10:13Z` and received no successor.
- Current quest source makes 3594 the direct successor of 3587. Quest 3594 is a level-65 epic `[seeking]` objective for item `10164797 x1`; quest 3596 requires 3594. The item is supplied by `[monster reward item]` row `66312, 93, -1, 10164797, 1, 100, 1`.
- The previous quest index had no plain `[seeking]` model, so 3594 was filtered before acceptance.

## Current-source scope

The current catalog contains 155 `[seeking]` rows. Three epic rows share the fully source-backed form implemented here:

- 3292: dungeon 46, six required items and nine distinct monster/item sources after exact duplicate rows are removed.
- 3293: dungeon 41, one required item and one monster source.
- 3594: dungeon 93, one required item and one monster source.

Each supported row has an even list of positive `(item, amount)` objective pairs, epic grade, subtype `-1`, matching `[dungeon info]`, and one or more seven-integer monster reward rows. This candidate accepts only the current rows whose difficulty is wildcard `-1`, probability is `100`, and reward stack limit covers the row amount. Other `[seeking]` shapes remain filtered.

## Generic implementation

- The quest index parses objective items and monster reward rows without quest-ID branches, validates that every objective item has a source, and removes only exact duplicate monster/item/amount rows.
- A confirmed player-owned death in the accepted quest's own maze and dungeon grants the configured quest item directly into the ordinary bag. The item creates no visible scene object. The character event is keyed by run and monster entity, so a replay cannot duplicate the item.
- After the committed direct grant, held objective items are counted and the quest trigger changes from pending `1` to ready `0`; an incremental inventory update and the existing NOTI291 snapshot publish the bag and objective changes.
- Submission removes the objective items inside the same storage transaction as experience, gold, item rewards and quest completion. The gateway then republishes the ordinary bag so the removed quest item disappears immediately. Retry receipts preserve idempotency.
- No packet shape, database schema or client patch was added.

## Verification

- Catalog coverage tests assert that the implementation recognizes exactly the three current epic rows, including 3594's prerequisite 3587 and 3292's duplicate/source fan-out.
- Inventory tests cover removal across multiple stacks, rejection when the required amount is missing, and preservation of unrelated character state.
- Inventory tests verify that item 10164797 is a `[quest]` stackable in the full current item index and is granted directly into an ordinary bag slot.
- `go test ./...` and `go vet ./...` pass. `go run ./cmd/charactercheck` reached the existing live-save fatigue clock guard and stopped with `reconnect/backwards clock reset fatigue`; this change has no schema migration.
- Candidate `bin/wireprobe-handoff-source.exe` SHA-256: `4CB86FADE626AEF3FC1F2CE036ECA274DF9EBB3BECAE70AB21BB496E928669BF`.

## Manual validation

The user confirmed the complete flow: quest appears and can be accepted; killing the target grants the quest item directly to the bag (no visible ground drop); the quest completes and can be submitted. This confirms the direct grant semantics for `[monster reward item]`. This is the confirmed baseline. The static source coverage for 3292 and 3293 remains; those two routes were not separately played.
