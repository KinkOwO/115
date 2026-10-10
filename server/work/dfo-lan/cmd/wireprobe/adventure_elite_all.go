package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"log"
	"os"
	"strings"
)

// Owner-requested extension. Membership is the already admitted PVF session,
// not a second dungeon table. This candidate reuses the mode-2 wire adapter.
func (w *worldSession) allDungeonEliteEntryPackets(ctx context.Context, sel protocol.DungeonSelection, s *dungeon.Session) ([]outboundPacket, error) {
	if os.Getenv("DFO_ELITE_COMPANION_PILOT") != "all" || w == nil || s == nil || sel.ID != s.Definition.ID || sel.Party != 65535 {
		return nil, nil
	}
	// The native training/event APC (3) and character-switching mode (4) have
	// different factories. Do not replace their owner/container lifecycles.
	if w.inTutorial || s.Definition.Tutorial || w.boostTrainingAPCRequested || w.channelType == 76 || w.townRobotPartySnapshot() != nil {
		return nil, nil
	}
	if w.dungeons != nil && dungeon.IsTrainingRoom(*w.dungeons, sel.ID) {
		return nil, nil
	}
	path := strings.ToLower(strings.ReplaceAll(s.Definition.Script.Path, "\\", "/"))
	if path == "" || !strings.HasSuffix(path, ".dgn") {
		return nil, fmt.Errorf("精锐扩展缺少已准入副本的PVF定义")
	}
	if w.store == nil || w.characters == nil {
		return nil, fmt.Errorf("精锐扩展缺少账号角色服务")
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	// Empty/deleted/current-character-only selections are ordinary solo entry.
	// Do not abort a stage or re-use clones left over from a previous dungeon.
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	selected := profile.Data.EliteSelections[2]
	valid := false
	for _, role := range roles {
		if role.ID == w.role.ID {
			continue
		}
		for _, id := range selected {
			if id != 0 && id == role.ID {
				valid = true
			}
		}
	}
	if !valid {
		return nil, nil
	}
	packets, err := w.adventureEliteSnapshotPackets(ctx, 2)
	if err != nil {
		return nil, fmt.Errorf("精锐扩展：%w", err)
	}
	log.Printf("elite_all prepared: dungeon=%d source=%s channel=%d packets=%d attempt=1/3", sel.ID, path, w.channelType, len(packets))
	for i := range packets {
		packets[i].Name = "elite_all_" + packets[i].Name
	}
	return packets, nil
}

// Insert only between the audited native N28 and N29 pair. Special entry
// owners (Ispins/Moon) keep their own acknowledgements and stage notifications.
func (w *worldSession) allDungeonEliteEntryPlan(ctx context.Context, s *dungeon.Session, plan []outboundPacket) ([]outboundPacket, error) {
	if os.Getenv("DFO_ELITE_COMPANION_PILOT") != "all" {
		return plan, nil
	}
	if s == nil {
		return nil, fmt.Errorf("精锐扩展缺少副本会话")
	}
	index := -1
	infoSeen := false
	for i, p := range plan {
		if p.Kind != 0 {
			continue
		}
		if p.ID == 28 {
			infoSeen = true
		}
		if p.ID == 29 {
			if infoSeen {
				index = i
			}
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("精锐扩展入场缺少N28/N29顺序")
	}
	packets, err := w.allDungeonEliteEntryPackets(ctx, protocol.DungeonSelection{ID: s.Definition.ID, Party: 65535}, s)
	if err != nil {
		return nil, err
	}
	if len(packets) == 0 {
		return plan, nil
	}
	out := make([]outboundPacket, 0, len(plan)+len(packets))
	out = append(out, plan[:index]...)
	out = append(out, packets...)
	return append(out, plan[index:]...), nil
}
