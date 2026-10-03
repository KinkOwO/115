# Ordinary Hell Party loot: live evidence and implementation gate (2026-09-27)

Scope: Trombe Power Plant, dungeon 103, CMD16 Mode 1, seal map 60069. This is distinct from Boundary of Attunement. The user manually confirmed the pillar appears, can be destroyed, and spawns monsters; they also confirmed killing those spawned monsters. Loot remains incomplete.

## Live result

Source log: `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_033055_321634_next37/events.jsonl`.

- NOTI27 enters maps 42051, 42052, 42053, then seal map 60069.
- The seal map's source `[monster]` has eight fixed rows (templates 64023, 64017 four times, 64023, and 64016 twice). The server registered eight actors and received eight CMD39 `ENUM_CMDPACKET_DIE_MONSTER` reports in that room, entity IDs 0x1018 through 0x101f. All eight NOTI38 confirmations have zero scene-drop rows.
- After those reports, the trace has no further CMD39 in the Hell room even though the user killed pillar-spawned monsters. CMD38 frames in the trace are `ENUM_CMDPACKET_USE_SKILL`; they do not identify Hell monster deaths.
- `loot.Session.Death` only calls the ordinary `RollWithBonus` table. It has no Hell Party actor classification or reward path. The roster currently comes from fixed map `[monster]` and `[ai character]` rows, not pillar-spawned monsters.

## Current-client PVF data

Source: `server/work/client-build/Script.inner.pvf`; extracted under ignored `runtime/hell-loot-pvf` and `runtime/hell-map-pvf`.

- Map `map/trombe/hell_14006(3,0)n.map` has a Hell Party special passive object (template 30528) and `[hellparty]` triples `(group ID, rate, order)`.
- `etc/hellparty.etc` maps group IDs to lettered monster lists and has difficulty fields, but reward-count and wave-selection semantics are not yet proven.
- `etc/itemdropinfo_monster_hell.etc` has `[dungeon difficulty drop prob]`, `[basis of rarity dicision]`, `[item drop ref table]`, and `[item drop rarity control]`. The 45-200 level row is `335 875 1000 1400 1000` for five difficulty columns. Denominator and complete selection semantics have not been established for the current client/server path.
- `etc/helldropepicitemtable.etc` has weighted item lists partitioned by `[drop area]`; the dungeon-to-area mapping and epic roll trigger are unproven.

## Native reader boundary

NOTI27 handler `sub_1452B7100` parses monster rows and additional groups. Actor source index branches in `sub_145B20DC0` include `<64`, `64..127`, and `10000`; the `64..127` branch consults map event-monster positions. NOTI666 handler `sub_1452AFF70` reads u32/u32 pairs; downstream `sub_145B4BAA0` iterates those pairs and creates client actors by template. Trigger, pair semantics, and the relationship to the pillar are still unclosed. These are evidence leads, not a verified server encoding.

## Required before a runtime change

1. Close current-client NOTI27 actor-row and group-row semantics for Hell Party templates, including source index, flags, entity allocation, and appearance trigger.
2. Confirm whether server-owned Hell actors cause CMD39 reports. A controlled run must show their entity IDs and templates in the server roster and subsequent CMD39 reports.
3. Derive reward count, probability denominator, rarity selection, epic-area mapping, and item eligibility from current PVF/IDA and a controlled live roll. Do not award Hell loot to the eight fixed map monsters.

No loot runtime attempt was made. The earlier entry probe remains attempt 1/3 for this feature; this document is static and log analysis only.
