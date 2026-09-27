# Quest NPCs in town phase maps (2026-09-26)

Status: user live confirmed on quest 13595 after the clear/hide timing correction. Attempt 1/3 for phase-map NPC CMD33 authorization was insufficient by itself; the confirmed successor is recorded in `empress-erje-quest-13595-current-clear-hide-20260926.md`.

## Why this path exists

Quest 13588 is the confirmed example: its NPC Woon is absent from Ghent 6/3's base map, but present at `(845, 180)` in the area's afterwar `[phase]` map. Quest 13587's clear/show visibility rule lists Woon second in one `[npc]` group. The user confirmed that the narrow fix allowed quest 13588 to complete and 13589 to become available; see `snake-in-the-hole-quest-13588-ghent-phase-npc-20260926.md`.

## General rule

- The world importer reads each town area's source `[phase]` map, including imported map scripts, and exports each well-formed `[NPC]` row with its map path, SHA-256 and position. The current 115 PVF has 62 phase-bearing areas; 60 contain 775 NPC rows. The PVF SHA-256 matches the existing world catalog source checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`.
- A native CMD33 meeting request can use a phase-map NPC only in that same town/area and only when the accepted quest's objective and completion NPC match a source accept/show rule or a visible prerequisite chain. The established `MeetNPC` state and version checks still apply. A phase row alone never authorizes a quest.
- The earlier quest-13588 area/ID exception is removed. Existing special cases for NPCs without a source phase-map placement remain as previously confirmed.
- `worldcatalog -base` refreshes only phase NPC rows in an existing catalog; it preserves the existing source metadata and unrelated map data. The generated catalog gained only `phase_npcs` fields. Database schema, save data, packet layout, client files and DLLs are unchanged.

## Verification and limit

The source-catalog test verifies Woon's afterwar map identity and position, grouped visibility, same-area authorization, refusal for another area or an unrelated quest, and the later Woon meeting quest 13590. `go test ./...` and `go vet ./...` passed. The isolated candidate is `bin/wireprobe-quest-phase-npc-general-20260926.exe` (SHA-256 `D957A1A52D6B245DC5A40CBAD1E59A07D4D6ADE95FA4387A100B1F2F09BE69AB`).

The user's first follow-up in Empyrean's Palace found quest 13595 still empty. The source and live CMD33 showed that the candidate wrongly treated quest 13595's own clear/hide rule as active before the quest was cleared. The corrected candidate passed full tests, then the user confirmed 13595 completed; the manual event trace also records quest 650 completion in the same session.

After the live server process exited, the same candidate bytes were staged at `bin/wireprobe-handoff-source.exe` for the next user-operated `启动游戏.cmd --source-build` run. The user-confirmed narrow candidate was copied to `bin/wireprobe-handoff-source-confirmed-13588-20260926.exe` first; local binaries are not committed. For future phase maps whose `[NPC]` and quest visibility data are both in this PVF, this path avoids another quest-specific exception. Cases lacking source visibility or phase-map placement still require direct evidence before broadening the rule.
