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

// UserPosition is NOTI 22, the movement update for an actor the client already
// knows. Recovered from the current client's own handler at 0x145312610, which
// reads +0 u16 actor, +2 u16 x, +4 u16 y, +6 u8 motion, +7 u16 speed: nine
// bytes total, i.e. the actor id followed by exactly the seven-byte body of the
// client's own CMD 35 position report (X u16, Y u16, Motion u8, Speed u16).
//
// This is the notification that drives smooth movement. NOTI 23 only places an
// actor at a new coordinate and reads as a teleport, which is why forwarding
// positions with it looked like one jump per second.
func UserPosition(actor, x, y uint16, motion byte, speed uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid actor identity")
	}
	p := add16(add16(add16(nil, actor), x), y)
	p = append(p, motion)
	return add16(p, speed), nil
}

// UserLeave is NOTI 6, which removes an actor from the scene of everyone who
// can see it. Recovered from handler 0x145312170: the entire body is one u16
// actor id.
//
// This is the notification that actually deletes an actor. NOTI 24 only places
// actors and never removes one, so a departure sent as a shorter area list
// leaves the actor standing there forever.
func UserLeave(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid actor identity")
	}
	return add16(nil, actor), nil
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
