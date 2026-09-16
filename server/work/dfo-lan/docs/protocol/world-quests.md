# Current-build world and quest protocol

Source is the preserved DFO 2.38.2.34 build. These findings do not imply a full
multiplayer game server. First-stage create/list/select/town visual acceptance
remains separate from this stage's town, quest and detail tests.

## Live capture 09

`runtime/roles_persist_select_actor_town_world_09/events.jsonl` records checksum
validated requests on 2026-09-11 UTC:

- 35: `3102ea0005640000` = x561/y234, motion5, speed100, zero padding.
- 31: `1f00490c000000000000000000000000` = inner marker31, quest3145.
- 36: `260000000100000020023701052600000000000000000000` = town38 area1,
  x544/y311, flag5, previous town38 area0, two zero tail flags.
- 36: area2 x1085/y249 also requested. The old gateway handled none of these.

Packet 35 native writer 145bec7da..145bec893 emits u16/u16/u8/u16.
Packet 36 writer 146d07599..146d07676 emits u32/u32/u16/u16/u8/u32/u16/u8/u8.
Success36 is a one-byte true response followed by NOTI24 to initialize the map;
real-client verification of this new transition flow is pending.

Packet31 sender14519fe80 emits two u16 fields, first fixed31 then quest ID.
Success31 reader145261c2a/34 and145261d00 consumes u16 quest ID, u32 progress,
u8 party-row count; zero party rows have no additional packet reads.
Success32 reader14527ddb6 consumes u16 quest ID. Other quest operation schemas
and objective/reward messages are not yet implemented.

## Ciphers

Slot3 is standard Twofish: setup146d99ba0, encrypt146d99b80,
decrypt146d99b60, 32-byte key, 16-byte alignment.
Slot7 is modified Blowfish: setup146d90e30, encrypt146d91bf0,
decrypt146d91450, 56-byte key and 8-byte ECB alignment. Setup expands only ten
P words and the first128 words of each S-box. Standard Blowfish does not match.
Six independent native vectors pass; live31 and35 checksums establish direction.
Regenerate with `../dfo_probe_tools/world_cipher_oracle.py`.

## Configuration evidence

Map resolution147c49900 first tries the exact path, then147c4bbb0 prepends `(R)`
to the filename, excluding ANI/MOB/OBJ at147c4b610. This resolves Seria's
`Common/Gate_Seria.map` to `map/common/(r)gate_seria.map` under the map root.
Raw cells, imported scripts, source paths, hashes and unresolved conditions are
retained in generated catalogs. Unresolved rules must not become access grants.

`list/quest.lst` contains2844 entries, including3145 ->
`contents/2022/new_scenario_renewal/season_1/grandflores/quest/grandflores_01.qst`.
An earlier research assumption that this index lacked modern quests was wrong;
direct typed-cell lookup confirms the entry. The separate indexhash file is
not necessary to establish this mapping.
Quest3145 source: level1..10000, all professions, clear-map objective76126,
NPC1 to NPC2, profession-dependent reward cells. Accepting is not completing.
Type8 source cells resolve to localization keys such as
`<11::name_Arade_GrandFlores_01>`; keys remain distinct from translated text.

## Live10 trial and persistence

The opt-in loopback run uses modular world/quest services and PostgreSQL world
and quest tables. World writes use revision checks; reads and writes enforce
account ownership. Quest retries retain progress and completed prerequisites
are required. No completion or reward grant is currently exposed.
The loopback policy permits requests to source-adjacent portals without a
proximity requirement while movement is under test. This is explicitly not
the final shared LAN authorization policy.
Mode1 sends source HP/MP and base stats; the raw prefix remains experimental,
and skills/equipment/inventory are not yet populated.

`go test ./...`, `go vet ./...`, native cipher vectors and isolated PostgreSQL
world/quest ownership, revision, reopen and retry tests pass. Client acceptance
of live10 must be recorded separately from those checks.

## Live results and combined correction (2026-09-11 UTC)

- Live10 failed: mode1 caused selection/entry regression and fatigue0. It was
  disabled in11-13. The missing equipment block is now identified separately
  inentry-userinfo.md; do not treat the original495-byte packet as valid.
- Live11: user accepted town and Seria-room entry; room exit remained unresolved.
- Live12: vault click caused second-chance AV1469d6195, invalid text pointer0x101.
  Uninitialized vault grade produced index-1 into the native tier table.
- Live13: user confirmed vault opens and closes. NOTI13 personal storage kind2,
  u16 slots8, u16 itemcount0 fixes initialization. Capacity8 is extracted from
  native constructor14008e7c0; it is not claimed to be PVF-derived. Vault contents
  persist; unsupported nonempty encoding is refused without clearing items.
  During reported room-exit attempts, only USER_STATE_MOTION623 was received,
  with no position35 or area36. Thus no server destination refusal was observed.
- Live14 candidate: restored corrected mode1; sends USER_AREA23 before AREA_USERS24.
  Native23 self branch145311ecb..145312054 updates area objectives and invokes
  146d12cd0, which updates area-specific warp/effect state. Previous runs skipped
  this distinct stage. Server return authorization still uses saved origin plus
  source Seria warp geometry; no arbitrary destination or forced unlock patch.
  Enter/exit against actual source38/0->38/1->38/0 is covered in tests. Actual
  client movement, room exit, final displayed stats/fatigue remain pending.

