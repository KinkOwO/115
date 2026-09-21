# next46 Magic-Seal Unsealing (CMD393)

## Confirmed baseline (2026-09-21, code-complete; live acceptance pending)

- **Opcode**: CMD 393 = `ENUM_CMDPACKET_UNSEAL_RANDOM_OPTION` (`analysis/dumps/opcode_table_detailed.json`).
  Right-clicking magic-sealed equipment sends it; before this handler the request met silence and
  the item could never be unsealed.
- **C2S body** (IDA sender `sub_145AE9AE0` + live capture 2026-09-21T18:53): `u16 target_slot`,
  `u16 scroll_slot` (`0xFFFF` = no scroll), then zero cipher padding. Live vector for slot 10:
  `0a00ffff00000000`. The scroll-assisted path (`sub_1451AAA80`, dstr 44327 "There is no Unseal
  Scroll") sets `[mgr+0xE8]` to the scroll slot; scroll unsealing is not proven and is refused
  with code 13.
- **S2C body**: the client's generic command reader (`sub_1459A1BB0` → `sub_14599D200`) consumes
  exactly one status byte on success and a `u16` error code when the status is 0. No 393-specific
  response handler is registered anywhere (both packet registrars and the UI map registrar were
  scanned exhaustively), so success is the bare byte `01`; refusal is `00` + code, the same
  `protocol.Refusal` shape other commands use. Codes follow the reference implementation's live
  mapping: 4 invalid target, 10 insufficient gold, 13 unsupported.
- **Sealed presentation**: the sealed items in the live bag (slots 10/11/13 = items 100310840 /
  100310581 / 100320649) carry **byte13=0 with an all-zero option block (record[60..75])**; the
  client still offers unseal because their `.equ` definitions flag `[random option] 1`
  (rarity 2, level 15, groups amulet/wrist/ring). A successful unseal therefore **fills the
  option block and keeps byte13=0** — the same post-unseal state the reference implementation
  proves. NOTI14 single-row update (`space 0, count 1, 181B record`) refreshes the client.
- **Roll rules** come from the client's own PVF, exported by `cmd/randomoptionimport` into
  `configs/randomoption.current37.json` from the seven authoritative tables under
  `etc/randomoption/`: `optionnumbering.etc` (value ratios), `optionquantity.etc` (quantity
  weights + per-type quantity bounds), `optiongrouping.etc` (option pools), 
  `optiongroupselection.etc` (per rarity/group/quantity group choice), 
  `randomizedoptionoverall1.etc` `[option type]` weights, `randomizedoptionoverall2.etc`
  `[different weight]` + `[break seal cost]`. Counts match the reference catalog exactly
  (ratios 4, quantity weights 2, quantities 246, groups 17, choices 438, type weights 82,
  different 6, costs 246).
- **Coverage audit**: all **3795** `[random option]==1` equipment rows in the current PVF have an
  exact `(rarity, level)` option-type row and a `(rarity, group, quantity)` group choice — zero
  gaps, so lookups are exact-match and miss = catalog drift (refused loudly, never guessed).
- **Break-seal cost is 0** for every current table row (rarity 2/3, all levels, parts 1..3);
  unsealing charges no gold and no NOTI519 balance push is sent. The cost lookup and gold charge
  are implemented for a future nonzero table.
- **Durability of the roll**: the full post-unseal 181-byte record is stored on the bag row
  (`BagEquipment.Record`), so options survive relogin; the unseal commits through
  `CommitCharacterEvent` with the sealed pre-image record hash in the event key, so a retried
  right-click replays the original roll and a stale replay that no longer matches the bag fails
  loudly instead of acknowledging a phantom unseal.
- **Container-hash note**: the current `runtime/pvf_source/Script.inner.pvf` hashes `be95d64e…`
  while the production catalog fleet pins `7ef2db59…`. A full content audit (all 3795 flagged
  `.equ` raw bytes + the seven tables) proved the two archives content-identical for the unseal
  input set; the config therefore pins the fleet baseline `7ef2db59…` and keeps the true archive
  hash in `exported_from_checksum`.

## Files

- `cmd/randomoptionimport/main.go` — PVF → `configs/randomoption.current37.json` exporter
  (`-baseline` pins the fleet checksum).
- `internal/inventory/randomoption.go` — catalog loader, roll algorithm, `Bag.UnsealRandomOption`.
- `internal/inventory/unseal.go` — `UnsealService` character-event transaction.
- `internal/game/protocol/unseal.go` — C2S393 decode, success/refusal bodies.
- `cmd/wireprobe/unseal_flow.go`, `main.go`, `request_scope.go` — dispatch, refusal-code mapping,
  observed-request registration.
- `server/work/dfo_probe_tools/channel_probe.py` — passes `-random-option-catalog` with an
  absolute path (the gateway's cwd is the project root, so relative defaults never resolve).

## Live Acceptance (2026-09-21)

- **Confirmed in live play**: Magic-seal equipment right-click unseals successfully with options rolled and bound.
- Equipped onto character cleanly; tooltip displays random option stats; attributes take effect.
- Survived relogin cleanly with random option block persisted.
- Unmasked and resolved creature slot 26 collision in `wornAppearance`.

## Still open

- Sealed drops are NOT generated by this change: drops/rewards keep zero-extension records.
  Whether eligible gear should drop sealed (the reference drops `reward_slot==2` gear sealed) is
  a separate design decision.
- Scroll-assisted unsealing (scroll slot != 0xFFFF) is refused with code 13 until the scroll
  path is proven.
- The other random-option opcodes (399 regeneration, 429 change, 446 insert, 447 reset) remain
  unimplemented.
- Sealed drops are NOT generated by this change: drops/rewards keep zero-extension records.
  Whether eligible gear should drop sealed (the reference drops `reward_slot==2` gear sealed) is
  a separate design decision.
- Scroll-assisted unsealing (scroll slot != 0xFFFF) is refused with code 13 until the scroll
  path is proven.
- The other random-option opcodes (399 regeneration, 429 change, 446 insert, 447 reset) remain
  unimplemented.
