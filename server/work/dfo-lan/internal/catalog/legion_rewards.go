package catalog

import (
	"fmt"
	"math"
)

// A phase owns the source dungeon identity, regardless of its presentation order.
// Party/pass/PC benefits require independent entitlement evidence.
type LegionPhaseRewards struct {
	Content, Checksum string
	Dungeon           uint32
	WeeklyLimit       uint32
	Basic, Additional []LegionOperationReward
}

// The caller supplies the capture-proven default UI ordinal. Dungeon IDs
// belong solely to the prepared source, never to the protocol fixture table.
func IspinsDefaultStageDungeon(contents *LegionContents, ordinal int) (uint32, error) {
	if contents == nil {
		return 0, fmt.Errorf("Ispins source stage list missing")
	}
	c, ok := contents.Contents["Ispins"]
	if !ok || len(c.Dungeons) != 4 || c.LastPhase != 3 || ordinal < 0 || ordinal >= len(c.Dungeons) || c.Dungeons[ordinal].Dungeon == 0 {
		return 0, fmt.Errorf("Ispins source does not match native four-stage geometry")
	}
	return c.Dungeons[ordinal].Dungeon, nil
}

func IspinsPhaseRewards(contents *LegionContents, operations map[uint16]IspinsOperationDefinition, dungeon uint32, operation uint16) (LegionPhaseRewards, error) {
	var r LegionPhaseRewards
	if contents == nil || contents.Source.Checksum == "" {
		return r, fmt.Errorf("native legion reward source missing")
	}
	content, ok := contents.Contents["Ispins"]
	if !ok || content.PhaseReward <= 0 || content.PhaseReward > math.MaxUint32 {
		return r, fmt.Errorf("Ispins phase reward rule missing")
	}
	owns := false
	for _, d := range content.Dungeons {
		owns = owns || d.Dungeon == dungeon
	}
	if !owns {
		return r, fmt.Errorf("dungeon does not belong to Ispins source")
	}
	op, ok := operations[operation]
	if !ok || op.Difficulty == "" {
		return r, fmt.Errorf("Ispins operation difficulty missing")
	}
	// Only the upstream time-limit operation has an established outcome executor.
	if op.Type != 6 || op.FixedValue == 0 {
		return r, fmt.Errorf("Ispins operation%d outcome executor is not bound (source type%d)", operation, op.Type)
	}
	r = LegionPhaseRewards{Content: content.Name, Checksum: contents.Source.Checksum, Dungeon: dungeon, WeeklyLimit: uint32(content.PhaseReward)}
	for _, row := range content.Rewards {
		if row.PartyType != "solo" || row.Difficulty != op.Difficulty || row.Kind != "basic" {
			continue
		}
		if len(row.Values) != 3 {
			return r, fmt.Errorf("invalid legion basic source row")
		}
		for _, v := range row.Values {
			if v <= 0 || v > math.MaxUint32 {
				return r, fmt.Errorf("invalid legion basic scalar")
			}
		}
		r.Basic = append(r.Basic, LegionOperationReward{Difficulty: row.Difficulty, Weight: uint32(row.Values[0]), Template: uint32(row.Values[1]), Amount: uint32(row.Values[2])})
	}
	for _, row := range op.Rewards {
		if row.Difficulty == op.Difficulty {
			r.Additional = append(r.Additional, row)
		}
	}
	if len(r.Basic) == 0 || len(r.Additional) == 0 {
		return r, fmt.Errorf("incomplete source phase reward pools")
	}
	return r, nil
}
