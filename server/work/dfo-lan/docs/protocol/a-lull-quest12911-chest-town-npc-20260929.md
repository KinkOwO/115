# A Lull (12911): Chest Town phase NPC

Status: user live confirmed on 2026-09-29, attempt 1/3 for quest 12911. Completion and successor acceptance are corroborated by the session trace and persisted state.

## Observed failure

The player's screenshot shows the level-101 Act quest **A Lull**, asking for Princess Hyria Grant in Chest Town, with no completion option under the NPC's Quest menu.

Session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_041820_410570_next37` records character 11 completing prerequisite 12909 and accepting 12911 at `2026-09-28T20:30:25.6040409Z`. After entering town 80/area 0, the current client sends checksum-valid CMD33 with plain body `21006f32000000000000000000000000` four times between `20:30:35.2345791Z` and `20:30:47.0902468Z`. Every request is refused with `quest 12911 NPC 100000670 is absent from current source area 80/0`.

A read-only PostgreSQL check confirms 12909 completed and 12911 accepted with progress 1, model `single-meet-npc-remaining-v1`, and the current catalog checksum. No task reset or archive repair is needed.

## Current resource evidence

Read-only `pvfinspect` exports from `server/work/client-build/Script.inner.pvf` reproduce the existing catalog source checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80` and all three script hashes below. The running client's `client/Script.pvf` matches the paired `Script.required.pvf` byte hash `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`.

- Quest 12911: `contents/2022/110levelscenario/sanctusbellum/quest/sanctusbellum_act_36.qst`, SHA-256 `2943dd54f5f6db3995f4c21b8974b4da13556698072e14325469719f19bf93e7`. It declares `[meet npc]`, `[sub type] -1`, objective and completion NPC `100000670`, level 101, prerequisite 12909, and relation quest 12930. It has no `[npc visibility]` or `[go guide]` block.
- Base map: `map/cataclysm/town/chesttown/chesttown_main.map`, SHA-256 `f70741ec9dbe6845b8d97dcb4215e998cb5c12f287470e0e54f95e0fb0755a56`. It has no NPC 100000670.
- The area's `[phase]` map: `map/cataclysm/town/destroyed_chesttown/destroyed_chesttown_main.map`, SHA-256 `f271d5b39ec0e757b1c311666e414767518cbbcd41c844d38519609b187da213`. Its `[NPC]` row is `100000670 [left] 495 182 0`. Both exported phase placements agree.

The existing phase-NPC authorization requires a quest visibility rule. That requirement cannot be satisfied here, even though the client's native trigger and source placement agree. This evidence identifies the server refusal; it does not establish a general town-phase selection algorithm.

## Correction and boundaries

Only the observed quest 12911 / NPC 100000670 / town 80 / area 0 receives the additional CMD33 authorization. The definition must retain a resolved single numeric meeting objective, matching completion NPC and subtype -1. World and quest catalogs must share their checksum, and the phase-map placements must exist and agree.

The existing native request validation, `MeetNPC` accepted-state/account/model/version checks and NOTI291 refresh remain intact. No new packet fields or codec are introduced. Passive proximity, other quests, client files, DLLs, database schema and existing saves are unchanged.

Regression coverage reproduces the old refusal from the real catalog and verifies the new allowance, source identities, wrong quest/NPC/area, unsupported definitions, missing or conflicting phase placements, and mismatched catalogs. Targeted checks, full `go test ./...`, `go vet ./...`, and the candidate build all passed with the bundled Go toolchain and repository-local cache.

Candidate `bin/wireprobe-quest12911-phase-npc-20260929.exe` has SHA-256 `83609DAEB41E4678630345C2E844A6CC0BDD70E2103BEF7A85DC7CDD7474A796`. After verifying the source server was stopped, those bytes were staged as `bin/wireprobe-handoff-source.exe`, which is the currently configured launcher binary. The previous candidate was preserved as `bin/wireprobe-handoff-source-before-quest12911-20260929.exe`, SHA-256 `99A3F17BA083754B0270E0AA6883CB6CCD85BBA2E34C1F9F8002DD3F5AEDDBFB`. The user launched and operated the client for validation.

## Confirmed baseline

The user reports success. Session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_044324_811234_next37` records the same native CMD33 at `2026-09-28T20:44:15.8171128Z`, followed by `quest_npc_objective`. CMD34 `22006f32ffff0100` at `20:44:19.1096898Z` produces `quest_finished`, reward persistence and `available_quests_updated` for 12911. A read-only PostgreSQL check confirms 12911 `completed/progress=0` and successor 12930 `accepted/progress=0`. Confirmation covers this meeting quest and successor acceptance; it does not establish general active-phase selection or all temporary NPC quests.

Rollback is the previous local candidate plus removal of the added authorization call and `quest12911_phase_npc.go`; no database rollback is needed. Local candidate backups, exported evidence and session logs stay outside Git.
