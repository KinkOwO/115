# Quest 3596 cinematic boss death — attempt 1/3

## Live evidence

- Player report: entering the Arden City boss room plays a cinematic; after it ends the boss dies and the dungeon settles, but the hunt objective stays at 0/1.
- Session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_212422_220092_next37`, character 11: at `13:26:31.604Z` dungeon 93, quest maze 3 enters map 76086 with one monster.
- At `13:27:19.771Z`, after the story pause ends, CMD 39 reports entity `0x1033` dead with killer `0xFFFF`. The server confirms that death. The client immediately sends CMD 117 for the same entity.
- At `13:27:19.775Z`, the server sends NOTI 291 with quest 3596 (`0x0e0c`) still at progress 1, then NOTI 115 and NOTI 31 to enable settlement. This directly explains the failed objective.
- Current quest source: 3596 is a single-target `[hunt enemy]` objective for dungeon 93, monster 75043. Dungeon 93 maze 3 ends at map 76086; the map contains one rank-3 team-100 monster of template 75043.

## Confirmed baseline and generic follow-up — attempt 2/3

The player manually verified the first candidate: quest 3596 completes after the cinematic death. That establishes this route as the live confirmed baseline; separate submission evidence was not supplied.

The player then requested the same rule for every supported single-target hunt, without a quest-ID exception. Both `[hunt enemy]` and `[hunt monster]` now count any server-confirmed death of their configured target in the accepted quest's dedicated dungeon and maze, including killer `0xFFFF`. The server still requires a loaded run, a matching source entity/template, and an accepted quest before its model/source-checked storage update. `0xFFFF` remains unowned for drops and experience, and foreign explicit killers remain rejected by dungeon death confirmation. Duplicate deaths do not advance twice.

The generic follow-up reuses the existing NOTI 291 synchronization and needs no schema, packet-layout, or client patch change. Unit tests cover scripted target deaths for both hunt models and rejection of unconfirmed, wrong-route, wrong-entity, and wrong-template deaths. The generic rule is code-tested; only quest 3596 has been manually replayed so far.

Full `go test ./...` and `go vet ./...` pass. The independent generic candidate is `runtime/wireprobe-hunt-death-generic.exe`, SHA-256 `FE2188CA3C1A512277D37F4A7C3ED8A74F394B411032C7911C8670E5AAD04C02`; `runtime/quest3596-repair.json` now selects it. The shared source executable remains unchanged.
