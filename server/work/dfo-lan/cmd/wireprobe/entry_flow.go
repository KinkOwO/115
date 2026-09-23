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
	// AccountMaterials is the NOTI13 list35 account material storage
	// snapshot. It must be delivered before the list0 inventory snapshot so
	// the client harvest (sub_145ADC2A0) moves the fixed slots 363..379 into
	// the soul-storage pipeline.
	AccountMaterials []byte
	SecondaryVault   []byte
	AccountVault     []byte
	AvailableQuests  []byte
	Worn             []byte
	AccountOptions   []byte
	// WornSlots is the id-14 per-slot update frame for the full worn set
	// (space 3), same builder the equipment-move path uses. The live
	// 20260921 probe timeline showed the equip-change heal always carries
	// equipment_slots_updated frames while entry never does, and the
	// arrow-up overlay reads per-slot levels fed by exactly this channel.
	WornSlots                                                     []byte
	WornUpdate                                                    []byte
	Avatars, AvatarReady, Creatures, CreatureList, CreatureGrowth []byte
	CinematicSkips                                                []byte
	SkillVariations                                               []byte
	OdysseyProgress                                               []byte
	// SkillLocks is the NOTI2827 character option block that restores the
	// player's locked skills. It is sent last: the forwarded evidence for this
	// client reports a crash on town entry when 2827 arrives early in the frame
	// sequence, whichever block it contains.
	SkillLocks []byte
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
	}
	if len(p.SecondaryVault) > 0 {
		out = append(out, outboundPacket{"secondary_vault_initialized", 0, 13, p.SecondaryVault})
	}
	if len(p.AccountVault) > 0 {
		out = append(out, outboundPacket{"账号金库登录恢复", 0, 13, p.AccountVault})
	}
	out = append(out, []outboundPacket{
		{"account_materials_restored", 0, 13, p.AccountMaterials},
		{"inventory_restored", 0, 13, p.Inventory},
		// Initialize list 1 empty. The client accepts authoritative avatar rows
		// only after the town actor/UserInfo graph has been installed.
		{"avatar_inventory_initialized", 0, 13, p.Avatars},
		{"worn_equipment_restored", 0, 13, p.Worn},
		{"user_area_sent", 0, 23, p.UserArea},
	}...)
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
		// Complete lists and visual refresh after the entry/actor initialization barrier.
		outboundPacket{"avatar_inventory_restored", 0, 13, p.AvatarReady},
		outboundPacket{"creature_list_restored", 0, 105, p.CreatureList},
		outboundPacket{"creature_inventory_restored", 0, 13, p.Creatures},
		outboundPacket{"creature_growth_restored", 0, 102, p.CreatureGrowth},
		// (20260921 second pass) The id13-worn-only re-feed did NOT fix the
		// upgrade arrows (session 194434: after_barrier worn arrived, arrows
		// still all shown). Mirror the equipment-move resync instead, same
		// order and same builders: bag list0 (2358B, identical to
		// inventory_restored) -> worn (1668B) -> worn window (1632B), and only
		// then the appearance block. equipment_flow.go's C9 comment explains
		// the ordering: the client's rebuild must see the id13/id14 rows above
		// as fresh item objects, so the appearance refresh goes LAST. At entry
		// the appearance block previously landed BEFORE the rows.
		outboundPacket{"equipment_bag_resynced_entry", 0, 13, p.Inventory},
		outboundPacket{"worn_equipment_restored_after_barrier", 0, 13, p.Worn},
		// (20260921 third pass) The equip-change heal (probe timeline
		// 21:56:14) always emits id-14 per-slot frames before the worn-window
		// refresh; entry never did, and the level compare behind the upgrade
		// arrows degenerates without them. Re-feed the whole worn set through
		// the same slot-update channel before the window refresh.
		outboundPacket{"equipment_slots_updated_entry", 0, 14, p.WornSlots},
		outboundPacket{"worn_equipment_window_refreshed_entry", 0, 14, p.WornUpdate},
		outboundPacket{"actor_appearance_ready", 0, 2, p.Basic},
		// The character option block goes after every other entry frame: this
		// client crashes on town entry when NOTI2827 arrives early.
		outboundPacket{"skill_locks_restored", 0, 2827, p.SkillLocks},
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
