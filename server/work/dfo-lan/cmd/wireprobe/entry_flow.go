package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"fmt"
	"io"
	"time"
)

type outboundPacket struct {
	Name    string
	Kind    byte
	ID      uint16
	Payload []byte
}
type preparedPacket struct {
	outboundPacket
	Raw []byte
}
type entryPayloads struct {
	Select, Basic, Addition, Skills, Vault, UserArea, Area, Fatigue, Complete []byte
	Experience, CompletedQuests, Inventory                                    []byte
	AvailableQuests                                                           []byte
	Worn                                                                      []byte
	AccountOptions                                                            []byte
	WornUpdate                                                                []byte
	Avatars, AvatarReady, Creatures, CreatureList, CreatureGrowth             []byte
	CinematicSkips                                                            []byte
	SkillVariations                                                           []byte
	OdysseyProgress                                                           []byte
	// Peers carries the USERINFO of every actor already standing in the scene.
	// It is emitted after this actor's own placement but before the area list,
	// because the client only places actors it already knows.
	Peers [][]byte
}

func (p entryPayloads) packets() []outboundPacket {
	out := []outboundPacket{
		{"select_parser_response", 1, 4, p.Select},
		{"account_options_restored", 0, 2826, p.AccountOptions},
		{"cinematic_skips_restored", 0, 1352, p.CinematicSkips},
		{"entry_basic_probe_sent", 0, 2, p.Basic},
		{"entry_addition_sent", 0, 2, p.Addition},
		{"entry_skills_sent", 0, 19, p.Skills},
		{"vault_initialized", 0, 13, p.Vault},
		{"inventory_restored", 0, 13, p.Inventory},
		// Initialize list 1 empty. The client accepts authoritative avatar rows
		// only after the town actor/UserInfo graph has been installed.
		{"avatar_inventory_initialized", 0, 13, p.Avatars},
		{"creature_list_restored", 0, 105, p.CreatureList},
		{"creature_inventory_restored", 0, 13, p.Creatures},
		{"creature_growth_restored", 0, 102, p.CreatureGrowth},
		{"worn_equipment_restored", 0, 13, p.Worn},
		{"user_area_sent", 0, 23, p.UserArea},
		{"town_entry_probe_sent", 0, 24, p.Area},
		{"fatigue_sent", 0, 36, p.Fatigue},
		{"enter_gameworld_complete_sent", 0, 124, p.Complete},
		{"entry_experience_restored", 0, 37, p.Experience},
		{"odyssey_journal_restored", 0, 2856, p.OdysseyProgress},
		{"completed_quests_restored", 0, 342, p.CompletedQuests},
		{"available_quests_restored", 0, 21, p.AvailableQuests},
		{"skill_variations_restored", 1, 29, p.SkillVariations},
		// Complete list-1 refresh after the entry/actor initialization barrier.
		{"avatar_inventory_restored", 0, 13, p.AvatarReady},
		{"worn_equipment_visuals_restored", 0, 14, p.WornUpdate},
	}
	// The players already in the scene have to be introduced before the area
	// list that places them. The client creates an actor from USERINFO and only
	// places actors it already knows, so a list arriving first is dropped: the
	// newcomer then sees nobody until the other player happens to move.
	for _, info := range p.Peers {
		out = append(out, outboundPacket{"entry_peer_info_sent", 0, 2, info})
	}
	return append(out,
		outboundPacket{"town_entry_probe_sent", 0, 24, p.Area},
		outboundPacket{"fatigue_sent", 0, 36, p.Fatigue},
		outboundPacket{"enter_gameworld_complete_sent", 0, 124, p.Complete},
		outboundPacket{"entry_experience_restored", 0, 37, p.Experience},
		outboundPacket{"odyssey_journal_restored", 0, 2856, p.OdysseyProgress},
		outboundPacket{"completed_quests_restored", 0, 342, p.CompletedQuests},
		outboundPacket{"available_quests_restored", 0, 21, p.AvailableQuests},
		outboundPacket{"skill_variations_restored", 1, 29, p.SkillVariations},
		outboundPacket{"worn_equipment_visuals_restored", 0, 14, p.WornUpdate},
	)
}

// Prepare every frame before the first write. In particular, a missing cipher
// must not disconnect an actor after SELECT but before scene/fatigue bootstrap.
func preparePackets(keys []byte, packets []outboundPacket) ([]preparedPacket, error) {
	var prepared []preparedPacket
	for _, p := range packets {
		if len(p.Payload) == 0 {
			continue
		}
		encrypted, err := wire.EncryptPayload(keys, p.ID, p.Payload)
		if err != nil {
			return nil, fmt.Errorf("%s type=%d id=%d encode: %w", p.Name, p.Kind, p.ID, err)
		}
		raw, err := wire.ServerFrame(p.Kind, p.ID, encrypted)
		if err != nil {
			return nil, fmt.Errorf("%s type=%d id=%d frame: %w", p.Name, p.Kind, p.ID, err)
		}
		prepared = append(prepared, preparedPacket{p, raw})
	}
	return prepared, nil
}

func writePackets(w io.Writer, packets []preparedPacket, sent func(preparedPacket)) error {
	// A previous reply may have left an expired socket deadline. Refresh it
	// for every batch, including quest replies after an idle conversation.
	if conn, ok := w.(interface{ SetWriteDeadline(time.Time) error }); ok {
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return fmt.Errorf("packet batch deadline: %w", err)
		}
	}
	for _, p := range packets {
		if _, err := io.Copy(w, bytes.NewReader(p.Raw)); err != nil {
			return fmt.Errorf("%s type=%d id=%d write: %w", p.Name, p.Kind, p.ID, err)
		}
		if sent != nil {
			sent(p)
		}
	}
	return nil
}
