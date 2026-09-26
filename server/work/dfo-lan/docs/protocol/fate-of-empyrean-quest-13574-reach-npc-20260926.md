# The Fate of Empyrean quest 13574: temporary NPC range trigger (2026-09-26)

Status: user live confirmed. Quest 13574 completed and follow-up quest 13575 accepted. Attempt 1/3 for subtype-0 NPC range CMD33.

## Evidence

- The user's screenshot shows level-92 act quest “The Fate of Empyrean” asking to meet Princess Erje in Slaugh Industrial Complex. The NPC cannot be clicked to open a menu, and approaching her did not advance the quest.
- Quest 13574 (`skywar_20.qst`) is `[reach the range]` subtype 0, with `[int data] 100001497 150 150`. Its `[complete npc index]` is 481. Its own visibility rule hides NPC 100001497 on **clear**, after the range objective is met. It is therefore not a normal `[meet npc]` menu objective.
- The client's manual session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_190605_934665_next37/events.jsonl` records a native CMD33 for quest 13574 at 11:06:37Z while the character stood in town 14/area 1 near the NPC. The service emitted no objective update because CMD33 previously returned without handling non-`[meet npc]` quests.
- In the user's validation session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_191315_595119_next37/events.jsonl`, CMD33 advanced the NPC range objective at 11:13:59Z, CMD34 completed quest 13574 at 11:14:04Z, and quest 13575 was accepted at 11:14:06Z.
- This temporary NPC is shown by quest 13562 on clear and guided to town 14/area 1 by quest 13563. The static map does not provide its position, so the existing passive `ProximityProgress` locator cannot settle the range objective.

## Candidate

The server now handles native CMD33 for subtype-0 `[reach the range]` quests when the objective NPC matches the quest source, an earlier prerequisite shows that NPC, and a source `[go guide]` in the same chain matches the current town/area. The current quest's clear/hide rule is treated as its future completion effect. The persistent update still requires this character's accepted quest, matching source checksum and `ReachNPC` progress model. It sends the existing NOTI291 trigger refresh when the objective advances.

The rule uses quest source fields rather than quest or NPC IDs. It adds no packet fields, client patch, database migration or save-format change. The candidate does not infer a new NPC position, so the source guide and the client's native range trigger are the spatial evidence for temporary NPCs.

Focused source-catalog checks, `go test ./...` and `go vet ./...` passed. The isolated candidate is `bin/wireprobe-quest-reach-npc-20260926.exe` (SHA-256 `658484BF51789A49878B0B77B2A228E90994113D3B5B5A351CACB189FDDC773B`).

## Manual validation

The user manually confirmed that approaching Erje advanced the range objective, quest 13574 could be submitted, and quest 13575 became available. The client was operated by the user.
