# The Path to Mt. Hardt quest 7978: station NPC range trigger (2026-09-27)

Status: user confirmed the quest can be completed. Attempt 1/3 for the subtype-0 static-NPC CMD33 path; the live success exercised proximity advancement, so that CMD33 branch remains covered by the source-catalog regression rather than a live hit.

## Evidence

- The user's screenshots show the level-78 adventure quest asking to board the Magatha at West Coast Station. Kargon's menu has no Quests option.
- Quest 7978 (`adventure_friends_7978.qst`) is `[reach the range]` subtype 0 with source `[int data] 11 50 50`; it is not a `[meet npc]` menu objective. Its completion NPC is 100000129, distinct from the range target 11.
- The current world catalog's West Coast Station area 40/4 base map contains NPC 11 at `(879,238)`. In the user's manual session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_143501_617255_next37/events.jsonl`, the character entered that area at `(846,242)` and the client sent native CMD33 for 7978 at 06:35:28Z. The existing handler silently ignored subtype-0 requests without a temporary-NPC visibility chain, so no objective update was sent.
- The observed trigger is 33 map units horizontally and 4 vertically from the source NPC row. The server's earlier passive interpretation treated 50×50 as a full centered rectangle (±25 each axis), so it did not advance the objective at this client-triggered position either.

## Candidate

The CMD33 subtype-0 handler now accepts a native request when its source target NPC is placed in the character's current area and the character lies within the script's two extents from that NPC. Quest ID, NPC ID, area, NPC coordinates and extents come from the accepted quest and world catalogs; the runtime branch contains no quest-specific or location-specific constants. The temporary NPC visibility path remains available. The existing `ReachNPCFromClient` store operation still requires the character's accepted quest, matching objective, progress model and catalog version. This changes no packet layout, client file, database schema or save format. The source extents are used only for this client-triggered path; the passive movement rule is unchanged.

The source-catalog regression checks the observed NPC position and character position, as well as rejection in another area and beyond either extent. `go test ./cmd/wireprobe ./internal/quest ./internal/world` and matching `go vet` passed. Full `go test ./...` and `go vet ./...` were attempted; both were blocked by the existing unrelated `internal/dungeon/tower_dazzlement_entry_test.go` reference to missing `DungeonDefinition.Tower`. The isolated candidate is `bin/wireprobe-quest7978-station-reach-20260927.exe` (SHA-256 `C1D2F0316C8417200D4E46AD6E3F97F6FBD33FC61B08B8861011E248485CEE2F`). The same bytes were staged at `bin/wireprobe-handoff-source.exe` after confirming no source server process was running; its previous bytes were backed up at `bin/wireprobe-handoff-source-before-quest7978-20260927.exe` (SHA-256 `7870E1638EEC38BE81077EEBE8AC32EFA9353EC66CC3A11C4D85B7EC4AFA5948`). Local binaries are not committed.

## Manual validation

The user confirmed completion. In `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_144330_143337_next37/events.jsonl`, `quest_proximity_advanced` advanced quest 7978 at `(865,259)` in town 40/area 4, followed by `quest_finished` for 7978 and acceptance and completion of 7979. No CMD33 for 7978 appears in that success session; this confirms end-to-end quest progression through proximity, while the new generic CMD33 fallback has only the source-catalog regression and the earlier native CMD33 evidence.
