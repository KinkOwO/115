package protocol

import (
	"encoding/binary"
	"fmt"
)

// N2253 completes the reward presentation, even when the local policy has no
// second award pool. Current1424FE320 reads2405B; mode+256 ->140696E90 ->
// 14250F5B0 skips additional rows when byte0!=1, refreshes the existing basic
// rows, then opens window642. No item may be added by this presentation packet.
func LegionNoAdditionalRewards115() []byte { return make([]byte, 2405) }

// LegionBasicRewards115 is the native N2252 layout, not Moon's N35.
// Current1424FE3F0 reads7772B;14250FAF0 reads4 seats x10 rows x40B,
// followed by140 additional44B rows, u64 elapsedMS and4 seat flags.
// Row template/value are at0/4, u16 at12, metadata21 at14. The initial two
// rows take the item branch; later categories have other semantics. Additional
// items use the35 per-seat44B records at1600+1540*seat; field+40=0 selects
// the native unmerged item branch (14250FAF0), preserving duplicate instances.
// This renderer awards nothing: it must receive server-frozen instance rows.
func LegionBasicRewards115(present [4]bool, rewards [4][]ConquestRewardValue115, elapsedMS uint64) ([]byte, error) {
	p := make([]byte, 7772)
	any := false
	for seat, active := range present {
		if !active {
			if len(rewards[seat]) != 0 {
				return nil, fmt.Errorf("legion reward for absent seat")
			}
			continue
		}
		any = true
		if len(rewards[seat]) == 0 || len(rewards[seat]) > 37 {
			return nil, fmt.Errorf("legion item rows require1..37 frozen grants per seat")
		}
		for i, r := range rewards[seat] {
			if r.Template == 0 || r.Value == 0 && !r.Equipment {
				return nil, fmt.Errorf("invalid legion frozen reward row")
			}
			off := 400*seat + 40*i
			if i >= 2 {
				off = 1600 + 1540*seat + 44*(i-2)
			}
			binary.LittleEndian.PutUint32(p[off:], r.Template)
			binary.LittleEndian.PutUint32(p[off+4:], r.Value)
			copy(p[off+14:off+35], r.Metadata[:])
		}
	}
	if !any {
		return nil, fmt.Errorf("empty legion reward roster")
	}
	// Native converts elapsed to signed32 before displaying seconds.
	if elapsedMS > 0x7fffffff {
		return nil, fmt.Errorf("legion elapsed time out of native range")
	}
	binary.LittleEndian.PutUint64(p[7760:], elapsedMS)
	return p, nil
}
