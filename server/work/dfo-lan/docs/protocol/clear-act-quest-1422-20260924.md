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

## attempt 1/3 live result: transport confirmed, clear scope rejected

- The CMD1422/CMD2278 handshake and client refresh succeeded, but the server cleared 12 quests by recursively treating newly unlocked successors as eligible. The next main quest therefore did not appear. The user rejected that scope: only quests already accepted when CMD1422 arrives may be cleared.
- Session log at `2026-09-24T13:06:57Z`: character 11 cleared 12 quests; the client then sent CMD2278 with an empty plaintext body. The server emitted the updated NOTI291 active triggers, NOTI342 completion bitmap, and NOTI21 available quest list.
- Candidate executable SHA-256: `388EEFFDC5033368F1A83D2FAC40ED1B7BBF10448A4EF22A40E512EA172825A0`.
- `go test ./...` and `go vet ./...` passed. No database schema change was required.

## Confirmed baseline — attempt 2/3

- The request still decodes only the observed 8-byte zero body. It carries no quest ID, level, or bulk-clear flag; the server uses the character's persisted `accepted` rows as the boundary.
- `ActClearPlan` selects only accepted quests whose source grade is `[epic]`. It does not walk prerequisites or synthesize newly available successors.
- The storage transaction is update-only and requires every selected row to still be accepted with the current source checksum. It cannot insert an unaccepted quest. New completions use `progress_model='act-clear-v2'`.
- Startup migration repairs attempt 1 rows conservatively: only `act-clear-v1` completions whose `accepted_at` exactly equals `completed_at` are synthetic inserts. Their full row is copied into `character_quest_repairs` before deletion. Previously accepted quests have an earlier `accepted_at` and remain completed.
- Static tests require an accepted quest to be selected, a completed quest to stay excluded, an empty accepted set to produce no IDs, and stale source state to be refused. `go test ./...` and `go vet ./...` pass. Confirmed candidate executable SHA-256: `D99B8A288DEDE963F6E54648EC93CF42769E1D26A340D5367A9D2FBFF3A84AE3`.

### attempt 2 live result: accepted-only scope confirmed, next quest blocked elsewhere

- At `2026-09-24T14:12:24Z`, character 11 accepted quest 3331; CMD1422 cleared exactly one row. The refresh exposed quest 3513, which was accepted at `14:12:42Z`; the second CMD1422 again cleared exactly one row. No unaccepted quest was inserted or completed.
- The character is level 55. Source quest 3281 is the level-50 `[epic]` successor of 3513 and is valid for `[all]` jobs, but its objective is the three-cell `[reach the range]` form `[39,250,250]`. The current strict objective decoder supports six-cell ranges and the separately evidenced quest-3252 form only, so 3281 is classified unimplemented and omitted from NOTI21.
- Therefore the one-at-a-time clear scope behaves as requested and is confirmed. The absence of the next Act quest is an independent availability gap at quest 3281. Quest 3522 has a level-56 gate and does not explain the missing level-50 successor.
