# Quest-visible NPC relaxation switch (2026-09-26)

Status: source candidate; opt-in path not yet live validated. Attempt 2/3 for the temporary quest NPC interaction feature. The confirmed quest 6200/Black Market path stays enabled with the switch off.

## Behavior

`DFO_QUEST_VISIBLE_NPC_RELAX=1` relaxes the static-map NPC presence check for a town CMD33 `[meet npc]` objective only when the quest source:

- has exactly one positive numeric NPC objective, with no unresolved parser fields;
- names that same NPC in `[complete npc index]`; and
- has one `[npc visibility]` block for that NPC with `[condition] [accept]` and `[visibility] [show]`.

The native CMD33 shape, accepted quest ownership, progress model, objective, and catalog checksum remain checked. The switch does not change passive proximity progress, dungeon scene triggers, client resources, or database schema. It does allow an accepted, source-matched CMD33 without proving the character's map contains that NPC, so use it only for manual quest testing; a modified client could submit such a request from another town.

The current catalog has three clear examples of this source pattern without a static town-map NPC: 6200 / 8000 (live confirmed only in Black Market 54/1), 12411 / 100000666, and 12952 / 100001416. Quest 12167 / 100000319 is a counterexample: its visibility block is `[clear] [hide]`, so the switch does not exempt it. Other quests may match the structural rule; this list is not an allowlist.

## Operation

After stopping the current game session, copy the built candidate to the standard source-candidate path. Set the variable in the same PowerShell session that starts the game, then restart the service:

```powershell
Copy-Item -LiteralPath .\server\work\dfo-lan\bin\wireprobe-quest-visible-npc-relax.exe -Destination .\server\work\dfo-lan\bin\wireprobe-handoff-source.exe -Force
$env:DFO_QUEST_VISIBLE_NPC_RELAX = '1'
& .\server\Start-DFO.cmd --source-build
```

To return to the default strict behavior, set the variable to `0` or remove it, then restart the service. The already-confirmed quest 6200 path remains available either way.

## Verification

Targeted source-catalog tests pass for the switch off/on cases and reject the hide-on-clear and wrong-NPC cases. `go test ./...` and `go vet ./...` passed. The currently running source-candidate process was left untouched, so the new build was saved separately as `bin/wireprobe-quest-visible-npc-relax.exe` (SHA-256 `FEC2EF88180B81DEE8C4DB3CBD311242435AB4545F72B7DE9A741A32C374EC4B`). User live validation is still needed for temporary NPC quests beyond 6200; when testing, record quest ID, town/area, CMD33, trigger refresh, submission, and successor quest.
