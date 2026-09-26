# Final Defense Line quest town entry — attempt 1/3

## Observed

- Player selected the quest book route for “Final Defense Line” and confirmed the notice for “Ep. Last Defense Line.” The supplied live CMD36 body was `37000000000000005e035901052600000000000000000000`: destination 55/0 at (862,345), flag 5, source 38/0, tail flags 0/0.
- The server recorded `area_refused town=55 area=0 reason="no authorized source portal to destination"` at 2026-09-26 17:13:37 UTC.
- The exported source defines 55/0 as `fiendwar_main_town_epic.map`, requires level 90 and quest 8645, and places NPC 619 at (862,315). Quest 8646 targets NPC 619 and lists quest 8645 as its prerequisite in its source cells. The requested landing is within 55/0's walkable geometry.
- Source 38/0 has seven ordinary outgoing portals; none targets 55/0. CMD36 has neither a map teleport tail nor a preceding special warp state in the supplied evidence. The generic portal transition therefore rejects it before destination geometry validation.

## Candidate

Authorize this exact 38/0 → 55/0 CMD36 form only for the same character with persisted quest 8645 completed and quest 8646 accepted. Keep the existing destination level and geometry checks. No packet layout or client code changes.

## Confirmed baseline

This is attempt 1/3 for the quest book route. `go test ./...` and `go vet ./...` passed with a project-local Go cache. The user confirmed successful live entry through the same quest book route. Source candidate SHA-256: `33954DBDA9BCD3978BC410CEEB35ED12BBE0C825CB8BC19D5A8B49A7A32C8A50`. No database schema, client, or DLL changes.

## Exit to Elvenguard — attempt 2/3, live confirmed

- After the confirmed entry, the user selected the in-town exit notice for Elvenguard. The captured CMD36 body was `26000000000000002c08dc00053700000000000000000000`: destination 38/0 at (2092,220), flag 5, source 55/0, tail flags 0/0. The server refused it with `no authorized source portal to destination` at 2026-09-26 17:31:15 UTC.
- The 55/0 source map lists only an ordinary portal to 55/1. The requested Elvenguard landing lies inside the 38/0 walkable area. The visible exit notice and the native CMD36 identify a separate town return route.
- Candidate: authorize only 55/0 → 38/0 with the captured request form for an owned town character, including characters already saved in 55/0. Validate the saved source position and destination level/geometry through the existing services. No quest status is required on exit because the player may have completed the quest inside the episode town.
- `go test ./...` and `go vet ./...` passed. Updated source candidate `bin/wireprobe-handoff-source.exe` SHA-256: `AB8F2A1B947218B0CFE6D06DB071E47039BE520854B934CAC47257AE24F19FA4`.
- The user later confirmed the town transfer succeeds with the candidate. This confirms the exit as well as the earlier quest-book entry.

## Source-driven NPC move and episode return — attempt 3/3, live confirmed

- The source `list/npc.lst` maps NPC 618 to `npc/618_fiendwar_upgrade.npc` and NPC 619 to `npc/619_fiendwar_upgrade.npc`. Their `[role] [move town]` targets are 619 and 618 respectively. The first role has `[npc role data] [move town] [in progress quest]` 8646–8650; the reverse role is unguarded. World map/import placements locate 618 in 38/0 and 619 in 55/0. This closes the exact live CMD36 entry and exit edges without numeric route exceptions.
- The source town script `town/180621_cosmofiend_epic.twn` declares `[episode town] [return npc] 618`; the same tags occur in towns 75, 82 and 149. Their return NPCs have unique world placements in 22/4, 22/4 and 6/2 respectively. The server resolves these from source tokens, so saved characters already inside a special town can take its quick return without a newly stored origin stamp.
- `cmd/npcteleportimport` extracts direct `[role] [move town]` links and known `[in progress quest]` role guards from the same inner PVF as the other generated catalogs. Its 236 relations are stored in `configs/npc-teleport.generated.json` with the source checksum and script SHA-256. The runtime authorizes an NPC edge only when the source NPC exists in the current area, the target NPC has one resolved area, any listed quest is accepted with matching catalog version, and its source prerequisite group is complete. Role data with a move but no recognized in-progress guard is withheld. Target NPCs absent from the current world index or present in multiple areas remain unresolved rather than opening arbitrary destinations.
- Both the NPC route and quick return still require an owned town character, matching previous town/area, native observed Flag 5/tail 0/0, and existing source/destination level and walkable checks. There is no database schema change and no client patch.
- `go test ./...` and `go vet ./...` passed. Candidate `bin/wireprobe-handoff-source.exe` SHA-256: `A1AFABDE8C5FEC2D34F69D050228DB139AB1089656A2530B7A2B9A311BACB9FF`.
- The user confirmed that the candidate transfers successfully. The observed entry and return routes are now driven by `.npc`/`.twn` rules; other data rows with missing or ambiguous NPC placements remain outside the confirmed scope pending additional evidence. This is the third runtime-path hypothesis for this function under the project's C2S limit.