Quest3145 is a clear-map76126 objective, not a talk-only completion. Native
addAcceptQuest144f38b60 and update145296130 mark progress0 ready. Initial0 was
therefore a false-completion bug. The supported single-map model starts with
remaining1, and active quest IDs/progress restore through SELECT's30 inline
slots and overflow. Other objective structures are refused until reconstructed.
No authoritative map-clear/combat ledger or reward application exists yet.

Cipher slot6 is native KASUMI, verified by original cipher execution and three
golden vectors. Live FINISH34 plaintext is2200490cffff0100: fouru16 fields
(inner command34, quest3145, reward selection65535, option1). A submit request
cannot prove completion. The current refusal isfalse + code19 (native cleanup
branch consumes no further body); its official business label is unrecovered.
Do not substitute code4: the loaded client string says inventory full.

LanTest01/database5's legacy accepted-progress0 record was repaired toremaining1
with a transactional comparison and a before/after row incharacter_quest_repairs.
The explicit repair output isruntime/quest_repair_char5_20260911.json. A repeat
preview yields no changes. Completed/new-model progress is never reset.

Fatigue usescharacter_fatigue, with ownership and rollover handled atomically.
Same-day reconnect and backwards-clock reads retain consumption. SELECT and
NOTI36 use the same saved state; NOTI36 follows scene initialization so its global
used/limit values are not left at the earlier baseline's empty defaults.
`configs/fatigue-probe.json` explicitly sets development limit156, UTC00 rollover.
These are editable local policy, not recovered official-server daily rules.
Town movement does not consume fatigue; dungeon charging awaits real entry logic.

Validation: native packed91 and addition cursor fixtures, Go tests/vet, actual
source Seria return geometry, isolated PostgreSQL ownership/revision/reopen,
fatigue rollover, retained vault items and quest repair audit all pass. Live14
has reached the persisted roster; combined user verification is still pending.

### Live14 failure and live15 correction

The user reported a confirmation followed by exit on selection. Live14 emitted
SELECT4, USERINFO2 mode0/mode1 and VAULT13, then silently returned on unsupported
cipher slot9 for USER_AREA23. AREA_USERS24 and FATIGUE36 were never sent. Native
exit code was0; no second-chance access violation was recorded. This is a server
entry-sequence regression, distinct from the accepted vault fix.

Slot9 wrapper146d8b240 uses a16-byte key and12 rounds. Encrypt146d8b150 calls
146d90df0; decrypt146d8afa0 calls146d90dd0; both use146d909f0. Key setup is
146d90330 through146d90e10. Tables are extracted from this exact EXE. Four
independent native48-byte fixtures pass portable Go encryption and decryption.
`area_cipher_oracle.py` reproduces the evidence without launching the game.

Live15 prepares all seven entry frames before writing SELECT. Its complete
entry test covers order4/2/2/13/23/24/36, encryption, frame lengths, checksum,
padding and no partial result on a later unsupported cipher/invalid frame.
Encoding and write errors are now logged with type, ID and stage. Full Go tests,
vet and build pass. The room-exit hypothesis and final UI stats remain subject
to live acceptance; this correction alone does not establish gameplay parity.

### Live16–18: menu and world-entry acceptance

Live16 CMD7 native acknowledgement is success + u32 zero. The client returned
to SELECT and requested GET_USERINFO8; repeated role entry works. CMD3 uses
the same reply layout and leaves socket closure to the native exit path.
CMD1301 replies success + town u32 + area u32 from the owned saved return.

Live17 added NOTI124 ENTER_GAMEWORLD_COMPLETE after the other seven entry
frames. Handler145301370 reads one u32 and sets game+1469 through1459b47f0.
Read-only snapshots show that member changed0->1. The user confirmed Inventory
and Settings open and the room portal is visible. The earlier missing stage
caused the native Please try again later refusal. No UI unlock patch was used.

The live17 room map selector sent CMD36 destination38/0,(746,157), source38/1.
That generic animation landing lies outside source walkable geometry. Live18
uses the character's saved return coordinates only after verifying a Seria
return warp and matching destination; it still validates geometry, level,
source and ownership. Normal portals retain their requested position checks.
At02:45:09 UTC the client reached38/0,(561,234), then moved to38/2 and sent
regular position updates. Revisions4–17 were persisted.

Roster fatigue is the row's final two u16 values before its last u32:
14563ea6c/76 -> info+6d8/+6dc ->140209be2 ->14020a04d renders %d+%d.
It now uses max(limit-used,0) from the same fatigue service, with bonus0.
The source's signed read caps each field at32767. This is distinct from the
in-world global used/limit fields, which were already populated.

CMD143 operation0 is u8 op + u32 index + u8 boolean (not a second u32).
character_tutorial_flags saves only client-reported flags0..100 per owned
character. SELECT restores sorted true indices through146cc6fd0. Success143
contains zero reward rows; it does not grant items or claim dungeon completion.
Isolated PostgreSQL checks cover retry, reopen, ownership and explicit updates.

The user accepted the combined live18 test (156+0, room exit, role return and
re-entry, no repeated sequence after recording, Exit), then requested dungeons
and actual quest completion. Full tests/vet and isolated database checks pass.
Accepted binary: bin/wireprobe-menu18.exe SHA256
f058b3d480c2ecda755ad94fd679466c9e200f5472b1be72ec3fe7c289fde490.
Dungeon15/16 requests are currently unhandled. Quest3145 still requires a
validated map76126 clear; equipment/reward delivery and battle are not complete.
