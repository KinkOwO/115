package legion

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"fmt"
	"math"
)

// CloisterInfoState follows US reader 1420a5730: five eight-byte dungeon
// records, four ten-byte operation records and the four-byte selection tail.
// The records at +63 describe offered operations, not character equipment.
type CloisterInfoState struct {
	Prefix                                  uint16
	Selected                                byte
	State, Outcome, Phase, Following, Extra int32
	Stages                                  [5][2]int32
	Offers                                  [4]DuskyOperationRecord
	TailFlag, TailReserved                  byte
	Reselections                            int16
}

func CloisterInfo(s CloisterInfoState) []byte {
	p := make([]byte, 107)
	binary.LittleEndian.PutUint16(p, s.Prefix)
	p[2] = s.Selected
	for i, v := range []int32{s.State, s.Outcome, s.Phase, s.Following, s.Extra} {
		binary.LittleEndian.PutUint32(p[3+i*4:], uint32(v))
	}
	for i, row := range s.Stages {
		for j, v := range row {
			binary.LittleEndian.PutUint32(p[23+i*8+j*4:], uint32(v))
		}
	}
	for i, row := range s.Offers {
		at := 63 + i*10
		for j, v := range []int16{row.Index, row.Fixed, row.Value, row.Grade} {
			binary.LittleEndian.PutUint16(p[at+j*2:], uint16(v))
		}
		p[at+8], p[at+9] = row.Flag, row.Reserved
	}
	p[103], p[104] = s.TailFlag, s.TailReserved
	binary.LittleEndian.PutUint16(p[105:], uint16(s.Reselections))
	return p
}

func nativeOperationRecord(c catalog.NativeLegionCard, dusky bool) (DuskyOperationRecord, error) {
	var row DuskyOperationRecord
	if c.Index == 0 || c.Index > math.MaxInt16 {
		return row, fmt.Errorf("native operation index outside wire contract")
	}
	value, err := DuskyOperationOperand(c.Parameter)
	if err != nil {
		return row, err
	}
	var fixed int16
	if c.FixedValue != nil {
		fixed, err = DuskyOperationOperand(*c.FixedValue)
		if err != nil {
			return row, err
		}
	}
	grade := c.Pool + 1
	if dusky && c.Pool >= 4 {
		grade = 3
		if c.Pool == 6 {
			grade = 4
		}
	}
	if grade > math.MaxInt16 {
		return row, fmt.Errorf("native card grade outside wire contract")
	}
	return DuskyOperationRecord{Index: int16(c.Index), Fixed: fixed, Value: value, Grade: int16(grade)}, nil
}

// Info projects one run snapshot. state/outcome are explicit native transition
// codes so presentation (including final movie) does not mutate game progress.
func (r *NativeRun) Info(state, outcome int32) ([]byte, error) {
	if r == nil || r.Phase < 0 || r.Phase > 2 || len(r.Offers) > 4 || r.Reselections > math.MaxInt16 {
		return nil, fmt.Errorf("invalid native run projection")
	}
	selected := byte(255)
	if r.Selected >= 0 {
		if r.Selected >= len(r.Offers) {
			return nil, fmt.Errorf("selected native offer absent")
		}
		selected = byte(r.Selected)
	}
	phase := int32(r.Phase)
	initial := r.State == NativeCountdown
	if initial {
		phase = -1
		selected = 255
	}
	var offers [4]DuskyOperationRecord
	if !initial {
		for i, c := range r.Offers {
			row, err := nativeOperationRecord(c, r.Source.Name == "DuskyIsland")
			if err != nil {
				return nil, err
			}
			offers[i] = row
		}
	}
	status := func(i int) int32 {
		if r.History[i] != nil {
			return 1
		}
		if i == r.Phase {
			return 0
		}
		return 2
	}
	if r.Source.Name == "Dimension Cloister" {
		s := CloisterInfoState{Selected: selected, State: state, Outcome: outcome, Phase: phase, Following: 1, Extra: -1, Offers: offers, Reselections: int16(r.Reselections)}
		for i := range s.Stages {
			s.Stages[i] = [2]int32{-1, -1}
		}
		if !initial {
			for i, index := range r.Order {
				s.Stages[i] = [2]int32{int32(index), status(i)}
			}
		}
		return CloisterInfo(s), nil
	}
	if r.Source.Name != "DuskyIsland" {
		return nil, fmt.Errorf("unknown native run projection")
	}
	s := DuskyInfoState{Selected: selected, State: state, Outcome: outcome, Phase: phase, Following: 1, Extra: -1, Offers: offers, Reselections: int16(r.Reselections)}
	for i := range s.Stages {
		s.Stages[i] = DuskyStageRecord{Index: 255, First: -1, Second: -1}
	}
	if !initial {
		for i, index := range r.Order {
			if index < 0 || index > 254 {
				return nil, fmt.Errorf("native stage index outside wire contract")
			}
			s.Stages[i] = DuskyStageRecord{Index: byte(index), First: status(i), Second: -1}
			card := r.History[i]
			if i == r.Phase && r.Selected >= 0 {
				card = &r.Offers[r.Selected]
			}
			if card != nil {
				row, err := nativeOperationRecord(*card, true)
				if err != nil {
					return nil, err
				}
				s.History[i] = row
			}
		}
	}
	return DuskyInfo(s), nil
}
