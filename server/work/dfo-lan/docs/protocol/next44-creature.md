# next44 Creature / Pet Protocol Evidence

## Confirmed baseline (2026-09-20)

- A character with an equipped creature in persisted worn slot 26 can enter the game when `NOTI 2 mode 1 / DetailedEquipment` contains avatar slots 0..11 only.
- Encoding slot 26 with the ordinary avatar-row projection caused the client to remain at character selection. That experiment is withdrawn and must not be restored without a complete reader/field/timing proof.
- `internal/character/detail_avatar_test.go` includes an equipped slot-26 fixture and locks the avatar-only mode-1 output length.
- The original defects remain open: the equipped creature slot is empty in the creature UI, and no creature world model follows the player.

## Current candidate packets (not live-confirmed)

- NOTI 105: creature list projection.
- NOTI 102: candidate 10-byte creature growth projection.
- NOTI 100: candidate creature scene activation projection.

These packet names and payload builders are investigation inputs only. Completion requires closing the native client reader, object identity/key mapping, scene readiness, and live packet ordering for both town and dungeon.
