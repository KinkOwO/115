package legion

import (
	"crypto/rand"
	"math/big"
	"sort"
	"strings"
	"time"

	"dfolan/internal/catalog"
)

// Native createScriptMonster/deleteScriptMonster update IS EXIST <kind>.
// Incoming_ready.act destroys ordinary actors when their region's symbol is0.
func (o *BakalOpening) presenceValues() map[string]int32 {
	values := map[string]int32{}
	for name := range o.rules.Symbols {
		if strings.HasPrefix(name, "[IS EXIST ") {
			values[name] = 0
		}
	}
	for location, kind := range o.placements {
		name := "[IS EXIST " + strings.ToUpper(kind) + "]"
		if _, known := values[name]; known && !o.defeatedLocations[location] {
			values[name] = 1
		}
	}
	return values
}

func (o *BakalOpening) presenceFrames(kinds ...string) []BakalFrame {
	values := o.presenceValues()
	wanted := map[string]bool{}
	for _, kind := range kinds {
		wanted["[IS EXIST "+strings.ToUpper(kind)+"]"] = true
	}
	var names []string
	for name := range values {
		if len(kinds) == 0 || wanted[name] {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return o.rules.Symbols[names[i]] < o.rules.Symbols[names[j]] })
	var frames []BakalFrame
	for _, name := range names {
		frames = append(frames, o.SymbolFrame(name, values[name])...)
	}
	return frames
}

// MonsterPlacements is the current source-created roster, including delayed
// initial reservations. A defeated placement remains available to death
// confirmation; spawning separately enforces IsLocationDefeated.
func (o *BakalOpening) MonsterPlacements() []catalog.BakalMonsterSpawn {
	var placements []catalog.BakalMonsterSpawn
	for location, name := range o.placements {
		placements = append(placements, catalog.BakalMonsterSpawn{Location: int(location), Name: name})
	}
	sort.Slice(placements, func(i, j int) bool { return placements[i].Location < placements[j].Location })
	return placements
}

// RESERVE CREATE MONSTER's first integer is a delay, not a location. The
// source LOCATION INFO / CREATABLE MONSTER determines eligible free slots.
// This executes only the phase INIT reservations; later recurring wave and
// movement triggers still require the general script executor.
func (o *BakalOpening) publishInitialReservations(now time.Time) []BakalFrame {
	var frames []BakalFrame
	for i, reservation := range o.script.ReserveMonsters {
		if o.reservedPublished[i] || now.Before(o.readyAt.Add(time.Duration(reservation.Location)*time.Second)) {
			continue
		}
		exists := false
		for location, kind := range o.placements {
			exists = exists || kind == reservation.Name && !o.defeatedLocations[location]
		}
		if exists {
			o.reservedPublished[i] = true
			continue
		}
		var candidates []uint32
		for _, loc := range o.rules.Locations {
			if _, occupied := o.placements[uint32(loc.Index)]; occupied {
				continue
			}
			for _, kind := range loc.Creatable {
				if kind == reservation.Name {
					candidates = append(candidates, uint32(loc.Index))
					break
				}
			}
		}
		if len(candidates) == 0 {
			continue
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		choice, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err != nil {
			continue // retain the pending reservation for the next serial tick
		}
		location := candidates[choice.Int64()]
		o.placements[location] = reservation.Name
		o.reservedPublished[i] = true
		frames = append(frames, BakalFrame{"bakal_initial_reserved_monster", NotiBakalMonsters, BakalMonstersFrame([]BakalMonster{{Slot: BakalMonsterType(reservation.Name), Location: location, MaxHP: uint32(o.script.MonsterMaxHP)}})})
		frames = append(frames, o.presenceFrames(reservation.Name)...)
	}
	return frames
}
