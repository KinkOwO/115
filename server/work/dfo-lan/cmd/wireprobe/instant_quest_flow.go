package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
)

// instantQuestPlan publishes the final state once, after the dispatcher sends
// ACK1422. Synthesized quest rows have no live client quest object, so they must
// not receive terminal CMD34 replies. Experience and gold stay unchanged; the
// bag is restored only when source-authorized rewards or key items moved it.
func (w *worldSession) instantQuestPlan(ctx context.Context, result quest.SkipResult) ([]outboundPacket, error) {
	if len(result.Settled) == 0 && len(result.KeyItems) == 0 {
		return nil, nil
	}
	// The role after everything the sweep wrote, key items included: the tail
	// packets describe the character the client is about to see, not the one the
	// last quest left behind.
	role := result.Role
	role.WireID = w.role.WireID
	var plan []outboundPacket
	// The key items move the bag even when no quest paid anything, which is the
	// common case: a skip mints no rewards of its own.
	moved := len(result.KeyItems) > 0
	for _, r := range result.Settled {
		if !r.Applied {
			continue
		}
		// Unaccepted quests have no live object for the terminal CMD34 reader.
		// The native force-clear ACK1422 + completed/available snapshots suffice.
		// Do not emit per-quest terminal replies for synthesized rows.
		moved = moved || len(r.Receipt.Items) > 0
	}
	active, e := w.quests.Active(ctx, role)
	if e != nil {
		return nil, e
	}
	done, e := w.quests.Completed(ctx, role)
	if e != nil {
		return nil, e
	}
	triggers, e := protocol.QuestTriggers(active)
	if e != nil {
		return nil, e
	}
	completed, e := protocol.CompletedQuests(done)
	if e != nil {
		return nil, e
	}
	experience, e := character.ExperiencePayload(role)
	if e != nil {
		return nil, e
	}
	if moved {
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, e
		}
		body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"quest_inventory_committed", 0, 13, body})
	}
	// A sweep can open extended equipment slots (the three [slot expansion] main
	// quests); the armoury keeps showing its padlocks until the new unlock byte
	// arrives in a USERINFO1 addition.
	if result.UnlockedEquipment != 0 {
		refresh, e := w.unlockRefresh(role)
		if e != nil {
			return nil, e
		}
		plan = append(plan, refresh...)
	}
	plan = append(plan, outboundPacket{"quest_experience", 0, 37, experience}, outboundPacket{"quest_triggers_updated", 0, 291, triggers}, outboundPacket{"completed_quests_updated", 0, 342, completed})
	w.role, w.level = role, experience[0]
	available, e := w.availableQuestPayload(ctx)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"available_quests_updated", 0, 21, available})
	return plan, nil
}
