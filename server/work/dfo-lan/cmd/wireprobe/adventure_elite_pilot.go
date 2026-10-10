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

// Owner-authorized deviation from PVF's native elite-content binding. Disabled
// unless the isolated candidate profile opts in. Dungeon membership is still
// read from the prepared PVF definition, never a parallel list of dungeon IDs.
func (w *worldSession) attunementEliteEntryPackets(ctx context.Context, sel protocol.DungeonSelection, s *dungeon.Session) ([]outboundPacket, error) {
	if os.Getenv("DFO_ELITE_COMPANION_PILOT") == "all" {
		return w.allDungeonEliteEntryPackets(ctx, sel, s)
	}
	if os.Getenv("DFO_ELITE_COMPANION_PILOT") != "attunement" || w == nil || s == nil ||
		s.Definition.Type != "boundary of attunement" || sel.ID != s.Definition.ID {
		return nil, nil
	}
	// Explicit owner-selected source namespace, shared with the build-time
	// client binding. Other abyss generations are outside this one pilot.
	path := strings.ToLower(strings.ReplaceAll(s.Definition.Script.Path, "\\", "/"))
	if !strings.HasPrefix(path, "contents/2026/skyofathousandseasofborder/dungeon/") {
		return nil, nil
	}
	// Native special-channel loaders keep ownership of their own lifecycle.
	if w.channelWorldIsolated || adventureEliteChannel(w.channelType) || sel.Party != 65535 {
		return nil, nil
	}
	if w.store == nil || w.characters == nil {
		return nil, fmt.Errorf("调律同伴测试缺少账号或角色服务")
	}
	// Only the already-validated initial/re-entry plan calls this adapter.
	// C1811 and mid-battle roster edits retain their native channel/busy gates.
	packets, err := w.adventureEliteSnapshotPackets(ctx, 2)
	if err != nil {
		return nil, fmt.Errorf("调律同伴测试：%w", err)
	}
	log.Printf("attunement companion prepared: dungeon=%d source=%s party=%d packets=%d", sel.ID, path, sel.Party, len(packets))
	for i := range packets {
		packets[i].Name = "attunement_pilot_" + packets[i].Name
	}
	return packets, nil
}
