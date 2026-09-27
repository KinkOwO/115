# A Snake in the Hole quest 13588: Ghent phase NPC (2026-09-26)

Status: user live confirmed. Attempt 1/3 for quest 13588 CMD33 area presence.

## Evidence

- The user's screenshot shows level-94 act quest “A Snake in the Hole” asking to show Cohen's map to Woon Lyonir in Ghent. Woon's Quests menu is empty.
- In the manual session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_215327_494151_next37/events.jsonl`, the client accepted quest 13588 and repeatedly sent native CMD33 for that quest in town 6/area 3. The server refused each request because NPC 100000304 was absent from the area's exported base map.
- Quest 13588 (`skywar_31.qst`) has `[meet npc]` objective and `[complete npc index]` 100000304. Its direct prerequisite is quest 13587 (`skywar_30.qst`). Quest 13587 shows NPCs 100000299 **and** 100000304 on clear in one `[npc]` list. The server's visibility reader previously recognized only the first entry.
- The current 115 client inner PVF contains `map/cataclysm/town/gent_afterwar/gent_dock.map` with source `[NPC]` row `100000304 [right] 845 180 0`. The exported area 6/3 definition lists that afterwar map under `[phase]`, while its base map lacks this NPC. The endwar variant does not contain Woon.

## Candidate

The visibility reader now checks all numeric IDs in each source `[npc]` list. For this observed native CMD33, the server accepts quest 13588's Woon interaction only in town 6/area 3 when its direct prerequisite's source clear/show rule names Woon. The existing `MeetNPC` path still checks the character's accepted quest and matching objective. Other areas and unrelated quests remain subject to the normal map presence check.

This changes no packet layout, client file, database schema or save format. The phase-map exception is confined to the area and quest proved by source resources and the user's live trace; broader phase selection remains separate work.

The source-catalog regression, `go test ./...` and `go vet ./...` passed. The isolated candidate is `bin/wireprobe-quest-ghent-phase-20260926.exe` (SHA-256 `74320ACDD2C760FDD3D51C62198FC4665956182EBD7550557D0D827DEC67276C`).
The same bytes were staged at `bin/wireprobe-handoff-source.exe` for the manual `--source-build` launch after confirming no such process was running. Its previous bytes were copied to `bin/wireprobe-handoff-source-before-quest13588-20260926.exe` (SHA-256 `7EBB27C1D414E98FDF8C23DD0C9227FE92297899A68E04432FA460E0C8B81965`). These local binaries are not committed.

## Manual validation

The user confirmed Woon's Quests menu worked and quest 13588 could be completed. The manual validation session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_222815_570045_next37/events.jsonl` records `quest_npc_objective` at 14:28:50Z, `quest_finished` for 13588 at 14:28:53Z and acceptance of follow-up 13589 at 14:28:54Z.
