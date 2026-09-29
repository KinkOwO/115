# Dungeon Oath selection, 2026-09-29

## Observed failure

The player selected Nature Oath options in town and then opened the Oath window inside a dungeon. The dungeon window showed no selected option. This also blocks any claim that the corresponding Nature effects are active.

In `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_155233_122802_next37/events.jsonl`, C2S2382 commits options 3, 1, 3, and 2 in town. The source candidate sent S2C2839 with the matching selected option after C2S37 on several dungeon entries (for example lines 246–260 and 660–674). The player still saw a blank selection in the dungeon. **Attempt 1/3, resend S2C2839 in the loading response, failed live verification.** The send log proves transport output, not client application.

## Source evidence

- Current `DFO.exe` handler `sub_1405757F0` parses a 15-byte semantic prefix and passes it with `sub_145EFAFB0()` to `sub_14057D140`. The latter returns immediately for a null actor. This is a possible timing gate, but the live trace does not prove that the actor was null.
- Town entry sends a complete worn container as NOTI13, then slot updates as NOTI14, before S2C2839. Dungeon entry and C2S37 loading restoration previously sent only NOTI14. `EntryAddition` excludes slot 47 from its mode-1 detail projection. The mode-1 reader clears omitted slots; the existing clone reattach notes already document this behavior.
- The current character wears Nature core `100610059` in slot 47 and Nature crystals in slots 36–46. Extracted client `etc/115lvability2/oathpointinfo.cos` assigns core 655 points, eight `100401626` crystals at a minimum of 165 each, and three `100401627` crystals at a minimum of 205 each: at least 2590 points, above the Nature 2550 threshold. The extracted `equipmentoath/16208_nature/3/3.etc` points to the active and on-hit effects. This establishes that the reported issue is not explained by missing equipped items or an obvious point shortfall.

## Candidate 2/3

`finishDungeonLoading` now sends the existing `inventory.WornPayload` as NOTI13 after loading completion, before NOTI14 worn updates and S2C2839. This uses the same packet builder and worn-state source as town entry. No new packet fields were invented. Candidate build `bin/wireprobe-handoff-source.exe` SHA-256 `3B945D8DB7FA080CDDA8A6073FC8A096E6A32493E50DA81CA336EE31E72CF5BA` compiled successfully.

Live session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_160843_495025_next37` shows the 6751-byte NOTI13 worn snapshot including slot 47, then the 6651-byte NOTI14 update, then S2C2839 on two dungeon entries. The player confirmed **the chosen option is visible in the dungeon**. Selection restoration is live confirmed; the Oath Points display and Nature battle effects still require their own observation.

The failed first attempt and successful second attempt point to the missing worn container as the practical cause of the blank selection. They do not separately prove how the client computes Oath Points or activates Nature effects.
