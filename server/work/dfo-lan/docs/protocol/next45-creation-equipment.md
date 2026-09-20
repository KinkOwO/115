# next45 Character Creation: Growth Branch + Source Create-Equipment

## Confirmed baseline (2026-09-20)

- The default launcher chain must hand the server a profession catalog that carries the growtype
  blocks. `channel_probe.py`'s tag degradation path (`_next37` → `_next34`) used to force
  `configs/characters.next25.json`, which predates growtype section parsing: it has no
  `advancement_growth` / `advancement_skills`. `Create` therefore skipped the advancement ledger
  (`len(prof.AdvancementGrowth[adv]) > 0`) and `automaticSkills` granted nothing, leaving the role
  at `advancement=0` while the client always renders the server value — the reported "created a
  branch character, entered the game as the base profession".
- That branch now points at `configs/characters.skycastle-release.json`. Both files are the same
  PVF snapshot (`checksum` / `file_count` / `size` identical), and all 17 professions are
  field-for-field identical outside the four growtype blocks (`advancement_growth`,
  `advancement_skills`, `awakening_skills`, `swordmaster_growth`), including `raw_sha256` — so no
  save migration is needed. The skill catalog stays `configs/skills.next27.json`: every
  growth-branch skill id is defined there and `[required level]` matches.
- A create request carries the growth slot in `option[8]` (0-based slot = `option[8]-1`, applied
  only under `Rules.AllJobsPilot` with a 12-byte option layout). Live 2026-09-20: a new
  Swordman-branch-1 role persisted `advancement=1`, `all_jobs_pilot=true`, `equipment_pending=false`.
- `[create equipment list]` is projected into `inventory.worn` at creation through
  `CreateEquipmentBySlot` (0-based slot, same domain as `Advancement`) plus the part → worn-slot
  map from `WearRules` (native evidence `1470cb2a0 current equipment type map initializer`).
  Live 2026-09-20 for that same role: worn slots `12/14/15/16/17/18` =
  `401040091 / 400070174 / 400170168 / 400120169 / 400220168 / 400270172`, durability taken from
  the equipment source. Every missing piece (no such slot, slot value 0, no slot mapping, no
  definition, no durability, part/label mismatch, not wearable by level/job/advancement, source
  checksum mismatch) is skipped without failing the creation.
- Per-profession weapons follow the source table and are the branch's signature weapon (Swordman
  slot 1 = beamsword). **No profession special case is applied.** A Swordman-only weapon override
  was tried and withdrawn: the root cause was the skill side (`光剑掌握` auto-learn level 1 versus
  learn level 15), which must be fixed in the skill data, not by the equipment projection.
- Old-format catalog files that only carry the raw cells are backfilled at load time by
  `ParseCreateEquipment`, so no re-export from the inner PVF is required.

## Still open

- The one-shot repair command for roles created before this fix (handover step 5) is not implemented.
- `server/package-manifest.json` (2026-09-12) does not list this batch's `*.release.json` files,
  including `characters.skycastle-release.json`; regenerate it before packaging for other machines.
