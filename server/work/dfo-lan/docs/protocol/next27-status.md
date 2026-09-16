# DFO current-client candidate 27 — 2026-09-11

## Running candidate

- Binary: `bin/wireprobe-dungeon27.exe`
- SHA-256: `6c2db165265b0c235044519129ec8f4607ea4e081898aabf78fb5fbbb812fb89`
- Skill projection: `configs/skills.next27.json`, 3,224 source definitions, no unreadable source rows.
- Skill projection SHA-256: `540513f3b7568d296e6ab96434d1a0a3640e9193d54c2c14bdee7fecf0b4f91e`
- Runtime: `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_27_next27`.
- Launch snapshot: helper 27576, server 25564, probe 27708, owned client 27688, loopback port 58147. These identifiers require rechecking before any operation.
- Candidate 26 closed normally before launch. Original client, previous executable, PostgreSQL and Redis were preserved.

## Character deletion

The previous request allowlist/dispatcher omitted CMD6; the client's pending UI locks therefore never received a reply. The current sender at `1467bb1a0` writes a roster slot and the exact confirmation name. Its string helper `146d76080` writes the UTF-8 byte count and bytes. This differs from assuming a numeric confirmation code.

The current handler `1452519d0` consumes a success flag and slot, calls roster removal `140235650`, clears both busy flags through `14023dfd0` and `14023dfc0`, and refreshes the UI. CMD6 success is `01 00 <slot:u16>`. Failure code 2 follows the existing native unlock path. CMD637 returns the empty pending-deletion list for immediate local archival.

Storage locks the owned account and checks both the current slot and exact name. It archives the role with `deleted_at`, retaining state and dependent records. Stale/replayed slots cannot delete a shifted neighbouring role. Archived names remain reserved; active slot capacity is released and durable identities are not reused.

Live candidate 27 received four distinct successful deletions, at 07:32:39Z, 07:32:57Z, 07:33:24Z and 07:33:54Z. Each produced a CMD6 response and a fresh roster. These were incoming real-client requests; automated database checks only used temporary schemas. Successive client actions establish that the first deletion did not leave the entire input flow permanently locked. Screenshot verification is unavailable because the computer-use capture API returns `SetIsBorderRequired ... 0x80004002`.

Evidence: `character_delete_handlers.asm`, `delete_name_and_skill_tail.asm`, `character_delete_oracle.py`, `delete_pending_oracle.py` in the adjacent probe-tools directory; native fixtures under `internal/game/protocol/testdata`.

## Skills, SP and layout

- CMD28 ordinary current sender: tree, from slot, to slot, signed context -1. Fourteen quick slots are observed in the native sender; the local ordinary palette is conservatively bounded below special slot 138.
- CMD29 current sender: tree, count, `(id:u16, refund:u8, delta:u8)` rows and four ordinary tail bytes. The current response also needs two extension-presence bytes missing from the 90 reference shape.
- Own base-profession manual SP learning checks current PVF required level, rank interval, advancement cap, prerequisite and purchase cost. Other-job skills, refunds and unsupported advanced/TP/preset modes receive a business refusal.
- Learned levels, SP and layout persist atomically, preserving unrelated character modules. Retry receipts prevent duplicate SP deductions. NOTI19 now carries saved SP and TP rather than resetting the client display to zero.
- Native reader/setter oracles validate the response lengths, updated skill, slot, SP/TP and unlock calls. PostgreSQL tests verify concurrent retry, ownership, reconnect and layout persistence.
- Live skill learning and dragging have not yet been accepted visually. Automatic passive growth, advanced occupations and all special palette modes are not claimed complete.

## Pickup and room progression

- CMD43 was absent from the request evidence/decryption gate. It now reaches the pickup handler, and the decoder accepts the actual 24-byte cipher-padded form of its 21-byte payload.
- A current map 76123 cinematic actor reports disappearance with killer 65535. This is accepted only for the source noncombat actor, never for an enemy or unknown identity.
- Corrected a previous semantic error: the final spawn u32 is **team**, not HP percentage. The native chain is NOTI29 → `145b0d910` record+40 → `145b219d0` → `145b144f0` → virtual+a40 → `145dd6080` → `145d87580`, storing actor+f10. Source team 0 is now transmitted for friends; enemy default remains 100. Dummy status alone does not change team.
- Mixed-team native fixtures and the complete source Mirkwood route validate actor indices, affiliation, enemy-only clear gating, cinematic disappearance and no duplicate respawns.
- Existing owner/room checks and durable pickup receipts cover gold and supported source stackables. Equipment drops and original server probability parity remain incomplete.

## Validation and remaining limits

Passed `go test ./...`, `go vet ./...` and isolated PostgreSQL `go run ./cmd/charactercheck`. The latter includes concurrent deletion/learning, source skill gates, saved layout, loot, quest, fatigue, progression and free-card settlement. Native oracles execute original packet readers in private emulated memory with game/UI calls stubbed; they are not a substitute for real-client gameplay acceptance.

The reported one-hit monster balance and level-up pause are **not confirmed fixed**. The current MOB files and base/difficulty tables have been exported under `runtime/monster_origin` and `runtime/monster_tables`; their complete HP/damage application still needs tracing. Do not treat the corrected spawn team field as an HP fix, invent an HP multiplier, or claim all gameplay complete. A current live capture of those behaviours and of skill/loot interactions remains necessary.

The supplied 90 archive is a compatibility reference, not verified original server rules. Current-client native readers and this client's PVF remain the data/protocol authority.
