package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/npcpresence"
	"dfolan/internal/quest"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"os"
	"time"
)

// npcPresenceShadow is read-only and opt-in. Persisted quest membership does
// not establish native phase/event/visibility caches, so those inputs stay
// unknown. No interaction decision or outgoing packet uses this result.
func (w *worldSession) npcPresenceShadow(p []byte) map[string]any {
	if os.Getenv("DFO_NPC_PRESENCE_DIAGNOSTICS") != "1" || w == nil || w.service == nil || w.quests == nil || w.quests.Store == nil || w.role.ID == 0 || w.activeDungeon != nil || len(p) != 16 || binary.LittleEndian.Uint16(p) != 33 {
		return nil
	}
	for _, v := range p[4:] {
		if v != 0 {
			return nil
		}
	}
	qid := binary.LittleEndian.Uint16(p[2:])
	d, found := w.quests.Catalog.Quests[uint32(qid)]
	if !found {
		return nil
	}
	var npc uint32
	if d.Kind == "[meet npc]" && len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Type == 0 && d.ObjectiveCells[0].Value > 0 {
		npc = uint32(d.ObjectiveCells[0].Value)
	}
	if d.Kind == "[reach the range]" {
		if r, ok := quest.ReachNPCObjective(d); ok {
			npc = r.NPC
		}
	}
	if npc == 0 {
		return nil
	}
	entry := map[string]any{"kind": "quest_npc_presence_shadow", "character_id": w.role.ID, "quest": qid, "npc": npc, "town": w.state.Position.Town, "area": w.state.Position.Area, "mode": "diagnostic_only", "snapshot_timing": "after CMD33 handling, before its response packets"}
	if w.npcPresenceIndex == nil && w.npcPresenceIndexErr == nil {
		world := w.service.Catalog
		if file := os.Getenv("DFO_NPC_PRESENCE_WORLD"); file != "" {
			world, w.npcPresenceIndexErr = catalog.LoadWorld(file)
			if w.npcPresenceIndexErr == nil && world.Source.Checksum != w.service.Catalog.Source.Checksum {
				entry["error"] = "shadow world source differs from active world"
				return entry
			}
		}
		if w.npcPresenceIndexErr == nil {
			w.npcPresenceIndex, w.npcPresenceIndexErr = npcpresence.NewIndex(world, w.quests.Catalog)
		}
	}
	if w.npcPresenceIndexErr != nil {
		entry["error"] = w.npcPresenceIndexErr.Error()
		return entry
	}
	accepted, completed := npcpresence.QuestSet{}, npcpresence.QuestSet{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	states, err := w.quests.Store.Quests(ctx, w.role.AccountID, w.role.ID)
	cancel()
	if err != nil {
		entry["snapshot_error"] = err.Error()
	} else {
		accepted = npcpresence.QuestSet{Known: true, IDs: make(map[uint32]bool)}
		completed = npcpresence.QuestSet{Known: true, IDs: make(map[uint32]bool)}
		for _, s := range states {
			if s.Status != "accepted" && s.Status != "completed" {
				continue
			}
			if s.ConfigVersion != savecontract.Identity() {
				accepted, completed = npcpresence.QuestSet{}, npcpresence.QuestSet{}
				entry["snapshot_error"] = "quest persistence source mismatch"
				break
			}
			if s.Status == "accepted" {
				accepted.IDs[uint32(s.ID)] = true
			} else {
				completed.IDs[uint32(s.ID)] = true
			}
		}
	}
	index := w.npcPresenceIndex
	entry["source"] = index.Source
	entry["target_accepted"] = accepted.Has(uint32(qid))
	entry["result"] = index.ResolveNPC(npcpresence.Query{Town: w.state.Position.Town, Area: w.state.Position.Area, NPC: npc, Accepted: accepted, Completed: completed})
	entry["source_candidates"] = index.Candidates(w.state.Position.Town, w.state.Position.Area, npc, accepted, completed)
	entry["visibility_sources"] = index.VisibilitySources(npc)
	entry["gaps"] = []string{"client phase/cache operation order is not observed", "final map/instance lifecycle is not observed", "visibility/override batch history is not observed", "metadata/unit/fallback rank state is not observed"}
	return entry
}
