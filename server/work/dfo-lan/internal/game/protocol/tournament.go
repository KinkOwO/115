package protocol

import "fmt"

// The 115 client's NOTI 0x174 reader consumes four rounds of teams, followed
// by four path actors. The widths below follow sub_1452B8F50 in DFO.exe.i64.
type TournamentTeam struct {
	Position byte
	Code     uint32
	Strength uint16
}

type TournamentOpening struct {
	DungeonID  uint32
	Difficulty byte
	Rounds     [4][]TournamentTeam
	Path       [4]DungeonMonster
}

func TournamentInfo(s TournamentOpening) ([]byte, error) {
	if s.DungeonID == 0 {
		return nil, fmt.Errorf("missing tournament dungeon")
	}
	p := add32(nil, s.DungeonID)
	p = append(p, s.Difficulty, 1) // source [limit party count] 1
	for i, teams := range s.Rounds {
		if len(teams) != 16>>i {
			return nil, fmt.Errorf("invalid tournament round %d team count", i+1)
		}
		p = append(p, byte(i+1), byte(len(teams)))
		for j, team := range teams {
			if int(team.Position) != j {
				return nil, fmt.Errorf("invalid tournament team position")
			}
			p = append(p, team.Position)
			p = add16(add32(p, team.Code), team.Strength)
		}
	}
	for i, actor := range s.Path {
		if actor.Entity == 0 || actor.Template == 0 {
			return nil, fmt.Errorf("missing tournament path actor %d", i+1)
		}
		p = append(p, byte(i+1))
		p = add32(add16(p, actor.Entity), actor.Template)
		p = append(p, actor.Level, actor.Rank)
	}
	if len(p) != 260 {
		return nil, fmt.Errorf("invalid tournament info length %d", len(p))
	}
	return p, nil
}

// NOTI 0x175 branch 1 initializes the source MAP without extra actor groups.
// Its native reader is sub_1452B94A0; the map ID is looked up as u32.
func TournamentMapInfo(position [2]byte, seed, mapID uint32) ([]byte, error) {
	if mapID == 0 {
		return nil, fmt.Errorf("missing tournament map")
	}
	p := []byte{position[0], position[1]}
	p = add32(p, seed)
	p = append(p, 0, 1)
	p = add32(p, mapID)
	return append(p, 0, 0, 0), nil
}

type TournamentCard struct{ Template, Amount uint32 }

// Native NOTI 374 reader sub_1452B8CF0 reads two rows of two ten-byte cards.
func TournamentClearReward(rounds byte, cards [2][2]TournamentCard) ([]byte, error) {
	if rounds != 4 {
		return nil, fmt.Errorf("tournament has not reached final round")
	}
	p := add32([]byte{rounds}, 0) // this quest DGN has no tournament EXP section
	p = append(p, 0)
	for _, row := range cards {
		p = append(p, 2)
		for _, card := range row {
			p = add16(add32(add32(p, card.Template), card.Amount), 0)
		}
	}
	return p, nil
}

// CMD 449's native success reader consumes eight party-slot flags.
func TournamentSelectState() []byte { return []byte{1, 1, 255, 255, 255, 1, 255, 255, 255} }

// CMD 450's native reader consumes two length-prefixed card-owner arrays.
func TournamentSelection(selected [2]byte) []byte {
	p := []byte{1}
	for _, choice := range selected {
		p = append(p, 2, 255, 255)
		if choice < 2 {
			p[len(p)-2+int(choice)] = 0
		}
	}
	return p
}
