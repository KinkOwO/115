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
		plan := []outboundPacket{
			{"awakening_character_updated", 0, 2, basic},
		}
		plan = append(plan, restored...)
		plan = append(plan, []outboundPacket{
			{"awakening_skills_updated", 0, 19, skills},
			{"awakening_completed", 1, 2177, []byte{1}},
		}...)
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
