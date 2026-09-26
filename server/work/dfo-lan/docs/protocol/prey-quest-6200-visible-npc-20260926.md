# Prey_01 / Flames of Sin NPC interaction (2026-09-26)

Status: user confirmed live. Attempt 1/3 for quest 6200.

## Evidence

- The user's screenshots show active level-86 “Flames of Sin” directing the character to Arthur in Black Market, while Arthur's menu only offers Request Chat and Gift.
- The current quest catalog defines quest 6200 (`contents/2022/new_scenario_renewal/season_7/prey/quest/prey_01.qst`) as `[meet npc]` with objective 8000, completion NPC 8000, and two `[npc visibility]` blocks naming 8000. Its prerequisite is 6045.
- The static world catalog's Black Market 54/1 map contains NPCs 605, 607, 608, 609, 611, 610, and 614, but not 8000. NPC 8000 is absent from every exported static town map.
- The 2026-09-26 16:02 live session (`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_160220_336425_next37/events.jsonl`) records quest 6200 accepted, then repeated client CMD33 `21003818000000000000000000000000` in 54/1. The service refuses each with “NPC 8000 is absent from current source area 54/1.”

## Candidate

The service now permits this exact quest/objective/area combination through the static NPC presence check, only after a native CMD33 request. `MeetNPC` still checks the owned, accepted quest, its progress model, objective identity, and current configuration version. The existing NOTI291 trigger refresh can then report zero remaining. No packet layout, client resource, or database schema changed.

`go test ./...` and `go vet ./...` passed with an isolated Go cache. The source candidate was built as `bin/wireprobe-handoff-source.exe` (SHA-256 `09B64B088443485F017AD85EDDEE0EA924FA8C0667A75F8B9A1639E822C8A939`).

## Confirmed baseline

The user confirmed that the quest interaction now advances. In the manual 2026-09-26 16:33 session (`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_163301_956534_next37/events.jsonl`), character 11 received `quest_npc_objective` at 08:33:59Z, completed quest 6200 at 08:34:05Z, and accepted successor quest 6201 at 08:34:06Z. This confirms the 6200 path only; other temporary quest NPCs still need their own source and live evidence.
