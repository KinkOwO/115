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

// IspinsBossCheckConfirmed builds the 16B NOTI115 used in the Ispins stage
// settlement chain (next79 §24/§26). The official s4 frame 495 body is
// `01 01 <u16 target> <5B token> <7B zero>`; the generic 4B
// BossCheckConfirmed form is rejected by the client in legion context
// (op=682 crash 1.2s after receipt). CMD117/N115 only appears on stage0
// (c2s 帧 337) and stage3 (c2s 帧 488) in the official run — stage1/2 return
// a nil body and the caller must not send NOTI115. The 5B token is the
// per-stage nonce observed on the official N115, replayed verbatim.
func IspinsBossCheckConfirmed(target uint16, stage int) ([]byte, error) {
	if target == 0 || target == 65535 {
		return nil, fmt.Errorf("invalid boss identity")
	}
	if stage != 0 && stage != 3 {
		return nil, nil
	}
	var token [5]byte
	if stage == 0 {
		token = [5]byte{0xaa, 0x53, 0x06, 0x2f, 0x42} // 官服 s4 帧 495 stage0
	} else {
		token = [5]byte{0xed, 0x9b, 0x7c, 0xfd, 0x35} // 官服 s4 stage3
	}
	p := make([]byte, 16)
	p[0], p[1] = 1, 1
	binary.LittleEndian.PutUint16(p[2:], target)
	copy(p[4:9], token[:])
	return p, nil
}

func DungeonClearEnabled() []byte { return make([]byte, 4) } // NOTI31 native1452ae550.
