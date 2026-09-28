# Luke quest 3939: scene completion with no boss identity

Status: **confirmed by the player on 2026-09-29**, attempt 1/3. Quest 3939 completes after the scene and settlement. This is the confirmed baseline for this task.

## Reproduction

Character `gugugaga` (character_id 11, account_id 1) was the only character with a completed 3939 row. The original quest row and its reward receipt were backed up in `runtime/quest3939-repro-20260929/before.json` and in `character_quest_repairs`, reason `manual-repro-reset-20260929`. The task was restored to accepted, remaining progress 1, with its reward receipt removed. Existing experience, currency, items, and other task rows were preserved. Completing it again may award its reward again.

Player-operated reproduction: `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_032035_479599_next37/events.jsonl`:

- line 611: dungeon 5109, maze 3 reaches objective map 312116, one monster;
- line 612: native CMD37 loading report, followed by the normal loading notifications;
- line 629: native CMD39 for entity `0x102f`, then its death acknowledgment/confirmation;
- line 633: native CMD33, plaintext `2100630f000000000000000000000000` (quest 3939);
- line 634: `quest_interaction_refused`, reason `invalid boss identity`;
- no ensuing clear-enable or result request in this reproduction.

The scene trigger did persist quest progress 0, but its response batch was dropped while building the dungeon completion plan. Thus the client received neither the task-trigger sync nor clear-enable even though the database objective had already advanced.

## Current-client source and existing native packet evidence

Read-only extraction from the running client's `client/Script.pvf`, wrapped SHA-256 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`, agrees with the exported task and map:

- current quest `contents/2022/new_scenario_renewal/season_6/luke/quest/luke_04.qst`, SHA-256 `2d5fd9095773da005cf5d509340290e2dba55816fef102ea14c0ccd499735387`: single `[clear map]` objective 312116;
- `map/cataclysm/luke/sanctum_of_birth/3939_312116.map`, SHA-256 `bb35d45ef4a31dd81f0d95fa783e1b8f1a3a8ba921a187e3cacbce372a20389d`: one template 74669, `[fixed] [normal]`, no `[boss]` rank on the monster row;
- `map/cataclysm/luke/sanctum_of_birth/action/312116.act`, SHA-256 `e279a3a3f16d06a60dc88ee33953ffb207b0fa506d2e6fe2d60b3871ab36c2e8`: `[ON START MAP]` starts cinematic 2909.

The map itself is a boss-room map, but this does not turn its sole rank-0 actor into a reportable boss. No new packet format is introduced. Existing native vectors `native_clear_enabled.json` and `native_boss_check_response.json` retain the contracts established at client readers `0x1452ae550` and `0x1452a5810`: NOTI31 consumes one u32; NOTI115 consumes a real entity identity. Read-only disassembly of the current DFO.exe reconfirmed read sites `0x1452ae59a`, `0x1452a5837`, `0x1452a5869`, and `0x1452a5873`. IDA connector was unavailable this turn; no new reader or codec conclusion relies on it. Existing source-matched closing-scene/tutorial paths already use NOTI31 without a boss confirmation.

Extraction and native listings are kept under `runtime/quest3939-repro-20260929/` and excluded from Git.

The configured client directory `F:/wip/dof/115US` has the same Script.pvf hash as the repository copy. Its DFO.exe whole-file hash differs from the frozen copy; the 1024-byte ranges at both relevant native readers were compared directly and are identical.

## Candidate

`CompletionNeedsBossCheck` now requires a nonzero `CompletionTarget` for every completed run, including normal story scenes. A completed scene with no reportable boss sends the existing clear-enable directly. A real source boss keeps NOTI115 followed by NOTI31. This only changes notification selection after the existing completion gate; it does not mark any additional room or task complete.

Regressions replay the real 3939 maze through its six doorway transitions, confirm the terminal actor death cannot complete the run on its own, then apply the existing scene-completion signal and require NOTI31 without NOTI115. A separate regression retains the actual boss confirmation identity and order. Neither test requires a player database.

No schema, catalog, client resource, DLL, reward algorithm, or protocol-layout change.

Validation: targeted regressions, full `go test ./...`, `go vet ./...`, and candidate build passed. Installed candidate SHA-256: `61434FE1412A7B1E742742CD9C0ED0721397B3BD7C75912A15F2D487879D969E`. Candidate and installed `bin/wireprobe-handoff-source.exe` hashes match. Archive39 was preserved.

## Confirmed player regression

The player explicitly confirmed quest completion. In `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_033430_711389_next37/events.jsonl`, line 622 records `quest_scene_trigger`, line 624 `dungeon_clear_enabled`, line 628 `dungeon_play_result`, and line 652 `quest_finished` for quest 3939. The character's saved task is completed, progress 0, with its reward receipt present. No invalid-boss-identity failure occurred in this completion chain. Confirmation covers this task and its settlement/submission; other scene tasks are not claimed as individually tested.

## Retest and rollback

After the player closes the client, restore only this task's remaining progress from 0 to 1 with a new audit snapshot, then install the tested candidate at `bin/wireprobe-handoff-source.exe`. The player starts the normal launcher and repeats 3939. Check for task-trigger notification, `dungeon_clear_enabled`, native result request, settlement, and successful task submission.

The previous candidate is preserved at `runtime/quest3939-repro-20260929/wireprobe-before-attempt1.exe`, SHA-256 `56066436CECAA0A69913674C5301405DEA1433362A6CD7881C7458AEE13BEA60`. With the client and game gateway closed, copying it back to the candidate path reverts the runtime change. The original pre-investigation completed quest/reward backup remains separately preserved; do not restore it over a newly advanced quest. Archive39 is untouched.
