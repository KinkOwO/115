# Zasura quest 6357 empty NPC quest list (2026-09-26)

Status: user live confirmed. Quest 6357 displays and completes through Zasura's Quests menu.

## Evidence

- The user's screenshots show level-89 “The End of the Abyss” active, Zasura visible in Black Market with a Quests menu, and an empty list after opening it.
- The current quest catalog has quest 6357 (`devildom_war_28.qst`) as `[meet npc]`, objective and completion NPC 100000175. Its source `[pre required quest]` names 6356.
- Quest 6356 (`devildom_war_27.qst`) declares `[npc visibility]` for NPC 100000175 with `[condition] [clear]` and `[visibility] [show]`. Quest 6357 has no visibility block of its own. The static town catalog contains no NPC 100000175.
- The user's manual 2026-09-26 18:15 session (`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_181551_054700_next37/events.jsonl`) recorded three native CMD33 requests for quest 6357 in town 54/area 1, each refused because NPC 100000175 was absent from the static area list.

## Candidate

The server now recognizes a quest objective NPC revealed by the `[clear] [show]` visibility rule of a named prerequisite. The observed 6357/NPC 100000175/Black Market 54/1 request passes the static NPC check with the environment switch off. `DFO_QUEST_VISIBLE_NPC_RELAX=1` applies the same source rule to other such quests without an area check. The native CMD33 form and the existing accepted quest, objective and catalog-version checks remain in force. No packet layout, client resource, or database schema changed.

The existing quest 6200 behavior remains covered by its own confirmed evidence. Source-catalog tests for both visibility forms and wrong NPC/quest cases passed. `go test ./...` and `go vet ./...` passed. The source candidate was built as `bin/wireprobe-handoff-source.exe` (SHA-256 `02E695230A98FC66B009BF80A2F5859C30DFC974B223F52A8F8E6D6F11DDE6BC`).

## Manual validation

The user confirmed that the previously empty Quests list now allows quest 6357 to be completed. The client was operated manually by the user.
