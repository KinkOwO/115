package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

// orderAwakeningPackets fixes the VP-system opening order: the awakening
// completion frame (id 2177) is the client's switch point for enabling the
// Skill Evolve/Enhance panel, so every VP frame — in practice the id 29
// variation restore — must arrive after it. Any id29 riding inside `restored`
// is pulled out and re-appended last; stage 1/2 builds carry an empty id29
// payload which preparePackets skips.
func orderAwakeningPackets(basic []byte, restored []outboundPacket, skills, preset []byte) []outboundPacket {
	var variation outboundPacket
	var base []outboundPacket
	for _, pkt := range restored {
		if pkt.ID == 29 {
			variation = pkt
		} else {
			base = append(base, pkt)
		}
	}
	plan := []outboundPacket{{"awakening_character_updated", 0, 2, basic}}
	plan = append(plan, base...)
	plan = append(plan,
		outboundPacket{"awakening_skills_updated", 0, 19, skills},
		outboundPacket{"skill_preset_restored_after_awakening", 0, 2758, preset},
		outboundPacket{"awakening_completed", 1, 2177, []byte{1}},
		variation,
	)
	return plan
}

func awakenCharacter(service *character.Service, w *worldSession, p, keys []byte) ([]preparedPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("awakening requires town character")
	}
	stage, err := protocol.DecodeSystemAwakening(p)
	if err != nil {
		return nil, err
	}
	planFor := func(role storage.Character) ([]preparedPacket, error) {
		role.WireID = w.role.WireID
		basic, e := service.EntryBasicProbe(role, [2]byte{})
		if e != nil {
			return nil, e
		}
		skills, e := service.EntrySkills(role)
		if e != nil {
			return nil, e
		}
		restored, e := appearanceRestore(service, role)
		if e != nil {
			return nil, e
		}
		preset, e := service.SkillPresetInfo(role)
		if e != nil {
			return nil, e
		}
		plan := orderAwakeningPackets(basic, restored, skills, preset)
		return preparePackets(keys, plan)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := service.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("awakening-v1:%d", stage), "system-awakening-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := service.ApplyAwakening(current, stage)
		if e != nil {
			return nil, nil, e
		}
		current.State = raw
		if _, e = planFor(current); e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]byte{"stage": stage})
		return raw, receipt, nil
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return planFor(saved)
}
