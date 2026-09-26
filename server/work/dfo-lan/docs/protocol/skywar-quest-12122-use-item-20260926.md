# Skywar quest 12122 item use (2026-09-26)

Status: baseline recorded 2026-09-27. The user authorized merging and committing the generalized quest-use path while noting that no second matching quest is currently available for another live test. The observed CMD507 action and quest 12122 flow are live-confirmed; broader catalog matching awaits another source-backed quest.

## Evidence

- The latest manual session completed quest 13595. The current quest catalog's `skywar_38.qst` rewards item 10312154 and relates to quest 12122.
- `skywar_airship.qst` (quest 12122, script SHA-256 `998acb719f793e8bac0f7fafb68225db84c97e6d01c0220eb75705524614398b`) is a level-94 epic `[use item]`, subtype 1, with `[int data] 1 10312154 1`. Its prerequisite is 13595 and its relation is 12123. Quest 12123 requires level 95.
- The item index maps 10312154 to `stackable/10312001/10312154.stk`. Read-only PVF inspection finds `[stackable type] [unlimited waste]`, `[action type] [move to airship]`, `[use action packet] 1`, and `[action usable place] [village]`.
- The current client's authoritative IDB parses `[use item]` into the quest data at `0x14730e6c8` (string `0x14b262698`); the source form above is one item entry with use count one.
- Existing `InitialProgress` had no `[use item]` branch; `Available` intentionally filtered quest 12122 as unimplemented. This is the direct cause of the missing successor.
- Attempt 1/3 used CMD44 based on the ordinary item-use path. The user-operated session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_231341_682490_next37` disproved that entry point. Right-clicking the communicator sent CMD507 with 64-byte plaintext `71000000000000ce0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000` after the player moved it to slot 113. The service rejected it as an unsupported fatigue-potion action. The owned bag confirms slot 113 contains template 10312154, amount 1, and quest 12122 remains accepted at progress 1.

## Candidate

- Only the source-backed epic subtype-1, one-item/one-use form is accepted. Other `[use item]` shapes remain filtered.
- Attempt 2/3 routed the captured CMD507 action 206 in town, consumed the item, and advanced quest 12122. The live session `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_234353_612221_next37` confirms CMD507 at line 186, `item_consumed` at 187, `quest_item_objective` at 188, and `quest_finished` at 192. The user confirms that no airship effect occurred. Thus the previous item decrement was premature and did not implement the source `[move to airship]` action.
- The character remains level 94 after submission. The live database confirms 12122 completed and no 12123 state. Source quest 12123 (`loschest_01.qst`) declares `[level] 95 10000` and `[imagine level] 94`. The service's level gate is therefore the reason 12123 is absent at level 94; do not lower it based on the display level.
- The one communicator spent during the earlier test was restored through audited grant `quest12122-communicator-restore-20260926`; read-back confirms template 10312154, amount 1, slot 72. The quest remains completed. This restored item is not automatically removed from an already completed quest.
- Attempt 3/3 follows the user's requested semantics: the native CMD507 action 206 looks up its owned bag slot, matches the template to a pending source-backed `[use item]` objective, consumes one item durably, sends the existing inventory update (NOTI14), and refreshes the quest objective (NOTI291). The template and quest ID are not hardcoded in the gateway. Quest reward submission still follows the player's ordinary CMD34 action. With the current catalog this supported epic single-item form matches quest 12122; title quests with different shapes remain outside the verified model.
- The current-client IDB identifies `[move to airship]` at `0x14B23BF08` in the source action parser and the native CMD507 request is captured, but the client reader and S2C transition for this action are not yet closed. No CMD507 acknowledgement or airship movement packet is invented. No schema migration, client patch or DLL is involved.

The attempt-2 candidate `bin/wireprobe-quest12122-action507-20260926.exe` (SHA-256 `4A3657486402BB561701E6BCE4134E3633AD1010AD021B339BC87942C2825146`) supplied the live vector proving consume → objective ready → submit. The safe attempt-2 fallback `bin/wireprobe-quest12122-safe-use-20260926.exe` (SHA-256 `499867C28A3C4EB430F1816420281CB163DC516DCD15975732EAC59861068F1B`) remains available for rollback. The attempt-3 generic candidate `bin/wireprobe-quest-use-generic-20260927.exe` (SHA-256 `D840C869CE82D32005FBD77C713EF684A5E724E10AD4FE12AA27E9F9111DF0EF`) passed `go test ./...`, `go vet ./...`, the PostgreSQL quest-use regression, and a launcher dependency check. Because the game and `wireprobe-handoff-source.exe` were still running, the live shared executable was not overwritten. The ignored `runtime/quest-use-generic-profile.json` selects this isolated candidate on the user's next manual launch.

`go run ./cmd/charactercheck` reaches its fatigue regression and exits with `reconnect/backwards clock reset fatigue: <nil>`; this candidate changes no fatigue behavior or storage schema. The quest-specific temporary-table regression above passes.

## Manual validation required

The next quest 12123 should be evaluated after the player manually reaches level 95. The `[move to airship]` effect is intentionally not part of the user-requested quest-use behavior. The restored communicator remains in the completed character's bag unless the player removes it through normal inventory operations.
