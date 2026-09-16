# Current build town-area notification

Build DFO 2.38.2.34, original SHA fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c.

Notification registration `1453141ff -> 1459a3dd0`, ID 24, handler `1452fc5b0`. Do not use the old x86 ID 800 actor packet: current notification 800 is unrelated PVP combo data.

| Field | Native read | Width |
|---|---|---|
| town ID | 1452fc679 | u32 LE |
| area ID | 1452fc683 | u32 LE |
| actor count | 1452fc755 | u16 LE |
| each actor ID | 1452fcfc9 | u16 LE |
| x, y | 1452fcfd3, 1452fcfdd | u16 LE each |
| flag A/B/C | 1452fcfec, 1452fd000, 1452fd014 | u8 each |

Native flags initialize to 0/1/1. Their complete business meanings remain unverified. Handler loads the map at `1452fcf0d -> 146d104e0`, looks up existing actors, and places or queues them. Packet `26 00 00 00 00 00 00 00 01 00 03 00 31 02 ea 00 00 01 01` is the explicit local one-actor entry experiment, not a vendor capture.

Slot 10 constructor `146d8d9d0`, key setter `146d8dd00`, copy transform `146d8dbb0`, in-place inverse `146d8da70`. Key size 8; block alignment 4. Setup masks the first key word to 31; transforms XOR each data word with the second key word and do not rotate. Sixteen independent emulated native vectors exercise 4/20/64/68-byte lengths and four key seeds.

Town/area/map and walkable rectangles are imported from the exact PVF. `town-entry-probe.json` selects a local spawn position separately from the generated source catalog. No world ID is embedded in the serializer; level and map bounds are validated before sending SELECT. Full stats/inventory mode 1 remains a separate missing bootstrap stage.

Read-only `inspect_owned_actor.py` checks the exact isolated EXE path, actor map and current map fields. Scene character existence does not prove visible game entry; client screenshot/user acceptance remains required.

## Live acceptance, 2026-09-11

Run roles_persist_select_actor_town_08 sent SELECT -> basic USERINFO -> AREA_USERS at 00:23:18 UTC. The read-only town_acceptance.json at 00:24:36 UTC recorded actor/self ID3, town38/area0, coordinates (561,234), and scene/virtual character objects present. The user confirmed visible town entry and a skipped tutorial. This passes the first town-entry gate, but does not establish tutorial, inventory, area transitions, movement synchronization or multiplayer correctness. Keep the accepted interactive window running.
