package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

type skillSession struct {
	nonce           [16]byte
	initialized     bool
	commandSequence uint64
}

func (s *skillSession) saveCommands(cs *character.Service, w *worldSession, p []byte) (int, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return 0, fmt.Errorf("skill commands require owned selected character")
	}
	req, err := protocol.DecodeSkillCommands(p)
	if err != nil {
		return 0, err
	}
	if !s.initialized {
		if _, err = rand.Read(s.nonce[:]); err != nil {
			return 0, err
		}
		s.initialized = true
	}
	s.commandSequence++
	key := fmt.Sprintf("skill-commands-v1:%x:%d", s.nonce, s.commandSequence)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := cs.SaveSkillCommands(ctx, w.role, key, req)
	if err != nil {
		return 0, err
	}
	w.role = saved
	return len(req.Entries), nil
}

// skillTreeRefreshRequired reports whether the id19 full skill-tree restore has
// to follow this mutation.
//
// A VP apply (CMD29 carrying variation slots) must not: its own response
// already carries the VP block, while id19 has none — the client overwrites the
// panel it just rendered with an empty variation state, which reads as "Apply
// silently reset my VP choices". CMD28 moves are already applied locally by
// the client; an id19 after their ACK also plays the learn sound and clears the
// displayed VP choices. Other mutations keep the restore to refresh the palette
// or return the client to stored state after a refused/idempotent request.
func skillTreeRefreshRequired(id uint16, applied, varied bool) bool {
	if id == 28 {
		return false
	}
	if !applied {
		return true
	}
	return !(id == 29 && varied)
}

func skillMutationResponsePlan(cs *character.Service, saved storage.Character, id uint16, body []byte, applied, varied bool) ([]outboundPacket, error) {
	plan := []outboundPacket{{"skill_committed_response", 1, id, body}}
	if skillTreeRefreshRequired(id, applied, varied) {
		restore, err := cs.EntrySkills(saved)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"skill_state_restored", 0, 19, restore})
	}
	return plan, nil
}

func (s *skillSession) handle(cs *character.Service, w *worldSession, id uint16, p, raw []byte) ([]outboundPacket, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("skill request requires owned selected character")
	}
	if !s.initialized {
		if _, e := rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	h := sha256.Sum256(raw)
	key := fmt.Sprintf("skill:%x:%x", s.nonce, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var saved storage.Character
	var applied bool
	var varied bool
	var body []byte
	var e error
	switch id {
	case 28:
		var r protocol.SkillMove
		r, e = protocol.DecodeSkillMove(p)
		if e == nil {
			saved, applied, e = cs.MoveSkill(ctx, w.role, key, r)
		}
		if e == nil {
			body = protocol.SkillMoveSuccess(r)
		}
	case 29:
		var r protocol.SkillPurchase
		r, e = protocol.DecodeSkillPurchase(p)
		if e != nil {
			break
		}
		varied = r.Intensions != nil || r.Options != nil
		saved, applied, e = cs.Learn(ctx, w.role, key, r)
		if e == nil {
			body, e = cs.LearningResponse(saved, r)
		}
	case 2179:
		// CHANGE_SKILLSLOT_TOTAL: the 自动加点 shortcut-bar layout. The client
		// sends this swap list right after the auto-set learn burst and waits
		// for the acknowledgement; with no handler it stayed an unimplemented
		// sample (zero response), so the bar kept the server's own order and the
		// preview the player confirmed was never applied. skillTreeRefreshRequired
		// is true for this id, so the acknowledgement is followed by the full
		// NOTI 19 redraw.
		var r protocol.SkillSlotTotal
		r, e = protocol.DecodeSkillSlotTotal(p)
		if e == nil {
			saved, applied, e = cs.MoveSkillTotal(ctx, w.role, key, r)
		}
		if e == nil {
			body = protocol.SkillSlotTotalSuccess(r)
		}
	case 483:
		// Two request shapes share this command. The Skill Reset window's
		// Confirm frame is at least 8 bytes with a (style, mask) layout:
		// p[0]=style (0/1), p[1]==0, p[2]=mask restricted to the defined bits.
		// Everything else is the Reset/Auto Set button's opaque body (reading
		// it as a (tree,mask) pair yielded tree 111 / mask 40) and clears all
		// three groups of the main tree, letting the client lay out its own
		// recommended shortcuts as it does natively.
		if len(p) >= 8 && p[1] == 0 && p[0] <= 1 && p[2]&^(character.ResetOrdinarySkills|character.ResetEnhance|character.ResetEvolve) == 0 {
			style, mask := p[0], p[2]
			saved, _, e = cs.ResetSkills(ctx, w.role, key, style, mask)
			if e != nil {
				break
			}
			w.role = saved
			restore, e := cs.EntrySkills(saved)
			if e != nil {
				return nil, e
			}
			body, e = cs.ResetResponse(saved, style)
			if e != nil {
				return nil, e
			}
			// Reset window responses always lead with the full skill tree and
			// close with the variation frame; the open Evolve/Enhance panel
			// renders the last variation frame it receives.
			return []outboundPacket{
				{"skill_state_restored", 0, 19, restore},
				{"skill_variation_reset_response", 1, 29, body},
			}, nil
		}
		if len(p) < 3 {
			return nil, fmt.Errorf("short reset request")
		}
		saved, e = cs.ResetAutoSet(ctx, w.role, key, 0, 7)
		if e != nil {
			return nil, e
		}
		w.role = saved
		restore, e := cs.EntrySkills(saved)
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{{"skill_state_restored", 0, 19, restore}}
		variation, e := cs.VariationRestore(saved)
		if e != nil {
			return nil, e
		}
		if len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
		}
		return plan, nil
	case 2347:
		// Chain / skill preset reset. The server stores no extra preset data:
		// echo the current full skill tree (id19) and variation frame (id29)
		// with zero state mutation. Omitting id29 would make the client render
		// Enhance/Evolve/VP as cleared until the next login.
		saved = w.role
		restore, e := cs.EntrySkills(saved)
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{{"skill_state_restored", 0, 19, restore}}
		variation, e := cs.VariationRestore(saved)
		if e != nil {
			return nil, e
		}
		if len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_chain_response", 1, 29, variation})
		}
		return plan, nil
	default:
		return nil, fmt.Errorf("unsupported skill mutation")
	}
	if e != nil {
		return nil, e
	}
	w.role = saved
	return skillMutationResponsePlan(cs, saved, id, body, applied, varied)
}
