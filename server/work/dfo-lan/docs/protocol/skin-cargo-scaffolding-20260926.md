# Skin cargo registration scaffolding (CMD507 action 169)

Date: 2026-09-26
Status: spend + durable registration + damage-font cargo display + apply
(CMD1565 -> NOTI1546 category 6) implemented
(consumption live via the proven CMD507/NOTI14 path, display through NOTI1545 page
2); selection persistence and the other skin families still open

## Symptom

Damage-font consumables (e.g. `10305398`, `10358669`) could not be used. The
expiry-prompt half was fixed separately by the period-tagged stackable fallback
(see `dfo-item-period-expiry-fix`). After that fix the prompt was gone but the
item still did nothing on use.

## C2S truth (captured)

Using an `[add skin storage]` stackable sends **CMD507**, the shared "use
stackable" frame (59 or 64 bytes: slot `u16` at `p[0:2]`, action `u32` at
`p[7:11]`, all other bytes zero). Live vector:

```
47000000000000a9000...0   # 64 bytes, slot 71 (0x0047), action 169 (0xA9)
```

- action **54** = fatigue potion (existing path)
- action **169 (0xA9)** = `[add skin storage]` (damage font)

Before this change every CMD507 was routed to `recoverFatiguePotion` →
`DecodeFatigueAction`, which asserts action==54 and refused action 169 as
"unsupported stackable action" (`fatigue_potion_refused`, seen 33×). The item was
never consumed, so nothing was lost.

## PVF truth (skin key)

`cmd/skinstorageimport` scans `list/stackable.lst` for `[action type]
[add skin storage]` and exports `configs/skin-storage-items.json`
(schema `skin-storage-items-v2`, checksum-pinned to the running PVF). 1733
templates resolve to a skin; 127 name a skin id `list/skin.lst` does not define
and are exported under `missing_skins` instead of aborting the import.

| template   | skin_id | skin_path                            | skin_type   | damage_font_index |
| ---------- | ------- | ------------------------------------ | ----------- | ----------------- |
| `10305398` | 12      | `skin/damagefont/18_miku.skn`        | damage font | 11                |
| `10358669` | 59      | `skin/damagefont/23_childrenday.skn` | damage font | (absent)          |

The registered skin key is the `[action type]` parameter, which is the
`list/skin.lst` identifier — the same id the client's cargo pages and registry
are keyed by. `[damage font info][index]` is a different namespace (it selects a
`.dfk` row in `event/damagefontskin/damagefontskin.lst`) and is exported only as
context: only 73 of the 354 damage-font templates declare it at all, and the
index space is not the cargo's, so using it as the key would register the wrong
skin. The family comes from the skin's own `.skn [type]` label: damage font 354
templates (135 distinct skins), skill cutscene 593, instant emoticon 540, party
frame 224, airship effect 11, spray 9, weapon skin 2.

## Durable half + spend (implemented)

- `internal/game/protocol/stackable_action.go`: `DecodeStackableAction`,
  `DecodeAddSkinStorageAction`, `AddSkinStorageAction = 169`.
- `cmd/wireprobe/main.go`: CMD507 dispatch now splits by action; 169 →
  `useAddSkinStorage`, everything else → the unchanged fatigue path.
- `cmd/wireprobe/skin_storage_flow.go`: `useAddSkinStorage` resolves the
  template from the bag slot, looks it up in the catalog, **spends the item via
  `loot.Consume` (idempotent, key `consume:slot:template:0`)**, records the
  unlock, and pushes the absolute committed bag back as **NOTI 14
  (InventoryUpdate)** for the spent slot — the exact mechanism the live-verified
  fatigue path uses on this same CMD507 frame. No speculative CMD507 ack.
- `internal/storage/skin_cargo.go`: `account_skin_cargo` table
  (`PRIMARY KEY(account_id, source_template)`), `MigrateSkinCargo`,
  idempotent `UnlockSkin` (`ON CONFLICT DO NOTHING`), `ListSkins`. Account-shared
  like `account_material_storage`; save-compatible (new table, no change to
  existing rows).

Both durable operations are idempotent, so a replayed hotkey press neither
double-spends nor double-registers; the InventoryUpdate carries absolute state.

## IDA-reversed S2C layouts (true source, `DFO.exe.i64`)

Skin-cargo manager constructor `sub_1444E81C0` registers, via the NOTI registrar
`sub_14599D5D0(mgr, id, handler, 0)` / CMD registrar `sub_14599D450`:

| id   | name                          | handler        | reader verdict |
| ---- | ----------------------------- | -------------- | -------------- |
| 1545 | SKIN_CARGO_INFO               | `sub_1444ED900`| `u8 page` then delegates body to `sub_1444EFF40` |
| 1546 | SELECT_SKIN_LIST              | `sub_1444ED4F0`| `u8 page` then delegates body to `sub_1444EECA0` |
| 1547 | RECENT_ADD_SKIN_LIST          | `sub_1444ED400`| fully inline (below) |
| 1565 | SELECT_SKIN (cmd)             | `sub_1444E8D00`| equip/select UI, 88-byte block |
| 1671 | TAG_TOURNAMENT_SKIN_CARGO_INFO| `sub_1444EE1C0`| tournament variant |
| 1672 | TAG_TOURNAMENT_SELECT_SKIN_LIST|`sub_1444EDFD0`| tournament variant |

Reader helpers: `sub_146EA09F0(&b,1)`=u8, `sub_146EA1920(&b)`=u16 count,
`sub_146EA0BA0(&b)`=u32, `sub_146EA0BE0(&b,8)`=u64.

**NOTI1547 RECENT_ADD_SKIN_LIST** (self-contained, fully reversed):

```
u8  count
count × { u8 flag ; u32 value }   # stored as 8-byte vector entries {u32@0, u8@4}
```

`value`/`flag` semantics (skin index vs template; category) NOT yet confirmed —
the consumer of the manager vector at byte offset 1096 was not traced.

**NOTI1545 SKIN_CARGO_INFO** (full cargo push, via `sub_1444EFF40(mgr, page)`):

```
u8 page_selector          # 0..9 = that page only; 10 = all pages, read in the
                          #   order 0,1,2,3,4,5,7,8,9 (page 6 skipped in the broadcast)
per page section:
  u16 count1
  count1 × ( page==4 ? u32 : u64 )      # low u32 = skin id, high u32 = expiry
  u16 count2
  count2 × u64                          # low u32 = id remapped through the type
                                        #   table, high u32 = expiry; page==5 also
                                        #   calls sub_146A19E60
```

## Display half: which page the damage-font panel reads

The panel object lives at `window+9760` (`sub_1441C4080` case 2) and its class
vtable is `off_14A230DE8`. Slot 3 of that vtable, `sub_1441E3510`, refills the
grid:

```c
v5 = sub_1444EBDF0(qword_14E638F28, 2);          // snapshot of owned page 2
if (sub_1444EBAB0(id) && *(_DWORD*)(record + 8) == 2)  // static registry says page 2
    append to grid
```

`sub_1444EBAB0` is a lookup-only accessor (`sub_140283D60(qword_14E683BF8, id)`,
no insert), so the skin-id -> page registry is the client's own static table,
built from the same `list/skin.lst` this server reads: an id it does not know is
simply not rendered. Every other family's panel was traced the same way and asks
for exactly one page (1, 3, 4, 7, 8, 9), which is why only page 2 is fed.

The equipped font is NOTI1546 **category 6** (`sub_1444EECA0` case 6): one `u32`
id, validated with `sub_1444EBD90(mgr, 2, id)`, then applied through
`sub_142581F20`. Its "not owned" branch loads the sentinel `99999999`.

`sub_1444EFF40` **rebuilds** the addressed page (it resets the page map before
inserting), so every page frame must carry the complete list; a frame naming
only the newest skin deletes the rest. Its inserts set the entry's expiry flag,
and `sub_1444EC2C0` treats "flag set, expiry 0" as never lapsing, so permanent
grants need no clock domain. A nonzero expiry is compared against
`sub_145A11A50`, whose encoding is unconfirmed, so this server never writes one:
an item that carries its own expiry still registers the skin permanently, which
is recorded as an open gap rather than guessed at.

### Implemented (server side)

- `internal/game/protocol/skin_cargo.go`: `SkinCargoPage(page, ids)` builds
  `u8 page, u16 count, (u32 id, u32 0) ×, u16 0`; `SkinCargoDamageFontPage = 2`;
  page 4 refused (its first list has no expiry column).
- `cmd/wireprobe/skin_storage_flow.go`: `damageFontCargo` reads
  `account_skin_cargo` and encodes the whole page; `useAddSkinStorage` pushes
  NOTI1545 page 2 after a committed registration, next to the NOTI14 bag row. The
  spent-slot row now also carries the item's remaining expiry.
- `cmd/wireprobe/entry_flow.go` + `main.go`: the same page is pushed on every
  town entry as `skin_cargo_damage_font_restored`, before the category-0 profile
  frames. A failed read only drops this frame.

## Apply half: what the 应用 button needs back (live 2026-09-27)

The grid filled, so the display half closed. Clicking 应用 did nothing, and the
session log showed why: the client's request arrived and was never routed.

```
{"id": 1565, "kind": "client_frame", "bytes": 101, "plain_hex":
 02000000 00000000 0c000000 00...}          # damage-font tab, skin id 12
{"id": 1565, ... "plain_hex":
 01000000 00000000 a0860100 00...}          # 边框 tab, id 100000
```

