package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"fmt"
)

func validateMoonResources(c *catalog.DungeonCatalog, s *loot.Service) error {
	if c == nil || s == nil || s.Store == nil || s.Equipment == nil || c.Source.Checksum != s.Catalog.Source.Checksum {
		return fmt.Errorf("Moon needs same-source dungeon, loot, equipment and persistent rewards")
	}
	if _, e := dungeon.MoonInitialProgress(*c); e != nil {
		return e
	}
	if _, e := dungeon.SelectMoon(*c, protocol.DungeonSelection{ID: 100004136, Party: 65535}, 255, nil, 0); e != nil {
		return e
	}
	if _, e := dungeon.Select(*c, protocol.DungeonSelection{ID: 100004137, Party: 65535}, 255, nil); e != nil {
		return e
	}
	drop := s.DropCatalog
	if len(drop.Items) == 0 {
		drop = s.Catalog
	}
	if drop.Source.Checksum != c.Source.Checksum {
		return fmt.Errorf("Moon drop source mismatch")
	}
	// Fail BEFORE entry, not at the first level145 kill. Source export must
	// include all monster levels, not the upstream level20 demonstration slice.
	for level := byte(1); level <= 150; level++ {
		if uint32(level) != c.Dungeons[100004136].BasisLevel {
			continue
		}
		for rank := byte(0); rank < 4; rank++ {
			if _, e := loot.Roll(drop, s.Tables, s.Rules, s.Equipment.DropPool(), 1, level, rank, 0); e != nil {
				return fmt.Errorf("Moon drop level%d rank%d: %w", level, rank, e)
			}
		}
	}
	if c.Dungeons[100004136].BasisLevel == 0 || c.Dungeons[100004136].BasisLevel > 150 {
		return fmt.Errorf("unsupported Moon source combat level")
	}
	return nil
}

// Shape-specific refusals only. Never use a generic short ACK for readers
// which unconditionally consume an entire card/exit structure.
func moonRefusal(id uint16, p []byte) []outboundPacket {
	switch id {
	case 71, 1426:
		body, _ := protocol.CardSelected(-1)
		return []outboundPacket{{"moon_card_refused", 1, 71, body}}
	case 72:
		req, e := protocol.DecodeSettlementExit(p)
		if e == nil {
			return []outboundPacket{{"moon_exit_refused", 1, 72, protocol.SettlementExitRefused(req.Option)}}
		}
	case 2284:
		return []outboundPacket{{"moon_start_refused", 1, 2284, protocol.SemiRaidStartReply115(false)}}
	}
	return nil
}
