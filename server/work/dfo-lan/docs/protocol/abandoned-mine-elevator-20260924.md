# Abandoned Mine elevator, quest 3352

## Evidence

- Current client manual runs at 2026-09-24 11:26 and 11:32 UTC entered dungeon 53, maze 2, map 76384. The server sent one monster in NOTI29. No transition to map 76382 followed.
- Source map `map/cataclysm/northmyre/05_town_of_doubt/3352_76384.map` has an off-map template-1 monster at (424,-364) and passive objects 1111 (`ElevatorScroll`), 1112 (`ElevatorSummon`), and 1113 (`ElevatorControl`). The ordinary map 16408 uses the same elevator objects without the off-map monster.
- Source `ElevatorSummon.obj` declares eleven timed spawns. The client's manual trace records all eleven 62014/62015 deaths by 11:32:36 UTC, but never a death of template 1 or a subsequent move request.

## Confirmed temporary measure: attempt 1/3

Exclude only the template-1 off-map sentinel from the server's spawn list for map 76384. This leaves the client-controlled timed wave unchanged. The change is reversible by reverting the one guarded branch in `internal/dungeon/session.go`.

The working explanation is that the surviving server-spawned sentinel kept the client's elevator controller occupied. The user manually confirmed that the elevator stopped and the door opened to the next room with this candidate build.

**This is a temporary map-specific measure.** The complete native `ElevatorSummon` / `ElevatorControl` OBJ/ACT event linkage is not implemented or reconstructed. Replace this exception when that native behavior is understood and verified. Candidate executable SHA-256: `B3492056DF6720E24D19DEFACF2524F234B40BB9F047283F0A3D7834A93F017C`.
