# next47 Character Job Advancement and Free Job Change (CMD1881 & CMD777)

## Confirmed baseline (2026-09-21, live acceptance passed)

- **Opcodes**:
  - CMD 1881 = `ENUM_CMDPACKET_CHANGE_GROW_TYPE` (`0x0759`): In-town character initial job advancement (base profession → advanced subclass).
  - CMD 777 = `ENUM_CMDPACKET_RE_GROWUP_CHANGE` (`0x0309`): In-town free job change (subclass → subclass).
  - Both belong to the same grow-type management family in DFO 115us client.
- **C2S Request Body**:
  - Native sender `0x143f2d110`: 14-byte payload carried on 8-byte aligned ciphers (CMD1881 cipher slot 5 skipjack, CMD777 cipher slot 7 blowfish).
  - Only byte 13 (`body[13]`) carries the target grow-type packed byte (`advancement | (awakening << 4)`); bytes 0..12 are uninitialized stack garbage and untrusted.
  - Live capture vectors:
    - Initial advancement to Dragon Knight: `2000000000000000c0f45f0000040000` (byte 13 = `0x04`).
    - Job change to branch 1: `2000000000000000c0f45f0000010000` (byte 13 = `0x01`).
    - Job change to branch 3: `2000000000000000c0f45f0000030000` (byte 13 = `0x03`).
- **S2C Response Body & Flow**:
  - Client response dispatcher `0x1459a2d70` handles CMD responses with standard `[result:u8][code:u16]` envelope. Success is `[01 00 00]`.
  - CMD 1881 response handler (`0x14524fd70`) reads grow-type from the local character object (`(*(*charObj+7656))(charObj)+12`) to refresh appearance visuals.
  - CMD 777 response handler (`0x1452aff30`) unconditionally triggers growtype manager refresh (`sub_144F609D0(mgr, 37, ...)`) and UI refresh.
  - S2C packet plan:
    1. `EntryBasicProbe` (kind 0, id 2): Synchronizes updated character basic info with packed `WireAdvancement()` byte to local character object.
    2. NOTI 718 `ENUM_NOTIPACKET_GROWTYPE_CHANGE_JOB_INFO` (`0x02CE`, 1 byte payload): Notifies grow-type manager (`qword_14E683D40+1036`) of the new grow-type byte.
    3. `appearanceRestore`: Rebuilds attributes, equipment, worn visuals, skills, variations.
    4. `EntrySkills` (kind 0, id 19): Delivers updated skill trees.
    5. Success ACK (kind 1, id = request opcode, body = `[01 00 00]`).
- **Persistence & Skills**:
  - State persisted via `CommitCharacterEvent` with unique key `change-grow-type-v1:<opcode>:<sha256(request)>`.
  - Updates `state.Advancement` and resets `state.Awakening = 0`.
  - Advancement skills are computed dynamically by `automaticSkills` and filtered by `knownSkills` from `AdvancementSkills[state.Advancement]`; no DB skill row mutations required.
- **Refusal Codes**:
  - Mapped to native DSTR strings: 7 (town only), 107 (advancement info error), 217 (selection error), 235 (cannot advance).

## Files

- `internal/game/protocol/advancement.go` — `DecodeChangeGrowType` request decoder and `ChangeGrowTypeSuccess` response encoder.
- `internal/game/protocol/advancement_test.go` — Unit tests for request decoding, padding validation, and edge cases.
- `internal/character/advancement.go` — `ApplyAdvancement` service method with catalog validation and awakening reset.
- `internal/character/advancement_test.go` — Unit tests for branch validation, awakening reset, and idempotency.
- `cmd/wireprobe/advancement_flow.go` — `changeGrowType` packet plan flow and native refusal code mapper.
- `cmd/wireprobe/main.go` — Dispatch hook for CMD 1881 and CMD 777.
- `cmd/wireprobe/request_scope.go` — Whitelist entries for 1881 and 777 in `observedGameRequest`.

## Live Acceptance (2026-09-21)

- **Confirmed in live play**:
  - Initial job advancement (CMD 1881) to Dragon Knight successful; character appearance, skills, and status updated immediately.
  - Subsequent free job change (CMD 777) between advanced subclasses successful; character switched cleanly without relogin.
