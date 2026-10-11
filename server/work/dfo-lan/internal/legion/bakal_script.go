// Bakal raid script frames (回放帧构造) — the rules-driven builders behind the
// N2286/N2288/N572 channels of the run.
//
// The handover binary drove these channels from its bakal.etc script engine
// (matchScript/executeScript/dispatchScript). This file keeps the subset the
// B-layer clear run pins byte-for-byte:
//
//   - the opening N2286 roster: 8 rows, one per [CREATE MONSTER] location, at
//     full [MONSTER MAX HP] (fixture bakal_monsters, 153B);
//   - the +1s boss health rows: the four boss-dungeon slots published with
//     row state 2 (fixtures bakal_script_monsters@12:29:33.878 +
//     bakal_script_actor_health@12:29:33.878);
//   - defeat rows from confirmed owned entity deaths: slot 0, row state 1,
//     HP 0 at the reported script location (fixture 20B);
//   - the boss HP progress pair the final fight published before its clear
//     (row state 1 + row state 2 with the same HP, 12:36:34.183 pair).
//
// Known gaps stay explicit (no invented bodies, root AGENTS.md §0):
//   - phase INIT's delayed field leaders execute in bakal_presence.go;
//     later recurring field-monster waves and movement timers remain outside
//     this subset;
//   - the buff grant policy (which slots a boss clear bumps) is weighted
//     random (binary symbols bakalWeightedNumbers/bakalNormalBiddingCount);
//     grants arrive through the GrantHook instead of a guessed table;
//
// Source initial symbols and timed anger changes use N570; this is not a
// complete replacement of the handover's general trigger interpreter.
package legion

import (
	"fmt"
	"sort"

	"dfolan/internal/catalog"
)

// BakalFrame is one outbound notification of the Bakal run. Wireprobe wraps
// these into outboundPacket (Kind 0); the command ACKs (2070/2073/2089) stay
// with the dispatch layer that answers the client request.
type BakalFrame struct {
	Name string
	ID   uint16
	Body []byte
}

// bakalRosterSlot is one row of the opening N2286 roster: the slot number the
// wire carries next to each [CREATE MONSTER] location. The location set is
// bakal.etc's (cross-checked against the catalog in the tests); the slot
// numbers are wire evidence from the handover clear run — the catalog does
// not project the slot assignment, so the pairing is recorded here.
// Wire monster type enum (handover BakalMonsterType), independent of PVF
// placement. Positions and the number of placements come from the script.
func BakalMonsterType(name string) uint32 {
	// Native147714120 namespace, cross-checked against the official paired
	// N2286/N570 creation updates: swan9/IS EXIST SWAN, eclair11/IS EXIST ECLAIR.
	return map[string]uint32{"bakal": 1, "sparazzi": 2, "skasa": 3, "hisma": 4, "basilisk": 5, "blona": 6, "gerda": 7, "nympha": 8, "swan": 9, "steich": 10, "eclair": 11, "brute": 12, "steel dragon": 13, "zamir": 14, "routund": 15, "normal monsters": 16}[name]
}

// BakalOpeningRoster builds the 8-row opening roster from the phase rules:
// every row at full [MONSTER MAX HP], row state 1.
func BakalOpeningRoster(phase *catalog.BakalPhaseRules) []BakalMonster {
	rows := make([]BakalMonster, 0, len(phase.InitMonsters))
	for _, spawn := range phase.InitMonsters {
		rows = append(rows, BakalMonster{
			Slot:     BakalMonsterType(spawn.Name),
			Location: uint32(spawn.Location),
			Count:    0, // official N2286 placement; 1 is a state transition
			MaxHP:    uint32(phase.MonsterMaxHP),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Location < rows[j].Location })
	return rows
}

// bakalBossHealthSlots are the four boss-dungeon slots the +1s publish carries
// (row state 2): the bakal room and the three dragon lords, in wire order.

// BakalBossHealthRows builds the +1s boss health publish: the four
// boss-dungeon rows at full HP with row state 2.
func BakalBossHealthRows(phase *catalog.BakalPhaseRules) []BakalMonster {
	var rows []BakalMonster
	for _, row := range BakalOpeningRoster(phase) {
		for _, spawn := range phase.InitMonsters {
			if uint32(spawn.Location) == row.Location {
				if _, ok := phase.BossHP[spawn.Name]; ok {
					row.Count = 2
					rows = append(rows, row)
				}
				break
			}
		}
	}
	return rows
}

// BakalDefeatRow builds the defeat row a confirmed battle report publishes:
// slot 0, row state 1, HP 0 at the reported script location.
func BakalDefeatRow(location uint32) BakalMonster {
	return BakalMonster{Slot: 0, Location: location, Count: 1, MaxHP: 0}
}

// BakalBossProgressRows builds the HP progress pair the final fight
// published at half HP (12:36:34.183): row state 1 and row state 2 with the
// same current HP for the boss slot.
func BakalBossProgressRows(slot, location, hp uint32) []BakalMonster {
	return []BakalMonster{
		{Slot: slot, Location: location, Count: 1, MaxHP: hp},
		{Slot: slot, Location: location, Count: 2, MaxHP: hp},
	}
}

// BakalOpenBuffCounts reads the initial N2288 panel out of the phase rules:
// one charge count per [ADD BAKAL RAID BUFF] row (normal mode opens 2/2/2/2/2,
// hard mode a single slot 4 at 2).
func BakalOpenBuffCounts(phase *catalog.BakalPhaseRules) [5]byte {
	var counts [5]byte
	for _, grant := range phase.RaidBuffs {
		if grant.Slot >= 0 && grant.Slot < len(counts) {
			counts[grant.Slot] = byte(grant.Count)
		}
	}
	return counts
}

// BakalLocationOfDungeon resolves the raid-map location of one of the 17
// opened dungeons from the catalog location table.
func BakalLocationOfDungeon(rules *catalog.BakalRaidRules, dungeon uint32) (uint32, error) {
	for _, loc := range rules.Locations {
		if loc.Dungeon == dungeon {
			return uint32(loc.Index), nil
		}
	}
	return 0, fmt.Errorf("bakal dungeon %d has no raid location", dungeon)
}

// BakalDungeonOfLocation resolves the dungeon a raid-map location belongs to
// (0 for camp/hub locations that map no dungeon).
func BakalDungeonOfLocation(rules *catalog.BakalRaidRules, location uint32) uint32 {
	for _, loc := range rules.Locations {
		if uint32(loc.Index) == location {
			return loc.Dungeon
		}
	}
	return 0
}
