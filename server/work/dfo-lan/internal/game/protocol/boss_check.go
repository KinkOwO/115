package protocol

import (
	"encoding/binary"
	"fmt"
)

type BossCheckRequest struct {
	Actor, Target uint16
	Check         uint32
}

func DecodeBossCheck(p []byte) (BossCheckRequest, error) {
	// Current x64 writer145c34f4d..145c34fc6 writes12 bytes, encrypted to16.
	// The90 reference writes39 bytes and is not wire compatible here.
	if len(p) != 16 {
		return BossCheckRequest{}, fmt.Errorf("boss check requires12 bytes padded to16")
	}
	if err := padding(p[12:], 16); err != nil {
		return BossCheckRequest{}, err
	}
	r := BossCheckRequest{Actor: binary.LittleEndian.Uint16(p), Target: binary.LittleEndian.Uint16(p[2:]), Check: binary.LittleEndian.Uint32(p[8:])}
	if r.Actor == 0 || r.Actor == 65535 || r.Target == 0 || r.Target == 65535 || binary.LittleEndian.Uint32(p[4:]) != 0 {
		return BossCheckRequest{}, fmt.Errorf("invalid boss check identity or reserved field")
	}
	return r, nil
}

func BossCheckConfirmed(target uint16) ([]byte, error) {
	if target == 0 || target == 65535 {
		return nil, fmt.Errorf("invalid boss identity")
	}
	return add16([]byte{1, 1}, target), nil // NOTI1151452a5810, not CMD117.
}

func DungeonClearEnabled() []byte { return make([]byte, 4) } // NOTI31 native1452ae550.
