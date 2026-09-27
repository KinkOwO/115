# A Farewell (quest 12165): cinematic boss and terminal scene

Status: attempt 1/3 source-backed server candidate. Code checks pass; the player has not yet rerun this scene. No client, DLL, packet layout, or database change.

## Live failure

The player's screenshot shows an empty Pilgrim Shelter after the boss cinematic, with no controllable character. In the user-operated session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260928_023256_707347_next37/events.jsonl`, quest 12165 was accepted at line 3065. The character entered source boss map 292106929 at line 3505, then final layer map 100000295 at line 3540. No CMD39 monster-death report occurred in boss map 292106929. At line 3560 the client sent native CMD45 for dungeon 100000006, layer position (2,2), with record `000000000405dc012c010000000000000000` (landing 476,300). The server refused it at line 3561 with `no next layer map`; no following CMD37 arrived before disconnect. This is the observed blocking transition.

## Source evidence and cause

The current 115 inner PVF has checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80` in the generated catalog. Quest 12165 (`theoculus_19.qst`) requires clearing boss map 292106929. That map contains a single `[monster]` boss, template 109010772, and starts `Action/292106929.act`. The action starts cinematic 14047. The source CMT is for map 292106929 and ends with an `[ACTOR] [MONSTER] [INDEX] 0` `[DESTROY]` behavior with `[IS REWARD] 1`. The client can therefore advance without a separate CMD39 for this boss. The final map 100000295 starts cinematic 14046, whose `[CHANGE MAP]` landing is exactly (476,300), matching the native CMD45. The existing terminal-scene rule rejected that request because it required every monster in the previous objective map to have a CMD39 death record.

## Candidate and verification

The terminal scene importer now records a cinematic-destroyed boss template only when the source objective map has exactly one `[monster]` boss, its basic action starts a CMT for that map, and the CMT destroys monster index 0 with reward. The runtime accepts the terminal scene without that boss's CMD39 only for the matching source checksum, dungeon, quest maze, final map, native CMT coordinate record, visited objective map, and imported template. An additional live enemy, absent cinematic evidence, wrong route, or wrong record still rejects. After the client's next CMD37, the existing completion path sends quest progress and clear notifications. The source export produced this field only for quest 12165 among seven terminal scenes.

`go test ./...`, targeted regressions, and `go vet ./...` pass with an isolated Go build cache. The isolated candidate is `bin/wireprobe-quest12165-attempt1-20260928.exe`, SHA-256 `A7E4F401024B78F0D3B662D29B7197DA15837F38EBDE8D5B041ABCD241EF3A3D`. Manual validation remains: the player starts the source-build candidate, repeats the boss scene, and checks that the character appears and can move, quest 12165 progresses, the clear/result path works, and exit is possible. Inspect the new `events.jsonl` for CMD45 ACK, NOTI29, CMD37, NOTI291, NOTI31, CMD46, and CMD42 before confirming the baseline. Do not infer live success from the code tests.
