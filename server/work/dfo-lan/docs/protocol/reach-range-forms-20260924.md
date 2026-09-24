# `[reach the range]` source forms — 2026-09-24

## Current 115 quest catalog audit

The source script's **first** `[type]` token defines the primary objective. The generated JSON's stale `kind` projection can instead contain its last `[type]` token. Loading through `catalog.LoadQuests` corrects that projection. One quest (21310) has a preceding `[meet npc]` and a following six-cell reach condition; its concatenated seven `objective_cells` are not a third primary reach encoding.

The corrected catalog contains **109 primary reach objectives**:

| `[sub type]` | `[int data]` cells | Count | Source interpretation |
| --- | ---: | ---: | --- |
| `0` | 3 | 20 | NPC identity, horizontal width, vertical height |
| `1` | 6 | 89 | Town, area, rectangle X origin, Y origin, width, height |

Every row has numeric cells, positive dimensions, and no unresolved source condition. The three-cell target is not necessarily the hand-in NPC: quest 3952 targets NPC 454 and hands in to NPC 1135. Examples include 3252 `[303,1000,1000]`, 3281 `[39,250,250]`, and the asymmetric 8643 `[15,100,50]`.

Six-cell quest 13748 is `[139,0,0,-240,2000,640]`. Map 139/0 has walkable Y coordinates beginning at 183. Treating `-240` as a rectangle center with height 640 ends at Y=80, entirely outside the walkable map. Origin plus extent reaches Y=400 and intersects it. The previous all-nonnegative decoder wrongly excluded this row.

For subtype 0, the first field is an NPC identity, and the last two fields define horizontal and vertical proximity to that NPC's source map position. The candidate uses an NPC-centered rectangle with the supplied numbers as full width and height (`abs(2*dx) <= width`, likewise for Y). This boundary rule is a **local interpretation** supported by the source shapes and comparison implementation in the historical `dfo115` project, not yet a confirmed native-client predicate. The old fixed 80-unit rule ignored both source dimensions and cannot represent the 20 rows consistently. The native predicate still needs an IDB or live hit before this boundary can be called final.

## Generic server candidate — attempt 2/3 for quest 3281

- Decode only the two observed subtype/length pairs. Reject malformed cells and unknown forms; use the source widths and heights in the positional check.
- Preserve the existing persisted model identifier used by accepted 3252 tasks while making its decoder and progress rule generic. A narrow startup migration translates accepted 3281 rows from the first candidate's model identifier and records full before/after snapshots.
- Six-cell rectangles now admit signed X/Y origins and use wide arithmetic for `origin + extent`.
- A catalog-wide test asserts all 109 reach objectives are indexable, including 3281 and 13748. Live acceptance, boundary crossing, hand-in, and successor availability await manual player verification.

## Selection regression and persisted-model migration

The first candidate was actually used: character 11 accepted 3281 at `2026-09-24T14:26:30Z`, storing `stormpass-3281-reach-npc-remaining-v1`. At `15:11:14Z` the generic candidate refused quest restoration with `quest 3281 requires progress migration`, aborting character entry. This was a missed save compatibility migration, before the town initialization could complete.

`MigrateQuests` now translates only accepted quest-3281 rows carrying that historical model with progress 0 or 1 into the generic decoder's persisted model. Progress, source version, timestamps, and status remain intact. `character_quest_repairs` records the full before/after snapshots. The migration is idempotent and deliberately leaves unknown models, invalid progress, completed rows, and other quests untouched.

PostgreSQL regression tests exercised pending and ready rows, excluded rows, snapshot preservation, and a second migration pass using transaction-local temporary tables. Full `go test ./...` and `go vet ./...` passed. The migration was applied to character 11: one row updated, still accepted with progress 1; audit comparison confirmed every field except `progress_model` was preserved. The source candidate executable was rebuilt (SHA-256 `2C958BC8CEEBD10360C791B4A97E36AEF7F725AF0A84AF41C10D7F80F9CA8437`). The user confirmed character selection and game entry work after the repair. This is the confirmed baseline.
