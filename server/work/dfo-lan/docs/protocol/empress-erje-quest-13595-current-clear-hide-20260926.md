# Empress Erje quest 13595: current clear/hide is a future effect (2026-09-26)

Status: user live confirmed. Attempt 1/3 for the quest-13595 CMD33 refusal.

## Evidence

- The user's screenshot shows the level-94 act quest “Empress Erje”, requesting a talk with Princess Erje in Empyrean's Palace. Its Quests menu is empty.
- The manual sessions `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_224355_999147_next37/events.jsonl` and `..._20260926_224507_665226_next37/events.jsonl` record native CMD33 for quest 13595 in town 6/area 2. Both were refused as NPC 100000305 absent from the area's base map.
- The 115 source phase map `map/cataclysm/town/gent_afterwar/imperial_palace.map` contains NPC 100000305 at `(563, 63)` (SHA-256 `864d17b763690a0c58a7deeeb79702c13a7b9911c8e707d7f55b96abf4db6d79`). Quest 13592 shows that NPC on clear; quest 13594 guides it to town 6/area 2. Quest 13595 is an accepted `[meet npc]` objective with matching completion NPC. Its own clear/hide rule removes Erje **after** the meeting quest is finished.

## Candidate

The prerequisite visibility walker now treats the current accepted meeting quest's clear/hide rule as a future effect when a matching source guide or current-area phase-map row supplies location evidence. The opt-in no-guide relaxation keeps its previous stricter behavior. The walker still follows prior clear/show and clear/hide rules in source order. Existing accepted-quest, objective and version checks remain in force. No packet, database, save-format, client or DLL change is involved.

The source-catalog regression covers quest 13592 show, quest 13594 guide, quest 13595 future hide, phase-map placement, and wrong area/NPC refusal. The existing relax-switch regression caught an overbroad first draft; the corrected candidate keeps that check passing. `go test ./...` and `go vet ./...` passed. The isolated candidate is `bin/wireprobe-quest13595-clear-hide-20260926.exe` (SHA-256 `53BCC1A57C0182C1DE6F378F23D8960DE6325C7CBD0FA694AD41D61634120AED`), also staged at `bin/wireprobe-handoff-source.exe` after the game server process exited. The previous general candidate was backed up as `bin/wireprobe-handoff-source-before-quest13595-20260926.exe`.

## Manual validation

The user confirmed quest 13595 could be completed. The manual session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_225250_449110_next37/events.jsonl` records `quest_npc_objective` at 14:53:22Z and `quest_finished` for 13595 at 14:53:24Z. The same session then records completion of quest 650.