- **C2S layout.** CMD1565 is a fixed 88-byte body, `u32 page, u32 result,
  u32 skin_ids[20]`. The client's own receive path proves the layout: registrar
  site `0x1444E85A1` maps opcode 1565 to `sub_1444E8D00(mgr, ok, u16 code)`, which
  on failure shows the text `sub_1444EC790(code)` and otherwise calls
  `sub_1444EE820`, and that function bulk-reads exactly 88 bytes through
  `sub_146EA0BE0` and switches on the first u32. The captured id `12` is the skin
  the server had just registered for template 10305398, so the third u32 is the
  skin id and the first is the cargo page (2 = damage font).
- **The client applied nothing on its own click.** Live: four presses on 应用 in
  one session changed nothing on screen, and the server had sent no frame for this
  feature. `sub_142581F20` was later enumerated in full (df25) and its callers are
  all server-driven — NOTI1546 `sub_1444EECA0` case 6, the 88-byte CMD1565 block
  `sub_1444EE820`, and the two panel handlers `sub_1441D21E0` / `sub_1441D23A0`,
  which read the selection vector back out of the manager rather than applying the
  click locally. `sub_1444E8ED0`, the only other function those readers share, is a
  set insert at `mgr+312`, not a sender.
- **Selection categories are not cargo pages.** `sub_1444EECA0` case 6 accepts its
  u32 only after `sub_1444EBD90(mgr, 2, id)` finds it in **owned page 2**, then
  applies it; an id the page does not hold resets the font to the built-in
  `99999999`. Case 2 runs the same page-2 membership check but applies through the
  heavier setter `sub_1447EF2F0`, which writes a different field and raises UI
  event 178 — so it is a different feature that happens to share the page. That
  makes the damage font page 2 in NOTI1545 and category 6 in NOTI1546, and the
  reply frame 5 bytes: `u8 6, u32 skin_id`.

### Implemented (apply half)

- `internal/game/protocol/skin_cargo.go`: `SkinSelectionDamageFont(id)` encodes
  NOTI1546 `u8 6, u32 id`; `DecodeSelectSkin` parses the 88-byte CMD1565 body and
  refuses any length other than 88 or any nonzero byte past the first id slot.
- `internal/storage/skin_selection.go`: `character_skin_selection
  (character_id, page, skin_key, updated_at)` with an upsert read back by
  `SelectedSkin`. Additive table only. The selection is per character because
  NOTI1546 is replayed per character, while the cargo that authorises it is per
  account.
- `cmd/wireprobe/skin_selection_flow.go` + the `frame.ID == 1565` dispatch: only
  page 2 is served; the id must be in the account's damage-font page or the click
  is logged and refused, because echoing an unowned id would unequip instead of
  apply. On success the row is persisted and NOTI1546 category 6 goes back.
- `entry_flow.go` + `main.go`: the persisted font is replayed directly after its
  own owned page, so the order is cargo → selection for each family.

## Switching and the dungeon half (round 2, live 2026-09-27)

The apply half landed and confirmed once, then two symptoms stayed open: a second
click on another font did not switch, and inside a dungeon the damage numbers kept
the default font. These turned out to be unrelated to each other and to the frame
layout — the 5-byte `u8 6, u32 id` reply is correct, and `sub_1444EECA0`'s tail
*assigns* the category vector, so the client has no "first click only" limit.

**Why switching stopped: the diagnostic gate was routing traffic.**
`cmd/wireprobe/request_scope.go` decides which plaintext to keep, and
`retainRequestBody` caps a not-yet-implemented command at `BodySampleLimit = 8`
bodies per connection. Opcode 1565 was never added to `observedGameRequest`, so
from the ninth click onward the frame was logged without `plain_hex`,
`verified` stayed false, and the dispatcher's `if !verified { continue }` answered
nothing. The client waited for a reply it never got — the same silence as before
the apply half existed, which is why the symptom read as "cannot switch". Fixed by
declaring 1565 an implemented command;
`TestSelectSkinRequestIsAlwaysDecrypted` fails if the opcode ever falls back under
the cap, since the sample budget and the routing gate were the same boolean.

