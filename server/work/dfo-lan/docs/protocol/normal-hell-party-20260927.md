# Normal Hell Party investigation (2026-09-27)

Scope: CMD16 `Mode=1` for ordinary dungeon 103 (Trombe Power Plant). This is distinct from Boundary of Attunement.

Captured client plaintext: `670000000300000100ffff0000000000ff180000000000000000000000000000`.
Decoded: ID 103, difficulty 3, extra 0, mode 1, flag 0, party 65535, reserved 0, tail 0, quest 6399, options 0/0, event 0.

The current `dungeon.Select` rejects this solely because `r.Mode != 0`. The 103 source script has `[hell dungeon] 1`, seal map 60069 at (3,0), seasonal seal map 91007 at (1,0), and a seal appearance rate of 10. Neither seal map is present in the current exported map catalog. `DungeonInfo` currently always sends 255/255 as the random Hell XY.

## Current-client evidence

- The native CMD16 sender `sub_146D495E0` writes the byte at CMD16 offset 7 from its selection-mode state (`a1+1444`), matching the captured `Mode=1`.
- The NOTI28 reader `sub_1452AC840` reads the two Hell coordinates after the boss coordinates; existing native oracle `dungeon_map_oracle.py` verifies the default `255,255` absent sentinel. Applying the source seal coordinate to this field for deliberate Hell selection is the current test hypothesis, not yet live-confirmed.
- NOTI666 handler `sub_1452AFF70` reads a u32 count followed by u32/u32 pairs and stores each pair with `sub_145B4BA70`. NOTI777 handler `sub_1452AFF30` refreshes the Hell Party UI state; neither message's emission timing is yet established.
- The current PVF's `list/map.lst` maps 60069 to `map/trombe/hell_14006(3,0)n.map`. Its `[special passive object]` contains a `[hellparty]` group. The DGN marks the seal position `(3,0)`, which lies outside quest 6399's 3x1 maze; the selected quest route is a key live-test risk.
- The runtime client's `Script.pvf` has the same SHA-256 as the frozen project client. Its `DFO.exe` differs from the frozen binary in only four bytes at two adjacent offsets, so the authoritative IDB's analyzed code regions remain identical.

## Attempt 1/3: source seal room and NOTI28 coordinate

This candidate accepts `Mode=1` only for the captured Trombe dungeon ID 103 when the DGN marks Hell Party and its seal map is imported. It keeps normal quest/difficulty checks, uses the current source's seal map and position, and sends that position in NOTI28. `Mode=0` is unchanged. The source-matched map overlay adds 55 missing map scripts without changing the catalog checksum or database schema; other dungeons remain gated pending native validation. One referenced map, 100016811, is absent from the current `list/map.lst`.

The candidate intentionally has no invitation payment or Hell-specific reward code yet. It must only be used to observe the native entry/room path; a successful map load does not establish complete Hell Party gameplay. No DLL is involved.

Validation before live test: captured CMD16 vector test, `go test ./...`, `go vet ./...`, and candidate build pass. Narrowed candidate (`wireprobe-hellparty-probe-v2.exe`) SHA-256: `3CB7049B05FFFEBFB4E2C2ACEFB521BD841A2FF2B46F53256A8E5AC6C1A65A5E`. The launcher profile uses the isolated binary and does not replace the archived 39 executable.
