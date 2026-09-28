package protocol

import (
	"encoding/binary"
	"fmt"
)

// 本文件是「军团组队名册」在补丁里的**自包含底座**。
//
// 捐赠者主线里这三个符号位于与上游同名的内部文件（party.go / conquest_party115.go），
// 但两边内容不同、上游没有它们；为了让这份补丁在上游能直接编译，这里作为新文件提供：
//
//   - PartyCreateOptions115：CMD12 原生通用八槽写入器字段（不是授权令牌，也不是推断出的副本身份）。
//   - PartyInfoType：N9 名册头第二个 u16。
//   - PartyRosterSeats115 / applySpecialPartyOptions115：最小名册语法与「专用模式」扩展写入。
//
// 若上游将来引入同名实现，请删掉本文件里的副本，只保留调用。

// These are the native generic eight-slot CMD12 writer fields, not an
// authorization token or inferred dungeon identity. Preserve separate numeric
// names until the target mode's consumer is bound. Short four-slot variants
// must not be silently decoded with this layout.
type PartyCreateOptions115 struct {
	Action       byte    `json:"action"`
	Reserved     byte    `json:"reserved"`
	Title        string  `json:"title"`
	Capacity     byte    `json:"capacity"`
	Info         uint32  `json:"info"`
	Byte5        byte    `json:"byte5"`
	Word6        uint16  `json:"word6"`
	Byte8        byte    `json:"byte8"`
	Mode         byte    `json:"mode"`
	ModeValue    uint16  `json:"mode_value"`
	SlotFilters  [8]byte `json:"slot_filters"`
	Field20      uint32  `json:"field20"`
	Selection    uint32  `json:"selection"`
	Variant      byte    `json:"variant"`
	Extra        byte    `json:"extra"`
	LogicalBytes int     `json:"logical_bytes"`
}

// PartyInfoType 是 N9 名册头的第二个 u16（当前客户端读流实证；0x270F = 9999）。
const PartyInfoType uint16 = 0x270f

// N9 tail+3 writes party+0x50, the native leader slot (1452F43C3/4931).
// Omitted slots become FFFF in the reader; never compact survivors on leave.
func PartyRosterSeats115(partyID uint16, context [2]byte, seats [4]uint16, leader uint16, capacity byte) ([]byte, error) {
	var actors []uint16
	leaderSeat := -1
	for slot, actor := range seats {
		if actor == 0 {
			continue
		}
		actors = append(actors, actor)
		if actor == leader {
			leaderSeat = slot
		}
	}
	if leaderSeat < 0 {
		return nil, fmt.Errorf("party leader seat missing")
	}
	if partyID == 0 || partyID == 65535 {
		return nil, fmt.Errorf("invalid local party id")
	}
	if len(actors) == 0 || len(actors) > int(capacity) || capacity > 4 {
		return nil, fmt.Errorf("invalid ordinary party capacity")
	}
	seen := make(map[uint16]bool, len(actors))
	for _, actor := range actors {
		if actor == 0 || actor == 65535 || seen[actor] {
			return nil, fmt.Errorf("invalid party actor")
		}
		seen[actor] = true
	}
	p := make([]byte, 90+17*len(actors))
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], PartyInfoType)
	binary.LittleEndian.PutUint16(p[4:], partyID)
	copy(p[6:8], context[:])
	p[20] = capacity
	p[69] = byte(len(actors))
	row := 0
	for slot, actor := range seats {
		if actor == 0 {
			continue
		}
		at := 70 + row*17
		p[at] = byte(slot)
		binary.LittleEndian.PutUint16(p[at+1:], actor)
		row++
	}
	p[70+17*len(actors)+3] = byte(leaderSeat)
	return p, nil
}

// Both N9 mode stores must survive the full-info read. Shared layout only;
// each content supplies its independently verified extension afterwards.
func applySpecialPartyOptions115(base []byte, options PartyCreateOptions115, route byte) []byte {
	p := base
	p[20] = options.Capacity
	binary.LittleEndian.PutUint32(p[21:25], options.Info)
	p[25], p[28] = options.Byte5, options.Byte8
	binary.LittleEndian.PutUint16(p[26:28], options.Word6)
	p[29] = options.Mode
	binary.LittleEndian.PutUint16(p[30:32], options.ModeValue)
	copy(p[46:54], options.SlotFilters[:])
	binary.LittleEndian.PutUint32(p[60:64], options.Selection)
	p[64] = options.Variant
	// G0260: 1452F4413 reads a SECOND mode and 1452F44C5 writes it to
	// PartyData+122 again. Leaving the ordinary tail at zero undoes p[29].
	// This offset is for PartyRosterSeats115's empty-title/minimal-row grammar:
	// 78+17*N == base length-12. Set it BEFORE appending route-specific data.
	p[len(base)-12] = options.Mode
	p[len(p)-1] = route
	return p
}
