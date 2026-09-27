package dungeon

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
	"sort"
)

type TournamentRun struct {
	Opening      protocol.TournamentOpening
	Seed         uint32
	CurrentRound byte
	Rewards      [2][2]protocol.TournamentCard
	Selected     [2]byte
	RewardReady  bool
}

// TournamentPrizeRules reads the nested result-card table from this DGN.
// The quest variant has gold in result row 4 and item rewards in row 5.
func TournamentPrizeRules(d catalog.DungeonDefinition) (uint32, [][3]uint32, error) {
	var items [][3]uint32
	var goldRate uint32
	var championRows [2][4]int32
	var rowsFound [2]bool
	for i, cell := range d.Script.Cells {
		if cell.Type != 3 {
			continue
		}
		switch cell.Text {
		case "[tournament clear reward gold rate]":
			if i+1 < len(d.Script.Cells) && d.Script.Cells[i+1].Type == 0 && d.Script.Cells[i+1].Value > 0 {
				goldRate = uint32(d.Script.Cells[i+1].Value)
			}
		case "[reward item rate]":
			for j := i + 1; j+2 < len(d.Script.Cells) && d.Script.Cells[j].Type == 0; j += 3 {
				a, b, c := d.Script.Cells[j], d.Script.Cells[j+1], d.Script.Cells[j+2]
				if b.Type != 0 || c.Type != 0 || a.Value <= 0 || b.Value <= 0 || c.Value <= 0 {
					return 0, nil, fmt.Errorf("invalid tournament item rate")
				}
				items = append(items, [3]uint32{uint32(a.Value), uint32(b.Value), uint32(c.Value)})
			}
		case "[/reward item rate]":
			for j := i + 1; j+3 < len(d.Script.Cells) && d.Script.Cells[j].Type == 0; j += 4 {
				row := d.Script.Cells[j : j+4]
				if row[1].Type != 0 || row[2].Type != 0 || row[3].Type != 0 {
					break
				}
				if row[0].Value == 4 || row[0].Value == 5 {
					n := row[0].Value - 4
					rowsFound[n] = true
					for k := 0; k < 4; k++ {
						championRows[n][k] = row[k].Value
					}
				}
			}
		}
	}
	if goldRate == 0 || len(items) == 0 || !rowsFound[0] || !rowsFound[1] || championRows[0] != [4]int32{4, 100, 0, 0} || championRows[1] != [4]int32{5, 0, 100, 0} {
		return 0, nil, fmt.Errorf("unsupported tournament champion prize source")
	}
	return goldRate, items, nil
}

type tournamentCandidate struct {
	code     uint32
	strength uint16
	player   bool
}

func tournamentCells(cells []pvf.Token, section string) []pvf.Token {
	var out []pvf.Token
	active := false
	for _, cell := range cells {
		if cell.Type == 3 {
			active = cell.Text == section
			continue
		}
		if active {
			out = append(out, cell)
		}
	}
	return out
}

func tournamentDungeon(d catalog.DungeonDefinition) bool {
	v := tournamentCells(d.Script.Cells, "[tournament dungeon]")
	return len(v) == 1 && v[0].Type == 0 && v[0].Value == 1
}

func newTournamentRun(d catalog.DungeonDefinition, script catalog.ScriptRecord, difficulty byte) (*TournamentRun, []protocol.DungeonMonster, error) {
	limit := tournamentCells(d.Script.Cells, "[limit party count]")
	if len(limit) != 1 || limit[0].Type != 0 || limit[0].Value != 1 || d.BasisLevel == 0 || d.BasisLevel > 255 {
		return nil, nil, fmt.Errorf("unsupported tournament party limit or basis level")
	}
	starts := tournamentCells(script.Cells, "[tournament start area]")
	if len(starts) != 8 {
		return nil, nil, fmt.Errorf("tournament source requires two solo start areas")
	}
	for i := 0; i < 8; i += 4 {
		if starts[i].Type != 0 || starts[i].Value != 1 || starts[i+1].Type != 0 || starts[i+2].Type != 0 || starts[i+3].Type != 0 {
			return nil, nil, fmt.Errorf("invalid tournament start area")
		}
	}
	cells := tournamentCells(script.Cells, "[tournament enemies]")
	if len(cells) != 47 || cells[0].Type != 0 || cells[0].Value != 1 || cells[1].Type != 6 || cells[1].Text != "[monster]" {
		return nil, nil, fmt.Errorf("tournament source requires 15 solo opponents")
	}
	candidates := make([]tournamentCandidate, 0, 15)
	for i := 2; i < len(cells); i += 3 {
		if cells[i].Type != 0 || cells[i].Value <= 0 || cells[i+1].Type != 0 || cells[i+1].Value < 0 || cells[i+1].Value > 65535 || cells[i+2].Type != 8 {
			return nil, nil, fmt.Errorf("invalid tournament opponent row %d", (i-2)/3)
		}
		candidates = append(candidates, tournamentCandidate{code: uint32(cells[i].Value), strength: uint16(cells[i+1].Value)})
	}
	var seed uint32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &seed); err != nil {
		return nil, nil, err
	}
	rng := mathrand.New(mathrand.NewSource(int64(seed)))
	rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].strength < candidates[j].strength })
	playerPosition := rng.Intn(16)
	first := make([]tournamentCandidate, 16)
	first[playerPosition] = tournamentCandidate{player: true}
	first[playerPosition^1] = candidates[0]
	remaining := append([]tournamentCandidate(nil), candidates[1:]...)
	rng.Shuffle(len(remaining), func(i, j int) { remaining[i], remaining[j] = remaining[j], remaining[i] })
	next := 0
	for i := range first {
		if i == playerPosition || i == playerPosition^1 {
			continue
		}
		first[i] = remaining[next]
		next++
	}
	t := &TournamentRun{Opening: protocol.TournamentOpening{DungeonID: d.ID, Difficulty: difficulty}, Seed: seed, CurrentRound: 1}
	var monsters []protocol.DungeonMonster
	previous := first
	for round := 0; round < 4; round++ {
		player := -1
		for i, team := range previous {
			if team.player {
				player = i
			}
			t.Opening.Rounds[round] = append(t.Opening.Rounds[round], protocol.TournamentTeam{Position: byte(i), Code: team.code, Strength: team.strength})
		}
		if player < 0 {
			return nil, nil, fmt.Errorf("tournament bracket lost player")
		}
		opponent := previous[player^1]
		rank := byte(0)
		if round == 3 {
			rank = 3
		}
		actor := protocol.DungeonMonster{Entity: uint16(4096 + round), SourceIndex: uint32(round), Level: byte(d.BasisLevel), Template: opponent.code, Rank: rank, Team: 100}
		t.Opening.Path[round] = actor
		monsters = append(monsters, actor)
		if round == 3 {
			break
		}
		winners := make([]tournamentCandidate, len(previous)/2)
		for i := range winners {
			left, right := previous[2*i], previous[2*i+1]
			if right.player || (!left.player && right.strength > left.strength) ||
				(!left.player && right.strength == left.strength && rng.Intn(2) == 1) {
				winners[i] = right
			} else {
				winners[i] = left
			}
		}
		previous = winners
	}
	return t, monsters, nil
}
