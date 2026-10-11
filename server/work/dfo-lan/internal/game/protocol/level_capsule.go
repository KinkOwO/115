package protocol

import "fmt"

// JumpLevelCapsuleAction is the native CMD507 action captured twice on
// 2026-10-06: slot 66, list 0, action 160, zero remaining operands (64 bytes).
const JumpLevelCapsuleAction uint32 = 160

func DecodeJumpLevelCapsule(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if action != JumpLevelCapsuleAction || p[2] != 0 {
		return 0, fmt.Errorf("unsupported jump-level capsule action/list")
	}
	return slot, nil
}
