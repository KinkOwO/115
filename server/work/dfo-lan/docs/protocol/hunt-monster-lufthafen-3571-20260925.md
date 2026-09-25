# `[hunt monster]` single-target epic — confirmed 2026-09-25

## Breakpoint

In the live session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_170128_909272_next37`, character 11 finished quest 3570 at `09:16:17Z`. Its level-62 `[epic]` successor 3571 was filtered from the available list because `InitialProgress` had no `[hunt monster]` model. Quest 3573 requires 3571.

## Source shape and evidence

- The current 115 quest catalog has 72 `[hunt monster]` rows. Seven `[epic]` rows share exactly four objective integers: positive dungeon ID, minimum difficulty `-1`, positive monster template, required count `1`. They are 3571, 3579, 3740, 3752, 3754, 3849, and 6210. Each has matching `[dungeon info]`, a dedicated quest maze in that dungeon, and the target template under a maze room's `[monster]` section. Quest 3571 is `[86,-1,65472,1]`; template 65472 appears in dungeon 86's quest-3571 maze, map 57813.
- The current client IDB's quest-type table recognizes `[hunt monster]` separately from `[hunt enemy]`; the historical tuple interpretation is consistent with all seven current PVF rows. The existing server CMD39 death path already resolves a reported entity against the loaded source monster and confirms ownership; the `[hunt enemy]` level-56 path was manually verified on that event source.

## Generic source-shape implementation

- A source-shape decoder recognizes all seven single-target, one-kill epic rows without a quest-ID branch. Other grades and target counts remain outside this progress model.
- Initial progress is 1. A confirmed player-owned death of the matching template in the accepted quest's dedicated maze and dungeon changes progress to 0 through the existing model/source-checked update. The existing NOTI291 sync reports the result. No new packet, client patch, or database change was added.
- The current catalog test checks all seven source routes, target presence and the 3570 → 3571 → 3573 prerequisites. The death test rejects unconfirmed, unowned, foreign-route, foreign-dungeon, and wrong-template kills. `go test ./...` and `go vet ./...` pass. Candidate `bin/wireprobe-handoff-source.exe` SHA-256: `EDF31F1A654E24427BA5061F8DCEE39AA5C4F508525EF36D9C6A7E150C6DE57D`.

## Live result and confirmed baseline

The player manually accepted 3571 at `09:22:59Z`, entered dungeon 86's quest maze 2 and reached map 57813, whose only configured monster is target template 65472. The confirmed target death caused the server to send the updated quest trigger at `09:24:06Z`. The player completed 3571 at `09:24:46Z` and accepted successor 3573 less than one second later. Session: `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_172229_261563_next37`.

This confirms the single-target `[hunt monster]` model, player-owned death path, quest-maze restriction and level-62 successor chain. The other six source-matched quests retain static coverage and have not been played separately.
