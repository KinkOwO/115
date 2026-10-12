package protocol

import (
	"encoding/binary"
	"fmt"
)

// UserStateMotionRequest is CMD623, emitted when an actor's motion changes.
// The native sender 145BEC5E5..145BEC6F8 writes the same seven-byte position
// prefix as CMD35 followed by action, action parameter and three reserved bytes.
// Personal town rooms use this report even when they do not emit periodic CMD35.
type UserStateMotionRequest struct {
	PositionRequest
	Action          byte
	ActionParameter uint16
}

func DecodeUserStateMotionRequest(p []byte) (UserStateMotionRequest, error) {
	var r UserStateMotionRequest
	if len(p) < 13 {
		return r, fmt.Errorf("short user state motion request")
	}
	if e := padding(p[13:], 8); e != nil {
		return r, e
	}
	if e := padding(p[10:13], 4); e != nil {
		return r, e
	}
	r.X = binary.LittleEndian.Uint16(p)
	r.Y = binary.LittleEndian.Uint16(p[2:])
	r.Motion = p[4]
	r.Speed = binary.LittleEndian.Uint16(p[5:])
	r.Action = p[7]
	r.ActionParameter = binary.LittleEndian.Uint16(p[8:])
	return r, nil
}
