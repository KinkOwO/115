# Current-build entry USERINFO reconstruction

Client 2.38.2.34, SHA256 fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c.
NOTI type 0 / ID 2, distinct from C2S command 8 and roster mode 2.

Mode 0 header: u8 mode=0, u16 count, then each row has two u8 context values.
Reader 145637a20 sets these context values, then calls 14563ec60. Context values
must agree with the client's channel context for a local actor. Zero is a probe
assumption for the current loopback channel, not a universal channel ID.

Minimum row reader sequence, with absent optional collections:

| Native location | Wire fields |
| --- | --- |
| 14563ecd2 | raw160; **advances** the packet cursor |
| 14563ed31 / 14563ee88 | u16 actor ID, u32 UTF-8 length + name |
| 14563eeb1..14563f0d1 | u8 profession, packed advancement, level, PvP grade, state |
| 145639840 / 14563f0f1 | u8 equipment count=0, u32 |
| 14563f10f..14563f161 / 145639b70 | 4*u8, u8 cosmetic count=0 |
| 14563f174..14563f276 | 2*u32, u8, u32, u8 |
| 1456394b0 | u32, u32 string length=0, u8 |
| 14563be50 / 14563f2a3..f2bc | u8 premium, 2*u32 |
| 14563a0b0 / 14563f2de | u32 string length=0, u32, u16, u32 |
| 14563f338..f36c / 145639dc0 | 3*u8, u8, u32 |
| 14563f3db..f430 / 14563a240 | u8, u32, u32, u16, u32, raw8 |
| 14563fc24..1456401d4 | u8, u32, u8, u16, u8 |
| 14563bdc0..145640351 | u16, u8, u16, u8, 3*u8 |
| 1456407ca / 14563c140 | u32, u32 collection count=0 |
| 1456407f1..145640965 | 6*u8, 3*u32 |

The one-row notification with these empty collections is 307 + UTF-8 name bytes.

The fixed160 structure contains inline NUL-terminated narrow strings at +0x1b
and +0x5f. 146e90b00 converts these via the Windows narrow-to-wide routine; they
are not std::string pointers. Other members were mapped to local object offsets
in `../dfo_probe_tools/userinfo_prefix_refs.asm`; most semantics remain unknown.

The serializer uses empty unknown data and native initial flag values only as a
bounded parser experiment. It does not establish official defaults, inventory,
HP/MP, a loaded map, or playable town entry. Equipment is explicitly rejected
until its initial data mapping is complete. Preserve the existing character DB.

Mode 1 requires the mode 0 actor to exist. It starts with raw250 and u16 actor
ID, then u64 experience (helper 145639c20 -> 146d77f90), a u32-sized packed
structure (constructor 147554f30 initializes 91 bytes), u8, switching inventory,
and skills/other collections. Do not use the older x86 mode 1 payload.

## Historical mode 1 experiment (live10 failed; superseded below)

entry_addition.go is an unintegrated experimental codec: minimum one-actor payload 495 bytes plus three bytes per skill, with TWO skill banks. Native 14563d9aa increments the bank counter, compares against 2 and loops to 14563d794. Both self/other branches consume both banks. Each bank has a byte count, u16 ID/u8 level skill rows, three fixed u16/u32 pairs and five fixed u16/u32/u8 triples.

Actor ID begins at byte255, experience257, packed length265 and the 91-byte record269. Native147555f80 expands the packed record into40 protected fields. packed_stats_oracle.py executes the original conversion and protection/getter routines in private emulated memory with initialized TLS epoch. Three vectors cover all40 fields including signed and floating-point values; fixture internal/game/protocol/testdata/native_packed91.json. Codec tests reconstruct original bytes and check both skill banks.

Packed wire widths: HP/MP u32; four u16 core values; four i16 elemental values; nineteen i16 status values; i32; two i16; u32; two u16; two i16; i32; u8 base percentage; float32. Native finalization145cce150 scales HP/MP through divisor10 and core fields by base percentage divided by1000. Other semantics and source PVF skill tuple mappings still need verification. Unknown prefixes and collection values remain experimental, not official defaults.

This codec is NOT called by the active gateway. Full mode1 was not required for the accepted town visual gate; complete stats, skills and inventory remain missing. go test ./... and go vet ./... passed after adding the codec and native fixtures.

## Combined correction, live_detail_14 (2026-09-11 UTC)

The historical 495-byte minimum above was incomplete. The call at14563d6cb
enters1452c1540, which ALWAYS reads an equipment block: u8 row count,
u32 scalar, u8 collection count and raw8 flags, even when both counts are zero.
Those14 bytes were missing between the byte atoffset360 and switching inventory.
The corrected minimum is509 bytes. A fixture with three skills is518 bytes.

`../dfo_probe_tools/addition_cursor_oracle.py` runs the original14563d400
packet-reading branches and nested parsers under Unicorn, with unrelated
business/UI calls stubbed and private dummy actors. It consumes the entire518
byte fixture. Removing the14 bytes reproduces a native read overrun at14563dbf8
(503+4 >504). The Go serializer must match this independent fixture exactly.
This proves wire cursor consumption, not live game/UI acceptance.

The opt-in detail build restores source base attributes from saved character
state. Slayer source includes HP520, MP480, movement850; native units are
documented above. Optional equipment, skills, rewards and unknown fixed-prefix
fields remain incomplete. No original EXE or PVF change was made for this fix.
Live14 includes the corrected mode1, vault initialization, USER_AREA23,
AREA_USERS24 and final FATIGUE36. Client verification is pending.
