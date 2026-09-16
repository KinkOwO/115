# Current-build dungeon compatibility

Source: inner PVF checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`. Only dungeons 3 and 7115 are imported; direct tutorial routing remains unsupported. Original client files remain untouched.

## Accepted runtime evidence

- live20: the user reached the dungeon selection list after CMD15 -> ACK15 -> NOTI27. NOTI27 is 36 bytes, confirmed by original native packet-reader control flow.
- live21: the user entered the map after CMD16 -> ACK16 -> NOTI28 -> NOTI29 -> CMD37 -> ACK37 -> NOTI30. User reported wrong speed and monsters dying immediately. Live21 also contains repeated bodyless CMD42 and successful subsequent town movement; visual selection-page exit still needs explicit acceptance.
- No dungeon completion, battle rewards, or quest completion has been accepted. Quest 3145 still requires an actual clear of source map 76126. Entry never grants progress or rewards.

## Fixes prepared after live21

- Monster record: the second u16 is the unique ID, the first byte after the template is the level. Confirmed by native constructor 145b0d910 and named argument log in 1452b6492, then replayed through the original NOTI29 decoder. Previous mapping gave all four monsters ID3 and level0. Each now gets a distinct ID4096..4099, source index0..3, source template109014858 and source basis level3. Tail initialization scalar uses the native local spawn default100 (145e9ee2f and1452b653e); further live HP acceptance required.
- Combat cipher slot11: original key setup146d945d0, encode146d945c0 and decode146d945a0. Four native vectors plus actual live21 CMD39 checksum verify direction. Decoding a death report does not establish a server-authoritative kill or clear.
- Character source scalar conversion: original .chr loader147557e90 scales numeric attributes by10. Entry previously omitted this for speed, recovery, jump, inventory, weight and resistances. The original loader also orders dark before light. Independent native conversion fixtures cover all17 professions. MP regeneration retains source*10 even though the runtime getter has a different unit.
- Initial skills: original .chr loader147559d80/dda stores two separate (ID,value) vectors. The first ID/level pairs are now supplied to both initial skill builds. All imported initial sections have second value1; other shapes and advancement skills are rejected until supported. No skill purchases, arbitrary grants, or inferred unlock thresholds.
- Fatigue: configurable local room_cost1, daily156, UTC reset. A durable PostgreSQL ledger keyed by character/run/map charges on successful room loading exactly once, with ownership checks, serialized rollover, exhaustion rejection, and source no-fatigue exemption. This is an explicit local consumption policy, not a recovered official-server rule. Reconnects preserve usage; failed selection and town movement do not charge. Subsequent room traversal remains to be implemented.

## Validation and remaining work

Native tools: `dungeon_gate_oracle.py`, `dungeon_map_oracle.py`, `source_stats_oracle.py`, `combat_cipher_oracle.py`. These execute original parser/arithmetic instructions in private emulation; game/UI effects are stubbed. They are not visual acceptance.

Temporary-schema charactercheck verifies concurrent duplicate room loading, new-room charge, exhaustion, exemption, ownership, reopen, rollover and new-run charging. It never changes user characters.

Pending: live movement/animation/skill/monster health acceptance, room transition acceptance, clear/result flows, durable source quest objectives and actual reward inventory. Do not advertise complete dungeon or quest gameplay based on map entry alone.

## live23 findings and live24 correction

- live23 received all four first-room CMD39 reports. Each generated ACK39 and empty-drop NOTI38 once; the previous repeated death reports stopped. No CMD45 was sent by the client. This is death acknowledgment, not proof of correct HP, rewards, or room clearance.
- NOTI28 offsets8/9 are **boss XY**, written to dungeon+1934/+193c. Offsets10/11 are **random-hell XY**, written to +1944/+194c. The prior encoder wrongly wrote start XY followed by boss XY. The live23 readonly snapshot confirms these misplaced values. The user's minimap showed the boss marker at the start room.
- Native145b34090 compares a supplied room against the boss coordinate pair. Both minimap142b3e620 and path gate14614de00 consume that predicate. Random-hell generation145b27520 compares the second pair before loading Etc/RandomHell.etc. Native reset1452a94b2..9550 uses255/255 for absent coordinates.
- live24 sends source boss(3,0), absent random-hell(255,255). Current room(0,1) remains in NOTI29 only. Maze1 already loads the original PVF grid: (0,1)76121 -> (1,1)76123 -> (1,0)76124 -> (2,0)76125 -> (3,0)76126. Optional NOTI28 group records remain empty; they are not the maze layout.
- `dungeon_map_oracle.py` now records original NOTI28 setter values and executes both original room predicates for every room. Only(3,0) is a boss; none is random hell. This validates field placement and native logic, not visible minimap/door behavior.
- The live23 self-skill NOTI19 was rejected by protobuf IsInitialized: each tree lacked required field4. live24 includes it, along with required1/2. The independent original parser/IsInitialized oracle checks two trees and all eight Slayer source initial entries. Skills are limited to each character's own .chr initial section; no advancement skills are granted. Shortcut placement is a local initial convenience based on original skill type, not a recovered official layout.
- Bodyless CMD132 now exits the dungeon selection through native NOTI132 normal-town branch plus saved world placement. CMD45 has a verified155-byte native sender shape padded to160. The server enforces loaded state, current-room death confirmations, source adjacency and unique entity IDs; visited rooms do not respawn confirmed kills. Actual selection return and subsequent-room loading still need live acceptance.

Validation: `go test ./...`, `go vet ./...`, native packet/room-predicate and skill protobuf oracles passed. Built `bin/wireprobe-dungeon24.exe`, SHA256 `26d780da9b19d5b70d66d1a34707775355af0629b317afc017cb343d674aa9cf`. Original files and all persisted characters are retained.

Rewards remain unimplemented. Original monster base parameters, monster experience, difficulty and item-drop tables were exported under runtime/monster_tables and runtime/reward_origin. Extracted table values alone do not establish final kill rewards or drop probabilities; verify their consumers before granting currency, experience or items. Quest3145 remains pending actual source map76126 clearance and durable reward accounting.

## Candidate25 after the user supplied 90CN source

The preceding paragraph describes live24, which is preserved. Candidate25 adds current-native actor state and initial-skill fixes, boss/result boundaries, durable source map objectives, experience/growth, the no-matching-item quest reward branch, and ordinary gold/stackable pickup persistence. It is not deployed and has no new live-client acceptance. See [the candidate evidence and remaining work](reference90-next25.md), including unsupported equipment generation, complete card rewards and later story objectives. The reference archive is a compatibility implementation, not established original server source.
