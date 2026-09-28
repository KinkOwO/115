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
	Select, Basic, Addition, Skills, SkillPreset, Vault, UserArea, Area, Fatigue, Complete []byte
	Experience, CompletedQuests, Inventory                                                 []byte
	// AccountMaterials is the NOTI13 list35 account material storage
	// snapshot. It must be delivered before the list0 inventory snapshot so
	// the client harvest (sub_145ADC2A0) moves the fixed slots 363..379 into
	// the soul-storage pipeline.
	AccountMaterials []byte
	RadiantSouls     []byte
	SecondaryVault   []byte
	AccountVault     []byte
	AvailableQuests  []byte
	Worn             []byte
	AccountOptions   []byte
	GamepadOptions   []byte
	// InformNotice / InformNotice2nd are the per-character read-notice sets
	// (NOTI402 / NOTI426). They ride right after account options: the client
	// clears its read set from them, and a third-awakened character whose
	// notice ledger is missing would otherwise re-pop the teaching frame on
	// every login. Empty payloads are skipped by preparePackets.
	InformNotice    []byte
	InformNotice2nd []byte
	// Category-0 owned skins must arrive before their persisted selection.
	ProfileSkinCargo, ProfileSkinSelection []byte
	// SkinCargoDamageFont is the NOTI1545 damage-font owned page: every
	// `[add skin storage]` skin the account has spent a damage font on. It is a
	// page frame, so it rebuilds that page from scratch and must carry the whole
	// list, not only the newest grant.
	SkinCargoDamageFont []byte
	// SkinSelectionDamageFontNormal / Cumulative are the two NOTI1546 frames that
	// re-apply the fonts this character chose in the panel's two tabs. Each has to
	// follow its own owned page above.
	SkinSelectionDamageFontNormal     []byte
	SkinSelectionDamageFontCumulative []byte
	// SkinCargoPartyFrame / SkinCargoSkillCutscene are the 边框 and 觉醒插图 owned
	// pages. The border page is also the profile-decoration feature's page, so these
	// arrive after its own pair below: a NOTI1545 frame rebuilds the page it names,
	// and the later frame is the one the client keeps.
	SkinCargoPartyFrame    []byte
	SkinCargoSkillCutscene []byte
	// SkinSelectionPartyFrame / SkinSelectionSkillCutscene re-apply what this
	// character has selected in those two panels. Both are sets, not single ids:
	// the cutscene renderer draws a random member of the vector.
	SkinSelectionPartyFrame    []byte
	SkinSelectionSkillCutscene []byte
	// SkinFamilyRestores carries the entry frames of the skin families added after
	// those two — 表情 / 涂鸦 / 飞空艇特效 owned pages and selections, plus the
	// NOTI2641 收藏 push. They sit in the same window as the pair above (after the
	// profile-decoration pages, before the town refresh) and each is emitted only when
	// it has something to say, so preparePackets drops the empty ones.
	SkinFamilyRestores []outboundPacket
	// WornSlots is the id-14 per-slot update frame for the full worn set
	// (space 3), same builder the equipment-move path uses. The live
	// 20260921 probe timeline showed the equip-change heal always carries
	// equipment_slots_updated frames while entry never does, and the
	// arrow-up overlay reads per-slot levels fed by exactly this channel.
	// Since 20260923 the frame is emitted TWICE per entry: once before
	// NOTI24 when a weapon is worn, and once after the entry barrier for
	// the equipment upgrade arrows.
	WornSlots        []byte
	WornUpdate       []byte
	WeaponEquipped   bool
	WeaponAppearance []byte
	// SkinCargo is the NOTI1545 body that refills the skin storage (weapon
	// appearance tab). The client keeps that container as session state and only
	// ever received a push alongside a replication confirmation, so a relog left
	// the storage empty while the applied skin still drove the real model (live
	// 2026-09-26). Empty when nothing was replicated, and preparePackets then
	// skips it.
	SkinCargo []byte
	// SkinSelection is the NOTI1546 body naming the skin currently worn on the
	// weapon. The storage window frames that row from the client's own per-page
	// selection table, which nothing ever fed - Apply only paints a window-local
	// cell - so the frame disappeared as soon as the window was closed and
	// reopened (live 2026-09-27). It rides immediately after the container push
	// that fills the same page, and a character with no skin applied leaves it
	// empty so preparePackets drops the frame.
	SkinSelection                                                 []byte
	Avatars, AvatarReady, Creatures, CreatureList, CreatureGrowth []byte
	CinematicSkips                                                []byte
	// SkinRecent is the NOTI1547 body the storage window's 最近获得 strip rebuilds from
	// its own five cells. The frame is absolute state - the reader clears the manager
	// vector before its loop - so the entry push carries the whole list: every skin the
	// account registered plus this character's replicated weapon shapes, oldest first.
	// Empty when nothing is visible to the client, which leaves the strip as the client
	// itself ships it.
	SkinRecent []byte
	// StoryDigest is the NOTI1370 4-byte little-endian story digest level.
	// It must follow NOTI1352 inside the same entry group: the client asks on
	// every town entry "how far has this character seen", and without an answer
	// it replays the opening recap movie from the start.
	StoryDigest []byte
	// BoosterGage is the NOTI398 14-byte frame with displayValue=0 that hides
	// the town top-left "Liberation Trace" mileage panel. It must be emitted
	// after NOTI124: before enter_gameworld_complete the panel object has not
	// finished initialization. Empty payload is skipped by preparePackets, so
	// the feature-off path equals the pre-fix behavior.
	BoosterGage     []byte
	SkillVariations []byte
	OdysseyProgress []byte
	// SkillLocks is the NOTI2827 character option block that restores the
	// player's locked skills. It is sent last: the forwarded evidence for this
	// client reports a crash on town entry when 2827 arrives early in the frame
	// sequence, whichever block it contains.
	SkillLocks   []byte
	SynopsisRead []byte
	CubeContract []byte
	// Peers carries the USERINFO of every actor already standing in the scene.
	// It is emitted after this actor's own placement but before the area list,
	// because the client only places actors it already knows.
	Peers [][]byte
}

