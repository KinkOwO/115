package legion

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	DuskyContentID          uint32 = 103
	CmdDuskyOperationSelect uint16 = 2143
	NotiDuskyInfo           uint16 = 2467
	DuskyInfoSize                  = 133
)

// DuskyRequest owns only this family's native content identity. In particular,
// C2143's action at +13 is not a content id. N2467 is a notification, never C2S.
func DuskyRequest(id uint16, p []byte) bool {
	if id == CmdDuskyOperationSelect {
		return true
	}
	switch id {
	case CmdStart, CmdFail, CmdEnterDungeon, CmdRewardEnd:
		return len(p) >= EnvelopeSize+4 && binary.LittleEndian.Uint32(p[EnvelopeSize:]) == DuskyContentID
	}
	return false
}

type DuskySelectionRequest struct {
	Action uint32
	Index  int16
}

// Current US writer 141c292f0 writes exactly envelope13 + action4 + index2.
// Transport padding is allowed but is not interpreted as additional fields.
func DecodeDuskySelection(p []byte) (DuskySelectionRequest, error) {
	if len(p) < EnvelopeSize+6 {
		return DuskySelectionRequest{}, fmt.Errorf("Dusky selection requires 19 bytes")
	}
	r := DuskySelectionRequest{binary.LittleEndian.Uint32(p[13:]), int16(binary.LittleEndian.Uint16(p[17:]))}
	if r.Action != 1 && r.Action != 2 && r.Action != 4 {
		return r, fmt.Errorf("unsupported Dusky selection action %d", r.Action)
	}
	if r.Action == 2 && r.Index < 0 {
		return r, fmt.Errorf("Dusky selection has no operation index")
	}
	return r, nil
}

// Names deliberately retain the native record positions where their gameplay
// meaning is not established. CN packets are not used as US wire templates.
type DuskyStageRecord struct {
	Index         byte
	First, Second int32
}

type DuskyOperationRecord struct {
	Index, Fixed, Value, Grade int16
	Flag, Reserved             byte
}

// DuskyOperationOperand guards the signed-short conversion used by 141c22e60.
// A source magnitude of 1.5 must never silently become 1 or a guessed scale.
func DuskyOperationOperand(value float64) (int16, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) || value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("Dusky source operand %v has no verified int16 representation", value)
	}
	return int16(value), nil
}

type DuskyInfoState struct {
	Prefix                                  uint16
	Selected                                byte
	State, Outcome, Phase, Following, Extra int32
	Stages                                  [3]DuskyStageRecord
	History                                 [3]DuskyOperationRecord
	Offers                                  [4]DuskyOperationRecord
	TailFlag, TailReserved                  byte
	Reselections                            int16
}

// DuskyInfo projects the 133-byte record read by US 141c29080 and copied by
// 141c22e60. Unlike N2895 this record has no extra content-id prefix.
// Construction is separate from authorization to send a lifecycle transition.
func DuskyInfo(s DuskyInfoState) []byte {
	p := make([]byte, DuskyInfoSize)
	binary.LittleEndian.PutUint16(p, s.Prefix)
	p[2] = s.Selected
	for i, v := range []int32{s.State, s.Outcome, s.Phase, s.Following, s.Extra} {
		binary.LittleEndian.PutUint32(p[3+i*4:], uint32(v))
	}
	for i, row := range s.Stages {
		at := 23 + i*12
		p[at] = row.Index
		binary.LittleEndian.PutUint32(p[at+4:], uint32(row.First))
		binary.LittleEndian.PutUint32(p[at+8:], uint32(row.Second))
	}
	put := func(at int, row DuskyOperationRecord) {
		for i, v := range []int16{row.Index, row.Fixed, row.Value, row.Grade} {
			binary.LittleEndian.PutUint16(p[at+i*2:], uint16(v))
		}
		p[at+8], p[at+9] = row.Flag, row.Reserved
	}
	for i, row := range s.History {
		put(59+i*10, row)
	}
	for i, row := range s.Offers {
		put(89+i*10, row)
	}
	p[129], p[130] = s.TailFlag, s.TailReserved
	binary.LittleEndian.PutUint16(p[131:], uint16(s.Reselections))
	return p
}

// US ACK reader 141c28e90 consumes success + 19 bytes. The selection deadline
// is body+12; close is body+7; reselection count is body+18 (not N2467's +131).
func DuskySelectionAck(action uint32, index int16, close bool, deadline uint32, secondary byte, reselections int16) []byte {
	p := make([]byte, 20)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], action)
	binary.LittleEndian.PutUint16(p[5:], uint16(index))
	if close {
		p[7] = 1
	}
	binary.LittleEndian.PutUint32(p[12:], deadline)
	p[16] = secondary
	binary.LittleEndian.PutUint16(p[18:], uint16(reselections))
	return p
}
