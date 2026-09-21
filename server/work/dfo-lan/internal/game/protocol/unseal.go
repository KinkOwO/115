package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD393 UNSEAL_RANDOM_OPTION. The native sender at 145ae9ae0 writes the
// target inventory slot and the unseal-scroll slot (0xFFFF when no scroll is
// used), exactly what live captures show: 0a00 ffff followed by zero cipher
// padding. The response follows the shared command convention the client's
// receive loop consumes for every command: one status byte, then a u16 error
// code only when the status is failure. Success carries nothing further.
type UnsealRequest struct {
	TargetSlot uint16
	ScrollSlot uint16
}

const UnsealNoScrollSlot = 0xFFFF

// Client-visible refusal codes, matching the message table the native client
// shows when unsealing fails (invalid target, insufficient gold, unsupported
// item).
const (
	UnsealRefusedInvalidTarget    uint16 = 4
	UnsealRefusedInsufficientGold uint16 = 10
	UnsealRefusedUnsupported      uint16 = 13
)

func DecodeUnseal(p []byte) (UnsealRequest, error) {
	var r UnsealRequest
	if len(p) < 4 || len(p) > 19 {
		return r, fmt.Errorf("unseal requires 4 bytes with cipher block padding")
	}
	for _, b := range p[4:] {
		if b != 0 {
			return r, fmt.Errorf("unseal padding")
		}
	}
	r = UnsealRequest{TargetSlot: binary.LittleEndian.Uint16(p), ScrollSlot: binary.LittleEndian.Uint16(p[2:])}
	if r.TargetSlot == 0xFFFF {
		return r, fmt.Errorf("invalid unseal target slot")
	}
	return r, nil
}

// UnsealSuccess is the bare success status byte; the client's generic
// response reader consumes exactly one byte when the status is nonzero.
func UnsealSuccess() []byte { return []byte{1} }

// UnsealRefused is status 0 plus the u16 reason the client displays.
func UnsealRefused(code uint16) []byte { return Refusal(code) }
