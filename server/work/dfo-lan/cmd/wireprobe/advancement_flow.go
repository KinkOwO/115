package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// advancementRefusalCode maps a refusal reason to the native CMD1881 error
// code the response handler 0x14524fd70 turns into a DSTR message:
// 7=only in town, 107=advancement info error, 217=selection error,
// 235=this character can't advance.
func advancementRefusalCode(err error) uint16 {
	if err == nil {
		return 0
	}
	switch msg := err.Error(); {
	case strings.Contains(msg, "town character"):
		return 7
	case strings.Contains(msg, "source mismatch"):
		return 107
	case strings.Contains(msg, "cannot advance to that branch"),
		strings.Contains(msg, "invalid advancement target"),
		strings.Contains(msg, "base profession"),
		strings.Contains(msg, "request length"),
		strings.Contains(msg, "padding"):
		return 217
	default:
		return 235
	}
}

// changeGrowType handles the in-town job-change family: CMD1881
// (ENUM_CMDPACKET_CHANGE_GROW_TYPE, first advancement base->job) and CMD777
// (ENUM_CMDPACKET_RE_GROWUP_CHANGE, the free job->job change). Both share the
// 14-byte body with the target grow-type at byte 13 and the [result][code]
// response; only the response opcode differs. It mirrors awakenCharacter:
// persist the new advancement, then rebuild the actor presentation (basic
// probe, job-info, attributes, equipment, skills, variations) so the client's
// character object already carries the new packed grow-type before the success
// response triggers the display refresh (0x14524fd70 / 0x1452aff30).
func changeGrowType(service *character.Service, w *worldSession, p, keys []byte, responseID uint16) ([]preparedPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("advancement requires town character")
	}
	advancement, err := protocol.DecodeChangeGrowType(p)
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
		var st character.State
		if e := json.Unmarshal(role.State, &st); e != nil {
			return nil, e
		}
		growType, e := st.WireAdvancement()
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{
			{"advancement_character_updated", 0, 2, basic},
			// ENUM_NOTIPACKET_GROWTYPE_CHANGE_JOB_INFO (718): handler
			// 0x1453072b0 stores the one-byte packed grow-type into the
			// job/grow-type manager the success refresh reads back.
			{"advancement_job_info", 0, 718, []byte{growType}},
		}
		plan = append(plan, restored...)
		plan = append(plan, outboundPacket{"advancement_skills_updated", 0, 19, skills})
		plan, e = appendSkillPresetRestore(plan, service, role, "skill_preset_restored_after_advancement")
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"advancement_completed", 1, responseID, protocol.ChangeGrowTypeSuccess()})
		return preparePackets(keys, plan)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A job change grants no level-dependent reward, so a unique per-request
	// key is safe: a retransmitted body re-runs apply, which is a domain no-op
	// once the character already holds the target branch. The opcode keeps the
	// 1881 (initial) and 777 (re-change) event streams distinct.
	key := fmt.Sprintf("change-grow-type-v1:%d:%x", responseID, sha256.Sum256(p))
	saved, _, err := service.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "change-grow-type-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := service.ApplyAdvancement(current, advancement)
		if e != nil {
			return nil, nil, e
		}
		current.State = raw
		if _, e = planFor(current); e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]byte{"advancement": advancement})
		return raw, receipt, nil
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return planFor(saved)
}
