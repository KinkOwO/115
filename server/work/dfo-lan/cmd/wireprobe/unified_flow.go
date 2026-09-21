package main

import (
	"dfolan/internal/game/protocol"
)

// unifiedCharacPayload builds the NOTI2827 payload that restores the locked
// skills: the client's own 3539 byte character option block with its two 386
// byte lock objects (subtype 19 at 2736, subtype 20 at 3122) filled in. Every
// other byte stays at the client default.
//
// The built-in block is the same version as the client, so the file template and
// the offset override only matter for a build whose layout differs.
func unifiedCharacPayload(template []byte, locks []uint16, skillLockAt int) ([]byte, error) {
	if len(template) == 0 && skillLockAt < 0 {
		return protocol.UnifiedCharacOptions(locks)
	}
	if len(template) == 0 {
		template = protocol.CharacOptionsTemplate()
	}
	if skillLockAt < 0 {
		skillLockAt = protocol.UnifiedCharacSkillLockAt
	}
	return protocol.UnifiedCharacOptionsFrom(template, locks, skillLockAt)
}
