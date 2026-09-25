# Quest 3634 completion: CMD33 nonzero trigger evidence gap (2026-09-26)

The player reported that both displayed objectives of “Militia Captain Hogas” show 1/1 after the boss settlement, but the quest remains in progress. The screenshot is evidence of the client display only; it is not a server completion receipt.

## Current-client and source evidence

- The current source quest 3634 (`timegate_14.qst`) is `[seek n meet npc]` with `[int data] 10164777 1 1298`, `[complete npc index] 1298`, `[npc index] 126`, and dungeon 71. The server selects `SeekAndMeetNPC`, initially progress 1.
- The latest manual session, `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_001903_302661_next37/events.jsonl`, repeatedly records client CMD33 plaintext `2100320e100000000000000000000000`: quest ID 3634 and first byte after the ID `0x10`. This is observed both before and after boss settlement, and again after returning to town.
- Every such request reaches `questInteraction` and is rejected as `unsupported quest check fields`, because it requires all bytes after the quest ID to be zero. No `quest_npc_objective` or `quest_proximity_advanced` event for this quest appears in that session. The boss clear publishes NOTI291 for quest 3634 with progress 1, not a completed progress 0.
- The existing CMD33 documentation covers only zero trigger fields. It cites a reference layout `u16 command, u16 quest, u8 triggerType, u8 isIncrement`, but the current-client sender and the meaning of `0x10` for `[seek n meet npc]` have not been closed in the authoritative IDB.

## Decision and next evidence

## Follow-up evidence and candidate, attempt 1/3

- Authoritative IDB `sub_1451A0370` builds CMD33 as quest ID, a 32-bit trigger type, then a conditional byte. It sends `16`, `32`, and `64` for the three indexed trigger positions. `sub_144F5DCA0` sends the first position as `16` when a source condition matches. Thus the observed `0x10` is a native trigger value, not padding. The current server still declines to interpret this client request as proof of the whole compound objective.
- Read-only PostgreSQL check of character 11 found quest 3634 `accepted`, progress `1`, model `seek-items-and-meet-npc-remaining-v1`; the ordinary bag contains item `10164777` ×1 in slot 95. The item condition is already persisted.
- The current source dungeon 71 (`epidemic_r.dgn`) has maze 2 owned by quest 3634. Its boss room is map 91798 (`91798.map`), whose `[NPC]` section includes NPC 1298. The latest manual session selected dungeon 71 with quest 3634 and completed that boss run. NPC 1298 is absent from the character's returned town 12/area 0, so the town proximity fallback cannot settle this quest.
- The candidate only advances quest 3634 on confirmed completion of its own dungeon 71/maze 2/boss room 91798, while the role holds the required item and the source boss map declares NPC 1298. It uses the existing source/model-checked objective update and NOTI291 refresh. No schema or new packet format is involved. The source-bound predicate has a regression test for missing item, foreign maze/map, and missing NPC source.

Rollback: remove the `hogasBossClearMatch` call and helper from `internal/quest/map_clear.go`, its `Service.Dungeons` field and `cmd/wireprobe/main.go` initialization, then rebuild the source candidate.

Validation: the focused source/negative-case test, `go test ./...`, and `go vet ./...` pass. The source candidate `bin/wireprobe-handoff-source.exe` was rebuilt (SHA-256 `A7F4744A52DBE1116B173FFE713F5CD9626D04B5C515203230316E44C1B93682`); the archived 39 baseline was untouched. The optional temporary-schema `go run ./cmd/charactercheck` exits at the existing fatigue-clock assertion (`reconnect/backwards clock reset fatigue: <nil>`), unrelated to this quest path; it did not alter player saves.

## Confirmed baseline

The player confirmed quest completion in the manual session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_004753_084613_next37/events.jsonl`. At `16:50:39Z`, the server sent NOTI291 with quest 3634 progress `0`; at `16:50:49Z`, the client submitted CMD34 `2200320effff0100`, and the server logged `quest_finished` for 3634, then committed inventory and experience. This confirms the source-bound boss-clear candidate. The player then requested a general rule for future quests of the same source shape; that broadening is a separate change, with its own tests and review.
