package protocol

import (
	"encoding/binary"
	"fmt"
)

const NotiUnassignedMonsterAdd uint16 = 2194

// Native1452B9C10 consumes u8 count + count*33 bytes. Moon's141330350
// uses row0/1 for grid,2 for entity,6 for template,17/21 for coordinates.
type UnassignedMonster115 struct {
	Grid     [2]byte
	Entity   uint16
	Template uint32
	Rank     byte // native row+11; Bakal dynamic bosses use rank 3 / grow type
	X, Y     int32
	Carried  bool // Moon source move: row+29 bit0, NOT a new normal spawn.
}

func UnassignedMonsterAdd115(rows []UnassignedMonster115) ([]byte, error) {
	if len(rows) == 0 || len(rows) > 255 {
		return nil, fmt.Errorf("invalid dynamic monster count")
	}
	p := make([]byte, 1+33*len(rows))
	p[0] = byte(len(rows))
	seen := map[uint16]bool{}
	for i, v := range rows {
		if v.Entity == 0 || v.Entity == 65535 || seen[v.Entity] || v.Template == 0 || v.Rank > 3 {
			return nil, fmt.Errorf("invalid dynamic monster identity")
		}
		seen[v.Entity] = true
		r := p[1+i*33 : 1+(i+1)*33]
		r[0], r[1] = v.Grid[0], v.Grid[1]
		binary.LittleEndian.PutUint32(r[2:], uint32(v.Entity))
		binary.LittleEndian.PutUint32(r[6:], v.Template)
		r[11] = v.Rank
		binary.LittleEndian.PutUint32(r[13:], 100) // observed enemy affiliation/default
		binary.LittleEndian.PutUint32(r[17:], uint32(v.X))
		binary.LittleEndian.PutUint32(r[21:], uint32(v.Y))
		binary.LittleEndian.PutUint32(r[25:], 0xffffffff) // native unset sentinel
		if v.Carried {
			binary.LittleEndian.PutUint32(r[29:], 1)
		}
	}
	return p, nil
}
