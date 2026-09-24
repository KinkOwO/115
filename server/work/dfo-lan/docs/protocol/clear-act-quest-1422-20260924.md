# Clear Act Quests / CMD1422 — 2026-09-24

## Evidence

- User's live C2S frame: opcode 1422 (`ENUM_CMDPACKET_CLEAR_QUEST_TICKET`), checksum valid, decrypted body `0000000000000000` (8 bytes), from the confirmation dialog's OK action.
- Current client's IDB sender `sub_144F5D810` writes CMD1422 and a zero DWORD. `sub_144F69180` confirms the request family and sets UI mode 1 for the no-target form.
- IDB response registration `sub_1452A1F90` maps CMD1422 to `sub_14526E8C0`. The success branch calls `sub_144F44BF0`, which sets the quest-clear UI flag, sends CMD2278 (`REQUEST_QUEST_AUTO_SKIP`) without a body, and closes the dialog. The failure branch closes without the refresh request. The CMD1422 reader does not consume any extra response fields.
- Current quest PVF classifies Act story quests with `[grade] [epic]`; the server's imported quest catalog also provides minimum level, job/growth restrictions, and prerequisite groups.

## Server candidate — attempt 1/3

- Decode only the observed zero 8-byte CMD1422 form. After an owned-character and checksum check, follow reachable `[epic]` prerequisite chains whose minimum level is at most the character's current level. Include accepted epic quests, retain completed rows, and leave other grades alone.
- Complete the selected rows in one PostgreSQL transaction with `progress_model='act-clear-v1'`; no EXP, gold, items, or database schema change. A source-version mismatch aborts the transaction.
- Send CMD1422 success flag, then answer the native follow-up CMD2278 with updated NOTI291 active triggers, NOTI342 completion bitmap, and NOTI21 available quest list.
- A level-50 source fixture plans 98 epic quests. `go test ./...` and `go vet ./...` passed with an isolated Go build cache.

## Confirmed live acceptance

- User confirmed the Clear Act Quests action succeeded.
- Session log at `2026-09-24T13:06:57Z`: character 11 cleared 12 quests; the client then sent CMD2278 with an empty plaintext body. The server emitted the updated NOTI291 active triggers, NOTI342 completion bitmap, and NOTI21 available quest list.
- Candidate executable SHA-256: `388EEFFDC5033368F1A83D2FAC40ED1B7BBF10448A4EF22A40E512EA172825A0`.
- `go test ./...` and `go vet ./...` passed. No database schema change was required.