func (p entryPayloads) packets() []outboundPacket {
	out := []outboundPacket{
		{"select_parser_response", 1, 4, p.Select},
		{"account_options_restored", 0, 2826, p.AccountOptions},
		{"gamepad_options_restored", 0, 2128, p.GamepadOptions},
		{"inform_notice_restored", 0, 402, p.InformNotice},
		{"inform_notice_2nd_restored", 0, 426, p.InformNotice2nd},
		{"skin_cargo_damage_font_restored", 0, 1545, p.SkinCargoDamageFont},
		{"skin_selection_damage_font_restored", 0, 1546, p.SkinSelectionDamageFontNormal},
		{"skin_selection_damage_font_restored", 0, 1546, p.SkinSelectionDamageFontCumulative},
		{"profile_skin_cargo_restored", 0, 1545, p.ProfileSkinCargo},
		{"profile_skin_selection_restored", 0, 1546, p.ProfileSkinSelection},
		// Both list families follow the profile pair above on purpose: the border
		// page is the same page that feature rebuilds, and the last page frame for a
		// page is the state the client keeps. A page rebuild is absolute, so these
		// payloads already carry the profile feature's built-in rows.
		{"skin_cargo_party_frame_restored", 0, 1545, p.SkinCargoPartyFrame},
		{"skin_selection_party_frame_restored", 0, 1546, p.SkinSelectionPartyFrame},
		{"skin_cargo_skill_cutscene_restored", 0, 1545, p.SkinCargoSkillCutscene},
		{"skin_selection_skill_cutscene_restored", 0, 1546, p.SkinSelectionSkillCutscene},
		{"cinematic_skips_restored", 0, 1352, p.CinematicSkips},
		{"story_digest_restored", 0, 1370, p.StoryDigest},
		{"entry_basic_probe_sent", 0, 2, p.Basic},
		{"entry_addition_sent", 0, 2, p.Addition},
		{"entry_skills_sent", 0, 19, p.Skills},
		{"skill_preset_restored", 0, 2758, p.SkillPreset},
		{"vault_initialized", 0, 13, p.Vault},
	}
	// The newer skin families and the 收藏 push join the same entry window as the two
	// list families above: an owned-page frame rebuilds its page from scratch, so it has
	// to arrive before the selection frame that names ids from it, and both have to
	// precede the town refresh.
	out = append(out, p.SkinFamilyRestores...)
	if len(p.SecondaryVault) > 0 {
		out = append(out, outboundPacket{"secondary_vault_initialized", 0, 13, p.SecondaryVault})
	}
	if len(p.AccountVault) > 0 {
		out = append(out, outboundPacket{"账号金库登录恢复", 0, 13, p.AccountVault})
	}
	out = append(out, []outboundPacket{
		{"account_materials_restored", 0, 13, p.AccountMaterials},
		{"radiant_souls_restored", 0, 13, p.RadiantSouls},
		{"inventory_restored", 0, 13, p.Inventory},
		// Initialize list 1 empty. The client accepts authoritative avatar rows
		// only after the town actor/UserInfo graph has been installed.
		{"avatar_inventory_initialized", 0, 13, p.Avatars},
		{"worn_equipment_restored", 0, 13, p.Worn},
	}...)
	// The NOTI24 town refresh checks worn slot 12. A weapon needs its explicit
	// appearance binding and NOTI14 rows installed before that check. Sending
	// this rebuild for an empty slot suppresses the client's real warning.
	if p.WeaponEquipped {
		out = append(out,
			outboundPacket{"weapon_appearance_entry_prelude", 0, 2, p.WeaponAppearance},
			outboundPacket{"equipment_slots_updated_entry_prelude", 0, 14, p.WornSlots},
			outboundPacket{"worn_equipment_window_refreshed_entry_prelude", 0, 14, p.WornUpdate},
		)
	}
	out = append(out, outboundPacket{"user_area_sent", 0, 23, p.UserArea})
	// The players already in the scene have to be introduced before the area
	// list that places them. The client creates an actor from USERINFO and only
	// places actors it already knows, so a list arriving first is dropped: the
	// newcomer then sees nobody until the other player happens to move.
	for _, info := range p.Peers {
		out = append(out, outboundPacket{"entry_peer_info_sent", 0, 2, info})
	}
	out = append(out,
		outboundPacket{"town_entry_probe_sent", 0, 24, p.Area},
		outboundPacket{"fatigue_sent", 0, 36, p.Fatigue},
		outboundPacket{"enter_gameworld_complete_sent", 0, 124, p.Complete},
		// NOTI398 displayValue=0 collapses the top-left Liberation Trace panel
		// (see docs/protocol/next52-liberation-trace-booster-gage-398.md). It
		// must follow 124: the panel object is not initialized before it. The
		// body is 14 bytes, not 18 — a longer frame overruns and the writer
		// swallows later CMDs (wire-overflow-report-217).
		outboundPacket{"booster_gage_hidden", 0, 398, p.BoosterGage},
		outboundPacket{"entry_experience_restored", 0, 37, p.Experience},
		outboundPacket{"odyssey_journal_restored", 0, 2856, p.OdysseyProgress},
		outboundPacket{"completed_quests_restored", 0, 342, p.CompletedQuests},
		outboundPacket{"available_quests_restored", 0, 21, p.AvailableQuests},
		// Rebuild unread synopsis IDs after the client has its quest lists.
		outboundPacket{"synopsis_read_restored", 0, 2310, p.SynopsisRead},
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
		// 原生 NOTI889 会查询晶块库存，须在库存和角色初始化后恢复。
		outboundPacket{"cube_contract_selection_restored", 0, 889, p.CubeContract},
	)
	// 幻化仓库（武器外观页签）的容器内容只在复制时推过一次，客户端把它当会话态，
	// 重登就空。这里按存档重推 NOTI1545，皮肤才会留在仓库里。
	//
	// 必须排在 actor_appearance_ready 之前：下面那条 id-2 帧要作为最后一个数据帧，
	// 客户端的 actor 重建才会看到前面所有刚装好的行。空列表不建帧（与
	// SecondaryVault/AccountVault 同），没有复制的角色与改动前完全一致。
	if len(p.SkinCargo) > 0 {
		out = append(out, outboundPacket{"skin_cargo_restored", 0, 1545, p.SkinCargo})
	}
	// 幻化仓库里"正在佩戴的那一行"的金色边框由客户端自己的页签选择表决定，而那张表
	// 从来没有被喂过（Apply 只写窗口本地格），窗口一关一开就没了（实机 2026-09-27）。
	// 紧跟着上面那条容器帧补一次 NOTI1546，重开时客户端才能按同一个 id 重新高亮。
	if len(p.SkinSelection) > 0 {
		out = append(out, outboundPacket{"skin_cargo_selected", 0, 1546, p.SkinSelection})
	}
	// The 最近获得 strip is the window's own five-cell row list, fed only by NOTI1547,
	// and that frame is a whole-list rebuild (sub_1444ED400 clears the vector before its
	// loop). Nothing re-sent it since replication, so a relog left the strip empty even
	// with skins registered. It joins the same place as the two weapon-shape frames
	// above: after the initialization barrier, before the last actor rebuild.
	if len(p.SkinRecent) > 0 {
		out = append(out, outboundPacket{"skin_recent_restored", 0, 1547, p.SkinRecent})
	}
	return append(out,
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
