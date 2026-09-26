# Quest-visible NPC relaxation switch (2026-09-26)

Status: user live confirmed for quests 6200 and 6357. Other structurally matching temporary NPC quests remain unverified. Attempt 2/3 for the temporary quest NPC interaction feature.

## Behavior

`DFO_QUEST_VISIBLE_NPC_RELAX=1` relaxes the static-map NPC presence check for a town CMD33 `[meet npc]` objective when the quest source names the objective NPC as its completion NPC and either:

- it has one `[npc visibility]` block for that NPC with `[condition] [accept]` and `[visibility] [show]`; or
- a quest named in its source `[pre required quest]` section has a visibility block for that NPC with `[condition] [clear]` and `[visibility] [show]`.

The objective must be exactly one positive numeric NPC and have no unresolved parser fields.

The native CMD33 shape, accepted quest ownership, progress model, objective, and catalog checksum remain checked. The switch does not change passive proximity progress, dungeon scene triggers, client resources, or database schema. It does allow an accepted, source-matched CMD33 without proving the character's map contains that NPC, so use it only for manual quest testing; a modified client could submit such a request from another town.

The current catalog has three clear examples of the accept/show form without a static town-map NPC: 6200 / 8000 (live confirmed only in Black Market 54/1), 12411 / 100000666, and 12952 / 100001416. Quest 12167 / 100000319 is a counterexample: its visibility block is `[clear] [hide]`, so the switch does not exempt it. Other quests may match the structural rule; this list is not an allowlist.

Quest 6357 / NPC 100000175 adds the prerequisite-clear form: quest 6356 shows that NPC on clear. The specific Black Market 54/1 interaction is enabled even with the switch off, based on the user's screenshot and native CMD33 log. The user confirmed quest 6357 can now be completed; see `zasura-quest-6357-empty-list-20260926.md`.

## Operation

Set the variable in the same PowerShell session that starts the source candidate, then restart the service:

```powershell
$env:DFO_QUEST_VISIBLE_NPC_RELAX = '1'
& .\server\Start-DFO.cmd --source-build
```

To return to the default strict behavior, set the variable to `0` or remove it, then restart the service. The already-confirmed quest 6200 path remains available either way.

## Verification

Targeted source-catalog tests pass for the switch off/on cases and reject the hide-on-clear and wrong-NPC cases. `go test ./...` and `go vet ./...` passed. The first switch candidate was saved separately as `bin/wireprobe-quest-visible-npc-relax.exe` (SHA-256 `FEC2EF88180B81DEE8C4DB3CBD311242435AB4545F72B7DE9A741A32C374EC4B`). The source candidate incorporating the prerequisite-clear form is `bin/wireprobe-handoff-source.exe` (SHA-256 `02E695230A98FC66B009BF80A2F5859C30DFC974B223F52A8F8E6D6F11DDE6BC`). Quest 6357 is live confirmed; 12411 and 12952 and other structurally matching tasks remain unverified.