**Why the dungeon kept the default font: the id is snapshotted, not read.**
`sub_1447EB510(obj, …, a8)` resolves a text object's font at creation — `if ( a8
<= -1 ) v91 = *(u32*)(sub_143C61150() + 232)` (df26,
`analysis/dumps/skin-noti/df26_holder_sub_1447EB510_*.c`) — and the damage-number
path `sub_1444EAD50` is exactly such a call (`a8 = -1`), reached from the skin-slot
record-pool dispatch method `sub_1441BDCD0` (round 3 corrected this: it is vtable
slot 9 of the window's `+9760` class `off_14A230DE8`, i.e. a pool walker, not the
battle damage render loop — see `analysis/dumps/CLIENT-MECHANICS.md` §12/§13).
`sub_143C61150()` is the lazy getter of the global 336-byte
avatar `qword_14E63AE60`, i.e. the object NOTI1546 category 6 already writes
(`*(avatar+232) = id`), so the town-side field *was* correct. The dungeon rebuilds
those text objects and never re-asks the warehouse, so state pushed at town entry
does not carry into a run. This is the same gap the worn-visual restore already
answers for equipment.

### Implemented (round 2)

- `cmd/wireprobe/request_scope.go`: 1565 is always decrypted and always reaches its
  handler.
- `cmd/wireprobe/skin_selection_flow.go`: `damageFontRestore()` re-pushes the
  absolute owned page (NOTI1545 page 2) followed by the applied selection (NOTI1546
  category 6) — the same cargo → selection order as town entry, and a font the
  account no longer holds is not replayed.
- `cmd/wireprobe/dungeon_flow.go`: that pair is appended to the plan in
  `finishDungeonLoading`, next to the creature list and the worn visuals. The new
  event kinds are `dungeon_skin_cargo_damage_font_restored` and
  `dungeon_skin_selection_damage_font_restored`; if neither appears in a run's log
  the read failed, and if both appear while the numbers stay default the client is
  discarding 1545/1546 inside a dungeon (untested — that is the evidence this round
  still needs from the live run).

## Round 3: expiry gate on GM-sent fonts, and the burst layer (2026-09-27)

The live run closed round 2's branch question: the pair of
`dungeon_skin_cargo_damage_font_restored` /
`dungeon_skin_selection_damage_font_restored` events appeared together **and the
in-dungeon damage numbers did change font**, so the client accepts 1545/1546 inside
a run. Two new symptoms came out of the same session.

**Symptom A — other damage-font consumables say 剩余期限已过 and cannot be used.**
No consume event exists in the log for them at all, unlike the round-1 items that
were spent with `remaining:0`. That places the failure entirely inside the client.
Per §11 of `CLIENT-MECHANICS.md`, byte offset 56 of the 181-byte item row is
**remaining seconds**, and the client's own gate treats `0` as expired: it renders
「剩余期限已过。」and refuses the use locally, sending no C2S. `[add skin storage]`
consumables declare no `[usable period]`, so server-side their row always carried 0.

Fix scope went through one deliberate change of direction. The user had approved
turning on `DFO_MAX_ITEM_PERIOD` from the launcher, but reading
`internal/game/protocol/item_period_override.go` before shipping showed that switch
rewrites *any* nonzero stored period to `MaxItemPeriod` — which would overturn the
already-confirmed pet baseline (7 days ⇒ panel 24856 days) — and makes
`StoredItemExpired` return false unconditionally, disabling expiry enforcement in
`internal/loot/consume.go:79`, `internal/loot/box.go:362/391`,
`cmd/wireprobe/lottery_item_flow.go:221` and `cmd/wireprobe/booster_flow.go:351`.
So the launcher default was reverted and replaced with a template-scoped override:
`protocol.ConfigureSkinStoragePeriods` installs the 1860 `[add skin storage]`
templates from `configs/skin-storage-items.json`, and `ItemPeriodForWire` lifts the
wire value to `MaxItemPeriod` **only** when the stored period is 0 *and* the template
is in that set. Real countdowns pass through untouched and expiry checks still bite.

**Symptom B — the `9999999` burst layer does not follow the applied font.**
Rounds df27–df31 (log `analysis/ida-work/dfNN.log`, dumps
`analysis/dumps/skin-noti/`) mapped the remaining state and are summarised in
`CLIENT-MECHANICS.md` §13; the two findings that change server behaviour:

- The holder's per-slot maps (`+120`, `+136`, written by `sub_1447EF3B0` /
  `sub_1447EF380`, i.e. by NOTI1672) have **no reader anywhere**. df31 decompiled
  all 338 callers of the map accessor `sub_1401C4620` and classified them by the
  constant offset of their first argument; only the two writers hit those offsets,
  and the one apparent extra hit indexes `+1064` in a larger class. ⇒ NOTI1672 must
  not be sent for damage fonts. (df30's earlier negative proof — intersecting
  "readers of `qword_14E63AE60`" with "users of the accessor" — was invalid, because
  member functions receive the holder as an argument; §9.18 records that.)
- The record pools (`sub_1441E5870` fills the damage-font one) store the concrete
  id at `record+40` and a category at `record+44`, where the category is `6` when
  `window+2184 == 1` and `2` otherwise, and selected-ness is
  `sub_1444EC5B0(mgr, category, id)` — membership in `mgr+296`. The dispatch method
  then branches on `record+40 == 99999999`: default builder (snapshots
  `holder+232`, follows the applied font) vs custom builder `sub_1444EAF10` (uses the
  record's own id, so it **never** follows). The burst layer is plausibly the
  custom-builder case, but which of the three `5×400` pools owns it, and what
  `window+2184` is at that moment, are unproven — and `holder+112` (the category-2
  applied value) has no reader outside its own setter. So sending NOTI1546 category 2
  would be a speculative frame; per hard constraint 3 nothing is emitted, and the
  gap is recorded with the observation needed to close it (does the blue layer ever
  change on any 应用 click, town vs dungeon).

### Implemented (round 3)

- `internal/game/protocol/item_period_override.go`: `ConfigureSkinStoragePeriods` +
  the zero-only branch in `ItemPeriodForWire`.
- `cmd/wireprobe/main.go`: installs the set from the skin-storage catalog next to
  the existing `ConfigureMaxItemPeriods` wiring.
- `scripts/launch_local.py`: the `DFO_MAX_ITEM_PERIOD` default added earlier in this
  session is removed again.
- `internal/game/protocol/item_period_override_test.go`:
  `TestSkinStoragePeriodsOnlyLiftZeroPeriodRows` pins all four directions (tagged
  zero lifts, nonzero preserved as `1`/`604800`, untagged stays 0,
  `StoredItemExpired` still true) and resets both global atomics via `t.Cleanup` so
  it cannot cross-contaminate `TestMaxItemPeriodCoversTaggedAndInstanceLimitedItems`.
- No database change, so no migration and no save impact. Candidate
  `bin/wireprobe-handoff-source.exe` rebuilt to SHA-256
  `930E0430E3F681CF627BEAADE98DC30A75E7532CC72FE061FBD7AE1C86093D65`;
  the previous `7229BF44…34B1` is 作废 (it predates this fix).

## Round 4: the panel has two tabs, and CMD1565 names one of them (live 2026-09-27)

Round 3's expiry fix confirmed on the live client (GM-sent fonts now consume into the
warehouse). The same run exposed the reason the second tab never lit up, and it was a
misread field, not a missing frame.

**The evidence is in one log line each.** From
`runtime/roles_persist_..._20260927_035034_350988_next37/events.jsonl`, the 88-byte
plaintexts were `02|00|12`, `06|00|12` and `06|00|ffe0f505`. Skin id 18 arrives under
**both** leading values, so that field cannot be "the cargo page holding the id" — it is
the selection category of the tab the click came from, and `99999999` is the 解除 button
naming the client's built-in font. Ten of the run's clicks carried 6 and were dropped as
`skin_selection_unsupported_page`.

**Why answering with a fixed category lights the wrong tab.** `sub_1444EE820` (the
receive path for this opcode) switches on the same case numbers NOTI1546 uses — case 2
through `sub_1447EF2F0` (`holder+112` plus UI event 178), case 6 through
`sub_142581F20` (`holder+232`) — and both then insert the value into
`mgr+296[category]`, which §13.2 of `CLIENT-MECHANICS.md` proves is exactly what
`sub_1444EC5B0(mgr, category, id)` reads to draw 生效中. Both tabs enumerate the same
owned page 2; only the record's `+44` category differs. So replying 6 to a category-2
click writes the *other* tab's vector: the tab that appeared to work was being lit by
clicks made in the tab that appeared broken.

**Unequip is asymmetric in the client, not in the server.** `sub_1444EECA0` case 6 has an
explicit else branch that writes `99999999` into `holder+232`, so a category-6 reset is
complete. Case 2's unowned path jumps to `LABEL_208`, which empties the selection vector
(marker disappears) but leaves `holder+112` at its old value. Recorded as a client-side
limit rather than worked around.

### Implemented (round 4)

- `protocol/skin_cargo.go`: `SelectSkinRequest.Page` → `Category`;
  `SkinSelectionDamageFontNormal` / `Cumulative` / `SkinSelectionDefaultFont`;
  `SkinSelectionDamageFont(category, id)` now returns an error for an unreversed
  category instead of encoding it.
- `cmd/wireprobe/skin_selection_flow.go`: `damageFontSelectionFrame` echoes the requested
  category, maps the `99999999` request to a stored 0 and answers with the sentinel
  (event `skin_selection_unequipped`), and still refuses an unowned real id.
- Entry and dungeon restore replay **both** tabs (`entry_flow.go` fields
  `SkinSelectionDamageFontNormal` / `Cumulative`, `damageFontRestore()` looping
  `damageFontSelectionCategories`), each after the owned page frame.
- `internal/storage/skin_selection.go`: no schema change — the `page` column already held
  this field, so existing rows stay valid; it now accepts `skin_key = 0` as an unequip and
  the comments state that it stores a category.
- Tests: `TestSkinSelectionDamageFontMatchesReader` (three byte layouts plus the refusal),
  `TestDecodeSelectSkinLiveVectors` (this run's four captured bodies),
  `TestDamageFontEntryOrder` (two selection frames adjacent after the page), and the
  isolated-schema storage test now asserts the two categories hold different fonts at the
  same time and that un-equipping one leaves the other.
- Candidate `bin/wireprobe-handoff-source.exe` SHA-256
  `2B732D787A2C982658E1EB82D419FC1DB79EBF315CF3EB619918ABE12D39C979`; `930E0430…3D65` is
  作废 (it predates the category echo).

## Round 5: the normal-damage tab's 解除 (live 2026-09-27)

Round 4's live result: both tabs now apply, and both fonts follow into the
dungeon — which also settles the round-4 prediction, the blue `9999999` layer
does read the category-2 chain. One defect remained: 解除 unloads the font on the
累计伤害 tab but not on the 普通伤害 tab.

Round 4 recorded that asymmetry as unfixable from the server (`sub_1444EECA0`
case 2's unowned path only empties the selection vector). That was wrong, and the
88-byte echo was left unimplemented because of a suspected re-send loop through
`sub_1444F1EE0` → `sub_1441ECDB0`. Both points closed from dumps already on disk
— no new IDA round:

| Claim | Evidence |
| --- | --- |
| Echo case 2 writes `holder+112` with **no ownership check** | `df25_caller_sub_1444EE820_0x1444ee820.c:102-107` — `sub_1447EF2F0(sub_143C61150(), v43[2])` |
| Echo stores `{v43[2]}` into `mgr+296[category]`, so no panel row keeps 生效中 | same file, `LABEL_37` (`sub_1401844D0(a1 + 296, …)` / `sub_143D855D0(v28 + 5, …)`) |
| The refresh rebuilds that layer from the stored id | `df13_cargo_render_tail_1444f1ee0.c` → `df12_page_select_1441ecdb0.c:140-143` → `df25_caller_sub_1441E0820_0x1441e0820.c:395-404` (`sub_1447EBED0(…, id, 0)` rebuilding `window+3680`) |
| **No loop** — nothing on the chain sends | `sub_1444E8ED0` is a map insert into `mgr+312`, not a sender (`df24_sender_0x1444e8ed0.c`); `sub_1441ECDB0`'s tail only erases from `window+3328..3336` and calls `sub_1441EC510` |
| **No loop** — the identical tail already runs live | the NOTI1546 handler ends in the same `sub_1444F1EE0(mgr, category)` (`handler_noti1546_select_skin_list_1444ed4f0.c:202`), and round 4 pushed one per tab per click with no re-send |
| The client dispatches an inbound 1565 at all | 1565 is registered in **both** the noti and cmd tables (§9.16, `df7-registrar-table.json`) |

Ordering is forced by the client: the echo's refresh reads `mgr+296[2]`, and
NOTI1546's `LABEL_208` empties it, so NOTI1546 must go **first** and the echo
second. Reversing them resets `holder+112` but leaves the stale font object up.
The cumulative tab keeps a single frame — its own case 6 else branch already
writes `99999999`.

Recorded as attempt 1/3 for the echo hypothesis.

### Implemented (round 5)

- `protocol/skin_cargo.go`: `SelectSkinEcho(category, id)` builds the 88-byte
  `u32 category, u32 0, u32 id, u32[19] 0` body and refuses an unreversed
  category. The result cell stays 0 because `sub_1444EE820` only reaches its
  switch when `v43[1] == 0`.
- `cmd/wireprobe/skin_selection_flow.go`: `damageFontSelectionFrame` now returns
  the frames, and `damageFontSelectionFrames` appends the echo **only** for
  category 2 with the `99999999` sentinel (events `skin_selection_damage_font_reset`
  then `skin_selection_normal_damage_reset_echo`).
- Tests: `TestSelectSkinEchoMatchesReader` (size, zero result cell, nothing set
  outside the first id slot, and that the request decoder reads the echo back) and
  `TestDamageFontResetFrames` (2 frames for the normal reset, 1 for every other
  case).
- No schema change, and the restore paths are untouched: `holder` is a process-wide
  global, so a session that starts un-equipped needs no reset frame.
- Candidate `bin/wireprobe-handoff-source.exe` SHA-256
  `5019F97502618F86A17E2B9D6F2F0A411B0A6959F66069371A2C67BF3E571ACA`;
  `2B732D78…39C979` is 作废 (it predates the category-2 reset echo).

## Round 6 — each tab names its **own** default id, and round 5's fix was inert

User: the paired frames never appeared; 累计伤害 解除 works, 普通伤害 解除 still does not.
The 05:28 run says why (`runtime/roles_persist_..._20260927_052804_605462_next37/events.jsonl`):
**seven** `skin_selection_refused` events with `category 2, skin_key 1` and not one with the
sentinel — the normal tab's 解除 button sends `u32 2, u32 0, u32 1`, so round 5's
`skinKey == 99999999` test never matched and the echo was never reached.

Where `1` comes from — the font holder's constructor, `df32_holder_ctor_sub_1447E41D0.c`
(holder is the 336-byte object behind `sub_143C61150()`):

| Field | Constructor default | What that tab's 解除 actually sends |
| --- | --- | --- |
| `holder+112` (what category 2 renders) | `*(_DWORD *)(a1 + 112) = 1;` (`:186`) | `1` |
| `holder+232` (what category 6 renders) | `*(_DWORD *)(v1 + 232) = -1;` (`:123`) | `99999999` |

⇒ There is no shared "unequip" sentinel: each tab asks for its own default by name. The
mechanism of the fix (NOTI1546 first, 88-byte echo second) is unchanged from round 5; only
the id both frames carry, and the recognition of a reset request, were wrong.

### Implemented (round 6)

- `protocol/skin_cargo.go`: `SkinSelectionNormalDamageDefaultFont = 1` plus
  `SkinSelectionResetID(category) (id, ok)`, which is the single place that maps a tab to
  its default; `SkinSelectionDefaultFont = 99999999` stays as the cumulative tab's value.
- `cmd/wireprobe/skin_selection_flow.go`: `damageFontSelectionFrame` looks the owned page
  up **first** and then treats the click as 解除 when `skinKey == SkinSelectionResetID(category)`
  **and that id is not owned**. The ownership test matters: a warehouse that really holds a
  font numbered 1 must keep 应用 semantics for it, otherwise the button would re-light the
  row it is meant to clear.
- `damageFontSelectionFrames` appends the echo for category 2 with **id 1** (was: the
  sentinel), so the replayed body is `u32 2, u32 0, u32 1` — byte-identical to what the
  client sent, which is what `sub_1447EF2F0` needs to put `holder+112` back to its ctor
  value. Stored key stays 0.
- Tests: `TestSkinSelectionResetIDsArePerTab` (both tabs, and that categories 0/1/3/4/5/7
  claim no reset id), `TestDamageFontResetFrames` now expects 2 frames for the normal reset
  with id 1, 1 for the cumulative reset, 1 for each apply, and 1 when the normal tab is
  handed the *other* tab's sentinel (the echo keys on the tab's own default, not on any
  reset-looking value); `TestDecodeSelectSkinLiveVectors` replays the `2, 0, 1` body too.
- No schema change; the `page` column still stores the request's category and 0 still means
  unequipped.
- Candidate `bin/wireprobe-handoff-source.exe` SHA-256
  `174A046BBBE598A50E899EFA08D476F9B2ACDA699F1675688F18B2B3891D1D4D`;
  `5019F975…571ACA` is 作废 (its category-2 reset branch was unreachable, so that build
  behaves exactly like round 4 on the normal tab).

## Round 7 — the echo was addressed to the wrong dispatcher (live 2026-09-27)

Round 6 landed: the 05:51 run has 11 × `skin_selection_unequipped` →
`skin_selection_damage_font_reset` → `skin_selection_normal_damage_reset_echo`, zero
refusals, and the user confirmed the 生效中 marker clears. But the panel's left preview
and the in-dungeon numbers kept the applied font, so the echo still did nothing.

Three read-only IDA rounds (`df33`, `df33b`, `df33c` — dumps in
`analysis/dumps/skin-noti/`, logs in `analysis/ida-work/`) closed why. Rounds 5/6 had
claimed "1565 is in both registrar tables, so a server-sent 1565 gets dispatched"
(§9.16). The conclusion holds but the criterion was wrong: what selects the table is
the **envelope's first byte**, which is exactly this server's `wire.ServerFrame(kind, …)`
argument.

| Claim | Evidence |
| --- | --- |
| The receive loop hands the packet to one router | `df33_caller_recv_loop_0x146d74a80.c` → `sub_1459A1BB0(router, a2, packet)` |
| The router branches on `*packet` (envelope byte 0): `0` → `sub_14599D380`, `1` → `sub_14599D200`, anything else → telemetry only, **no dispatch** | `df33_caller_dispatcher_0x1459a1bb0.c` (`v11 = *a3; if (v11) { if (v11==1) … } else …`) |
| The two registrars fill two **different** tables: CMD → `qword_14E6836F8`, NOTI → `qword_14E683700` | `df33_caller_reg_cmd_0x14599d450.c`, `df33_caller_reg_noti_0x14599d5d0.c` |
| Only the **flag==1** branch consults the CMD table: `sub_14599D200` → `sub_1459A2D70(qword_14E6836F8, id, status, code)` | `df33b_fn_branch_flag1_14599d200.c`, `df33c_fn_cmd_dispatch_1459a2d70.c` |
| The flag==0 branch goes to the notification bus `sub_1456B8AB0(id, packet)` — where 1565 is `sub_14399DD70`, an unrelated subsystem (reads 8 bytes, posts UI event 1636) | `df33b_fn_branch_flag0_14599d380.c`, `df33c_fn_noti_call_1456b8ab0.c`, `df7_reg_noti1565_14399dd70.c` |
| `sub_1459A2D70` invokes the found record as `(+8)(obj, status, code)` ⇒ matches `sub_1444E8D00(a1, a2, a3)` | `df33c_fn_cmd_dispatch_1459a2d70.c` (`LABEL_13`) |
| `a2` is a **u8 status byte the router consumes from the body before dispatching**; a u16 code is read only when status == 0 | `df33b_fn_branch_flag1_14599d200.c` (`sub_146EA09F0(v62,1)` / `LOWORD(v63)=0` / else `sub_146EA1920(&v63)`) |
| `sub_1444E8D00` with status 0 never reaches the skin code: `if (!a2) return sub_1444EC790(a3)` is a localized toast | `df7_reg_cmd1565_1444e8d00.c` |

This is the same envelope the already-live-confirmed kind-1 acks use
(`booster_open_ack` 160, `settlement_focus_ack` 72, `awakening_completed` 2177), and
`protocol.BoosterOpenSuccess` / `UnsealSuccess` already start their bodies with the
byte `1` — the convention was documented in this repo (`unseal.go:12`) but had never
been applied to the echo. So rounds 5 and 6 pushed a frame that the client routed to a
different subsystem: an inert frame, not a wrong value.

### Implemented (round 7)

- `protocol/skin_cargo.go`: `SelectSkinEcho` now returns **`u8 1` + the 88-byte payload**
  (89 bytes) and its comment names the three-hop chain it has to satisfy.
- `cmd/wireprobe/skin_selection_flow.go`: the echo packet is `kind 1`; the 1546
  selection stays `kind 0`. Ordering and the category-2 + id-1 gate are unchanged.
- Tests: `TestSelectSkinEchoMatchesReader` and `TestDamageFontResetFrames` now pin the
  status byte (non-zero), the 1+88 length, `Kind` per frame, and decode the payload from
  `p[1:]`. The first cut of `SelectSkinEcho` forgot `p[0] = 1` and both tests failed,
  which is the check doing its job.
- No storage or schema change; nothing else emits 1565.
- Attempt **3/3** for the echo hypothesis: the value (id 1) was proven in round 6, the
  address (kind 1 + status byte) is proven here. If the frame lands and the layer still
  does not revert, the remaining unknown is which pool renders it, and that is a
  client-side observation gap, not another frame to invent.

## Remaining evidence gaps

- **The burst (`9999999`) damage layer.** Round 4's prediction is now live-confirmed
  — driving the category-2 chain makes that layer follow — but *which* of the
  window's three `5×400` record pools (`+5384` / `+7536` / `+9912`) renders it, and
  what `window+2184` is when it does, are still unproven. No frame is invented for
  it; it works because the tab's own setter updates it.
- **NOTI1672's slot parameter** has no client-side consumer (§13.1), so per-slot
  font delivery is not available from the server at all.
- **Other pages' selection categories.** The 边框 tab sends page 1 with id 100000;
  which NOTI1546 category owns page 1 is unreversed, so that click stays
  unanswered rather than being answered with a guessed category.
- **The echo is emitted for one case only.** `sub_1444EE820` also has branches for
  categories 0, 1, 3, 4 (and a `v43[1] == 4` second-list path); none of them is
  needed by the damage-font tabs, so the server never sends them.
- **A warehouse that owns the default id stays ambiguous.** `id 1` on the normal tab is read
  as 解除 only while page 2 has no entry numbered 1; if an account really holds such a font,
  the click is served as an ordinary 应用 and the two-frame reset never fires. The live
  damage-font ids in this build are five-to-nine digits (`12`, `59`, `500000013`, …), so the
  collision is theoretical, but it is not excluded by evidence — and the safe direction is the
  one that never silently unequips an owned skin.
- **Why the ctor default is `1` and not `0`/`99999999` is unexplained.** `df32` dumped the
  constructor and the two stores; the enum that gives the normal tab's built-in font the id
  `1` was not located, so the value is taken from the store instruction plus the matching live
  request rather than from a named constant.
- **NOTI1547 RECENT_ADD_SKIN_LIST** `{u8 flag, u32 value}` semantics are still
  unconfirmed (consumer of the manager vector at offset 1096 untraced), so no
  "recently added" frame is emitted.
- **Timed skins** need the `sub_145A11A50` clock domain before the expiry column
  can carry anything but 0.
- **Other families** (skill cutscene, emoticon, party frame, spray, weapon skin,
  airship effect) register durably but are not rendered yet: their unlocks land in
  `account_skin_cargo`, and each needs its own registry page confirmed the way
  page 2 was.
- NOTI1231/CMD1282-style damage-font-specific opcodes were never found in the
  registrar; they are not needed, since the panel reads the generic pages.
