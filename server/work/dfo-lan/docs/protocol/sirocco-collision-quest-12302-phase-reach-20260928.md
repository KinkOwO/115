# Collision (quest 12302): Central Tent range target

Status: quest 12302 user live confirmed on the source-based candidate. The generalized follow-up is attempt 2/3 for this quest function and has code-level validation. No C2S packet layout was changed.

## Evidence

- The user's screenshots show the level-97 Collision objective at the Central Tent and a central dais that blocks walking directly toward the upper NPC.
- The 115 client PVF export gives quest 12302 (`siroco_02.qst`) as `[reach the range]`, subtype 0, `[int data] 100000374 2000 2000`, after quest 12301. It is a range objective, not an NPC submission objective.
- The exported world source places NPC 100000374 at `(515,114)` in the phase maps for town 40/area 3, including `westcoast_d.map`. The base map has no matching NPC. The area's walkable source rectangle begins at Y=160; `(515,160)` is reachable and lies inside the quest's NPC-centered range.
- The existing passive progress path queried base-map NPC positions only. It could not find 100000374 in this area and could not advance 12302 from movement.

## Confirmed candidate

For accepted quest 12302 in town 40/area 3, the server used the source phase-map placement when every matching phase row agreed on its coordinates. The existing rectangle check, accepted-state check, owner check, source checksum and persisted `ReachNPC` model remained in force. The user manually confirmed that the quest could be completed.

`go test ./...` and `go vet ./...` passed with Go 1.26. The isolated candidate is `bin/wireprobe-quest12302-phase-reach-20260928.exe` (SHA-256 `E611CD2079F514A5BEF88DD3362668CE4490A09EFDEE8C486E9C800414D1DEB5`).

## Generalized follow-up

The confirmed quest-ID and area branch was removed. Every accepted subtype-0 NPC range objective now checks its source phase-map placement if the base map placement does not satisfy the range; multiple phase placements must agree on coordinates. This lookup does not grant NPC dialogue or bypass the accepted-state, owner, source-checksum and progress-model checks.

`DFO_QUEST_NPC_DISTANCE_MULTIPLIER` expands server-side NPC proximity. Its default is `1`. Set it to a finite number at least `1`, for example `2`, before starting the server. Invalid or smaller values use `1`. It scales the local dialogue proximity radius and the source NPC range width/height; fixed map rectangles and client movement/collision stay unchanged. The server's native CMD33 source-position check uses the same multiplier. The setting is deliberately server-side: the client still decides when to send its native requests, so passive movement progress is the fallback for blocked approaches.

No client, DLL, database schema or save-format change was made. `go test ./...` and `go vet ./...` passed after generalization. The final isolated candidate is `bin/wireprobe-quest-npc-distance-20260928.exe` (SHA-256 `FCD25E0293B3EB672E8E8EAD8D24521AD0E19CC3D2B38B820F8DFC59BA931A19`). The broader phase-map behavior and nondefault multiplier have automated coverage but await separate user live validation.
