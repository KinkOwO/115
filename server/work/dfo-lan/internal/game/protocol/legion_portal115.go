package protocol

import (
	"encoding/binary"
	"fmt"
)

type LegionPortal115 struct {
	Dungeon     uint32
	Difficulty  uint32
	TargetGrid  [2]int32
	SpawnWindow [4]int32
	Mode        byte
}

// Generic N2281 reader consumes38B and releases the direct-move UI latch.
func LegionDirectMoveNotice115() []byte { return make([]byte, 38) }

// Native14069BCC0 ->146D465C0 sends46 logical bytes, with an opaque13B
// prefix. Position words are presentation data, NOT a trusted spawn location.
func DecodeLegionPortal115(p []byte) (LegionPortal115, error) {
	return decodeLegionPortal115(p, true)
}

// Bakal portals carry a real two-dimensional source maze grid. The generic
// legion ingress has only witnessed column0; keep its stricter contract.
func DecodeBakalPortal115(p []byte) (LegionPortal115, error) {
	return decodeLegionPortal115(p, false)
}

func decodeLegionPortal115(p []byte, columnZero bool) (LegionPortal115, error) {
	var r LegionPortal115
	if len(p) < 46 || len(p) > 61 {
		return r, fmt.Errorf("invalid legion direct-move size")
	}
	r.Dungeon = binary.LittleEndian.Uint32(p[13:])
	r.Difficulty = binary.LittleEndian.Uint32(p[17:])
	for i := range r.TargetGrid {
		r.TargetGrid[i] = int32(binary.LittleEndian.Uint32(p[21+4*i:]))
	}
	for i := range r.SpawnWindow {
		r.SpawnWindow[i] = int32(binary.LittleEndian.Uint32(p[29+4*i:]))
	}
	r.Mode = p[45]
	if r.Dungeon == 0 || r.Difficulty > 4 || columnZero && r.TargetGrid[0] != 0 {
		return r, fmt.Errorf("unsupported legion direct-move fields")
	}
	if !columnZero {
		for _, coordinate := range r.TargetGrid {
			if coordinate < 0 || coordinate > 255 {
				return r, fmt.Errorf("invalid native Bakal target grid")
			}
		}
	}
	for _, v := range p[46:] {
		if v != 0 {
			return r, fmt.Errorf("nonzero legion direct-move padding")
		}
	}
	return r, nil
}
