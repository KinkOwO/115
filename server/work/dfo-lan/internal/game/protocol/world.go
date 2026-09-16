package protocol

import (
	"encoding/binary"
	"fmt"
)

type PositionRequest struct {
	X, Y   uint16
	Motion byte
	Speed  uint16
}

func DecodePositionRequest(p []byte) (PositionRequest, error) {
	var r PositionRequest
	if len(p) < 7 {
		return r, fmt.Errorf("short position request")
	}
	if e := padding(p[7:], 8); e != nil {
		return r, e
	}
	r.X = binary.LittleEndian.Uint16(p)
	r.Y = binary.LittleEndian.Uint16(p[2:])
	r.Motion = p[4]
	r.Speed = binary.LittleEndian.Uint16(p[5:])
	return r, nil
}

// AreaChangeRequest is the current native 146d07599..146d07676 sequence.
// The three flag meanings remain separate from destination authorization.
type AreaChangeRequest struct {
	Town, Area   uint32
	X, Y         uint16
	Flag         byte
	PreviousTown uint32
	PreviousArea uint16
	TailFlags    [2]byte
}

func DecodeAreaChangeRequest(p []byte) (AreaChangeRequest, error) {
	var r AreaChangeRequest
	if len(p) < 21 {
		return r, fmt.Errorf("short area request")
	}
	if e := padding(p[21:], 8); e != nil {
		return r, e
	}
	r.Town = binary.LittleEndian.Uint32(p)
	r.Area = binary.LittleEndian.Uint32(p[4:])
	r.X = binary.LittleEndian.Uint16(p[8:])
	r.Y = binary.LittleEndian.Uint16(p[10:])
	r.Flag = p[12]
	r.PreviousTown = binary.LittleEndian.Uint32(p[13:])
	r.PreviousArea = binary.LittleEndian.Uint16(p[17:])
	copy(r.TailFlags[:], p[19:21])
	return r, nil
}

// UserArea is NOTI23 (145311b76..145311bba). It updates a known actor's
// placement; destination map loading for the local actor also uses NOTI24.
func UserArea(town, area uint32, user AreaUser) ([]byte, error) {
	if user.ActorServerID == 0 || user.ActorServerID == 65535 {
		return nil, fmt.Errorf("invalid actor identity")
	}
	p := add32(add32(add16(nil, user.ActorServerID), town), area)
	p = add16(add16(p, user.X), user.Y)
	return append(p, user.Flags[0], user.Flags[1]), nil
}

// SET_USER_AREA success handler 145296b40 takes its early return when success
// is nonzero. A failure additionally reads the destination town/area.
func AreaChangeSuccess() []byte { return []byte{1} }

func AreaChangeFailure(code uint16, town, area uint32) ([]byte, error) {
	if code == 0 {
		return nil, fmt.Errorf("missing native area refusal code")
	}
	return add32(add32(add16([]byte{0}, code), town), area), nil
}
