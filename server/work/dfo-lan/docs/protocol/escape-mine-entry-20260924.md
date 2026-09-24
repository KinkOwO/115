# Escape the Mine entry, quest 3354

## Evidence

- The client sent dungeon selection CMD16 for dungeon 53 and quest 3354 at 2026-09-24 11:57:33 UTC. The server refused it with `no resolved source maze for requested quest`.
- The authoritative catalog's dungeon 53 maze 4 is tied to quest 3354. Its source script repeats `[size]`: first `(4,3)`, then `(4,4)`. The source start room is `(3,3)`, and three later map rows also use row 3. Parsing the first size marks the maze `invalid room specification`.
- The old partial import contains maps 76400..76403. The three additional source rows 76404, 76405, and 76407 were imported from `server/work/client-build/Script.inner.pvf`, whose checksum matches the catalog's `7ef2db59...44d88e80`. Their script SHA-256 values are `424fcfd554ae81ac27896ae1d10d0ad379c53ed6604f9027662f10bade4b07f4`, `e808683ee3f5892e6e7012107199e2a91554f9761931866a9e5355d534a7725b`, and `023bd1953012d9f1913883742f44ba3cb0325eb71ee590f9d992e3ea76fce038`.
- Although the local runtime PVF has a different whole-archive checksum, these three map scripts have identical raw SHA-256 values in that runtime copy.

## Confirmed baseline: attempt 1/3

Use the last declared `[size]` for this source maze and add only the three missing maps to `dungeons.full.json`. A normalized comparison of the old and new catalogs found only dungeon 53 changed, three maps added, and no existing map or scene route changed. The catalog source checksum remains unchanged. No database or saved character data is changed.

The regression check selects quest 3354 and resolves all seven source rooms, starting at map 76407. `go test ./...` and `go vet ./...` passed. The user confirmed in a live client that the second quest now enters the instance successfully. Candidate executable SHA-256: `941E9D9C6A70FCFF3E642F94A8002236876434DC4074D8DFD94A47C8C754DA1A`.
