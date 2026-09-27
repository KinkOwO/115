package main

import (
	"dfolan/internal/character"
	"dfolan/internal/storage"
)

// appendSkillPresetRestore keeps the native skill-chain object alive across a
// NOTI19 rebuild. NOTI19 reconstructs the client's learned-skill objects, so a
// saved chain must be re-applied through NOTI2758 immediately afterwards.
func appendSkillPresetRestore(plan []outboundPacket, service *character.Service, role storage.Character, name string) ([]outboundPacket, error) {
	preset, err := service.SkillPresetInfo(role)
	if err != nil {
		return nil, err
	}
	if len(preset) > 0 {
		plan = append(plan, outboundPacket{name, 0, 2758, preset})
	}
	return plan, nil
}
