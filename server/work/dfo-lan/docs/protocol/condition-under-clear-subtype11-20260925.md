# `[condition under clear]` subtype 11 — confirmed 2026-09-25

## Breakpoint and source evidence

- The 2026-09-25 live session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_163135_847513_next37` records character 11 finishing quest 3540 at `08:42:15Z`. Quest 3543 is its level-59 `[epic]` successor. `InitialProgress` previously rejected every `[condition under clear]`, so the available-quest filter omitted 3543; the level-60 successor 3546 requires 3543.
- Current 115 PVF export has two subtype-11 three-cell rows: quest 3543 `[83,-1,6]` and quest 13130 `[100002783,-1,7]`. Both are `[epic]`, with matching `[dungeon info]` and a quest-specific maze. Dungeon 83's quest-3543 maze has seven concrete rooms; dungeon 100002783's quest-13130 maze has eight. The final cell matches the number of non-start rooms in both cases. Other `[condition under clear]` subtypes have different shapes and remain unsupported.
- The current client IDB maps the encrypted `[condition under clear]` string at `0x14B2C2B10` from the quest-type table at `sub_147668960+0x9729`. This establishes the distinct quest type, not the subtype-11 completion predicate. Historical `usdof` notes treat subtype 11 as all-room clear, but are only a clue. The two current source mazes and server-owned room traversal facts are the basis for this scoped candidate.

## Generic source-shape implementation

- A generic decoder accepts the subtype-11 three-cell shape when its objective dungeon and minimum difficulty agree with `[dungeon info]`. It stores a distinct progress model, starting pending at 1; it has no quest-ID branch.
- At confirmed dungeon completion, the server requires the accepted quest's own maze, matching dungeon and difficulty, no layered room alternatives, a final-cell value equal to the number of non-start maze rooms, and evidence that every configured room was visited. Movement already requires clearing each room before leaving; completed boss settlement proves the last room is clear. The accepted quest's progress is then set to 0 using the existing model/source-checked update and the existing NOTI291 trigger sync. No packet or schema was added.
- The current source catalog test covers both quests, their maze shapes, the 3540 prerequisite, and rejection of missing rooms, foreign mazes, and foreign dungeons. `go test ./...` and `go vet ./...` passed. Candidate `bin/wireprobe-handoff-source.exe` SHA-256: `ACD04507B2F9317551F3722BE9AD68EFF64AA64FB5CBCC393760A445F48A0DC2`.

## Live result and confirmed baseline

The player manually accepted 3543, entered dungeon 83's quest maze 4, and cleared its seven rooms (including the upper branch). The server recorded room maps `92370`, `92375`, `92377`, `92376`, `92378`, `92379`, and boss map `92380`. At `09:04:22Z` it sent the updated quest trigger; the client submitted and completed quest 3543 at `09:04:27Z`, then accepted successor 3546. This confirms that all configured rooms are required and that completing the source maze satisfies the objective. Session: `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_170128_909272_next37`.

No client patch or schema change was needed. The same generic parser covers 13130's matching eight-room maze shape; that quest has source/static coverage but was not separately played.
