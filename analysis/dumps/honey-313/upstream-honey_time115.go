package protocol

import (
	"encoding/binary"
	"fmt"
)

const HoneyTimeEvent115 uint32 = 313
const HoneyTimeInfo115 uint16 = 1294

// HoneyTimeGroup115 contains raw reader fields. In particular, Step and Limit
// are not interpreted as claim masks. Reader 1452d4b30 skips group 3 in its
// first loop, reads sixteen individual bytes for groups 0/1, and a u32 for 2.
type HoneyTimeGroup115 struct {
	Enabled byte
	Seconds uint32
	Step    byte
	Values  [16]byte
	Mask    uint32
	Limit   byte
}

type HoneyTimeStatus115 struct {
	Groups        [3]HoneyTimeGroup115
	SourceFlag    byte
	PeriodSeconds uint32
	LoopCounts    [4]uint32
	FinalFlag     byte
}

// Bytes encodes only the 81 bytes consumed by the current US reader. Its
// four loop counters are fixed by 143cc6090(1), not a count prefix. Registration
// 1452faa44..53 binds the reader to NOTI1294. No gateway enables this codec yet.
func (s HoneyTimeStatus115) Bytes() ([]byte, error) {
	for i, g := range s.Groups {
		if (i == 0 && (g.Enabled != 0 || g.Limit != 0)) || (i < 2 && g.Mask != 0) || (i == 2 && g.Values != [16]byte{}) {
			return nil, fmt.Errorf("honey time field absent in native group %d", i)
		}
	}
	p := add32(nil, HoneyTimeEvent115)
	for i, g := range s.Groups {
		if i > 0 {
			p = append(p, g.Enabled)
		}
		p = add32(p, g.Seconds)
		p = append(p, g.Step)
		if i < 2 {
			p = append(p, g.Values[:]...)
		} else {
			p = add32(p, g.Mask)
		}
		if i > 0 {
			p = append(p, g.Limit)
		}
	}
	p = append(p, s.SourceFlag)
	p = add32(p, s.PeriodSeconds)
	for _, n := range s.LoopCounts {
		p = add32(p, n)
	}
	return append(p, s.FinalFlag), nil
}

// Native 143cc7a60 sends CMD680/event313/action0 without additional words.
func DecodeHoneyTimeSync115(p []byte) error {
	if len(p) != 8 || binary.LittleEndian.Uint32(p) != HoneyTimeEvent115 || binary.LittleEndian.Uint32(p[4:]) != 0 {
		return fmt.Errorf("invalid honey time synchronization request")
	}
	return nil
}

// HoneyTimeRequest115 preserves the native group and zero-based UI offset.
// This is a wire contract, not permission to execute a claim: the source group,
// timer, offset and saved state must still be checked by the domain service.
type HoneyTimeRequest115 struct {
	Sync   bool
	Group  uint32
	Offset uint32
}

// DecodeHoneyTimeRequest115 follows UI virtual callback 14412c770. Its four
// group arrays map to actions 1, 2, 3, 5 (action 4 is skipped). After the event
// and action it writes the zero-based position in the selected UI array, not
// a one-based day, item ID, bit mask or extra group word.
func DecodeHoneyTimeRequest115(p []byte) (HoneyTimeRequest115, error) {
	if len(p) < 8 || binary.LittleEndian.Uint32(p) != HoneyTimeEvent115 {
		return HoneyTimeRequest115{}, fmt.Errorf("invalid honey time request header")
	}
	action := binary.LittleEndian.Uint32(p[4:])
	if action == 0 {
		if err := DecodeHoneyTimeSync115(p); err != nil {
			return HoneyTimeRequest115{}, err
		}
		return HoneyTimeRequest115{Sync: true}, nil
	}
	if len(p) != 12 {
		return HoneyTimeRequest115{}, fmt.Errorf("invalid honey time claim width")
	}
	var group uint32
	switch action {
	case 1, 2, 3:
		group = action - 1
	case 5:
		group = 3
	default:
		return HoneyTimeRequest115{}, fmt.Errorf("unverified honey time claim action %d", action)
	}
	offset := binary.LittleEndian.Uint32(p[8:])
	if offset > 0x7fffffff {
		return HoneyTimeRequest115{}, fmt.Errorf("negative native honey time offset")
	}
	return HoneyTimeRequest115{Group: group, Offset: offset}, nil
}
