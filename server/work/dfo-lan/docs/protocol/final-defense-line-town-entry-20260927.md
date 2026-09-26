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
