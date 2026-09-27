# Wish quest 13565: NPC revealed earlier in the quest chain (2026-09-26)

Status: user live confirmed. Quest 13565 completed and follow-up quest 13566 accepted.

## Evidence

- The user's screenshot shows the level-91 act quest “Wish” directing the player to Princess Erje in Slaugh Industrial Complex. The NPC's Quests menu is empty.
- In the user's manual 2026-09-26 18:26 session, four native CMD33 requests for quest 13565 were refused in town 14/area 1 because NPC 100001497 is absent from that static map. See `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_182650_330272_next37/events.jsonl` at 10:44:28, 10:44:31, 10:44:38 and 10:46:05 UTC.
- In the user's validation session `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_185638_805103_next37/events.jsonl`, CMD33 for quest 13565 at 10:57:24Z recorded `quest_npc_objective`; CMD34 at 10:57:26Z recorded `quest_finished`; the client then requested and received quest 13566 at 10:57:28Z.
- The source quest `skywar_10.qst` (13562) declares `[npc visibility]` for NPC 100001497 with `[condition] [clear]` and `[visibility] [show]`. The next source quest, `skywar_11.qst` (13563), has `[go guide] 14 1 100001497`. `skywar_12.qst` (13564) leads to `skywar_13.qst` (13565), whose `[meet npc]` objective and `[complete npc index]` are both 100001497. The latter two quests have no matching visibility block themselves. `skywar_20.qst` (13574) later hides this NPC on clear.

## Source-driven rule

For a native CMD33 meeting objective, the server follows the quest's `[pre required quest]` chain, with a cycle and depth guard. It accepts a temporary NPC absent from the static map when an earlier quest clears to show that NPC, no later clear rule on that chain hides it, and a `[go guide]` in the same chain names the NPC and current town/area. The existing accepted quest, objective, completion NPC and catalog checks remain in place. A quest's own future clear/show rule does not justify its current meeting request.

`DFO_QUEST_VISIBLE_NPC_RELAX=1` retains the opt-in broader behavior and can use the same ancestry rule without a matching area guide. The user-confirmed 6200 and 6357 paths remain available with the switch off. No packet layout, client resource, database schema or save format changed.

The source-catalog regression covers 13565 in 14/1, wrong area and NPC, the later hide rule, and switch behavior. `go test ./...` and `go vet ./...` passed. The candidate is `bin/wireprobe-quest-lineage-guide-20260926.exe` (SHA-256 `2A80AB939D324E6FA4E9326D2483E73C324C9A664ECD314A44A3D6933BDB067F`).

## Manual validation

The user manually confirmed that Wish appeared under Princess Erje, could be submitted, and advanced to quest 13566. The client was operated by the user.
