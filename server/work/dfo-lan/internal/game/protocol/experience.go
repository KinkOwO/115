package protocol

import (
	"encoding/binary"
	"fmt"
)

type ExperienceUpdate struct {
	Level         byte
	Total         uint64
	SP, TP        [2]uint16
	CurrencySlot2 uint32
}

func ExperienceState(s ExperienceUpdate) ([]byte, error) {
	if s.Level == 0 {
		return nil, fmt.Errorf("invalid experience level")
	}
	// Native1452ce4d0, current self branch: two-u32 EXP adapter, two SP/TP
	// trees, slot2 currency, empty optional EXP categories and honor state.
	p := make([]byte, 82)
	p[0] = s.Level
	binary.LittleEndian.PutUint64(p[1:], s.Total)
	for i := 0; i < 2; i++ {
		binary.LittleEndian.PutUint16(p[13+i*2:], s.SP[i])
		binary.LittleEndian.PutUint16(p[17+i*2:], s.TP[i])
	}
	binary.LittleEndian.PutUint32(p[21:], s.CurrencySlot2)
	return p, nil
}
