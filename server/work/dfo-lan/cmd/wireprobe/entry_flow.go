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
	CloneSources     []outboundPacket
	KnightDeck       []byte
	AccountOptions   []byte
	GamepadOptions   []byte
	// Journal 是装备库完整状态（NOTI2610，恰好 16444B）。
	Journal []byte
	// EquipmentSkill 是装备技能栏/冷却提醒/自定义按键的两组快照（S2C2609，168B）。
	// 客户端一次消费前 160 字节，所以**恒发**（没设过就是全零）。
	// 它在 packets() 里的位置必须晚于 2758，见那里的注释。
	EquipmentSkill []byte
	// OathPartSetPoints 是 NOTI2634「每角色一对 Set/Oath Point」的若干条候选帧
	// （每条 10 字节：`u16 实体键 + u32 SetPoint + u32 OathPoint`）。它们在 packets()
	// 里被排在**所有帧之后**：handler 写的是角色实体对象，必须等 actor 重建完。
	OathPartSetPoints []outboundPacket
	// 冒险图鉴为账号登记集合，登录即恢复，不依赖角色移动上报。
	AdventureCollection []byte
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
	ComboSkillInfo  []byte
	BuffEnhancement []byte
	SkillLocks      []byte
	SynopsisRead    []byte
	CubeContract    []byte
	OathSystemInfo  []byte
	// Starter Boost 662 进城恢复：BoostGifts 是 NOTI2265 礼物可领集合，
	// BoostTraining 是 NOTI2638 训练进度。活动关闭时两者为 nil，preparePackets
	// 跳帧，普通角色进城序列与改动前逐字节一致。
	// 2265 必须排在 enter_gameworld_complete(124) 之前（先建事件行），2638 必须排在
	// 124 之后（任务面板对象这时才存在）—— 见 TestBoostGiftEntryOrderingAndPerRoleRestore。
	BoostGifts    []byte
	BoostTraining []byte
	// Peers carries the USERINFO of every actor already standing in the scene.
	// It is emitted after this actor's own placement but before the area list,
	// because the client only places actors it already knows.
	Peers [][]byte
}

// weeklyDungeonInfoConfig is the verbatim NOTI708 body captured from the
// official server's channel-entry announce (2026-10-02 16:03 capture, s1
// frame 8, byte-identical in s4). The client's own fallback legion schedule
// closes Ispins on Thursday/Friday; these bytes are what the official server
// answered the same client with on a Friday, so they are replayed as-is.
// Layout beyond the capture is NOT interpreted - no field semantics are
// guessed (project rule: bytes must come from official evidence).
var weeklyDungeonInfoConfig = []byte{
	0x32, 0x33, 0x36, 0x3f, 0x3e, 0x3b, 0x3a, 0x39,
	0xff, 0xff,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x51, 0xfc, 0x40, 0xed, 0x35, 0x00,
}

// integrateEventData is the verbatim NOTI1198 INTEGRATE_EVENT_DATA body from
// the same official capture (s1 frame 15 / s4 frame 15, byte-identical, 176
// bytes; the u64 at +0xA5 is the capture-time unix timestamp). Official
// announce order is 708 (frame 8) -> 1198 (frame 15) -> 108 (frame 58): the
// 1198 frame arrives BEFORE the 108 event table. Replaying 108 without it made
// the client answer CMD217 ENUM_CMDPACKET_OVERFLOW_INFO (plain
// 006c000000000000, i.e. reporting NOTI108) right after the announce and then
// go fully silent - the client appears to need the 1198 container state before
// it can absorb the 108 table (next79 §10, freeze regression 2026-10-02
// 21:45). Layout beyond the capture is NOT interpreted (project rule).
var integrateEventData = mustHexDecode(
	"4209000001000000000000000000000000000000000000000002000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"00000000009fd695103f000000000000")

// weeklyDungeonInOutInfo is the verbatim NOTI1336 WEEKLY_DUNGEON_INOUT_INFO
// body from the official channel-entry announce (2026-10-02 capture: s1
// frame 60 / s4 frame 60, byte-identical). s4 is the session where the
// client actually fired CMD2043 into Ispins, and this frame rides its
// announce before the click. Field semantics unknown - replayed as-is.
var weeklyDungeonInOutInfo = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xf4, 0xf7, 0x69, 0xb2, 0x3a, 0x00, 0x00, 0x00,
}

// dungeonEnterCountInfo is the verbatim NOTI537 DUNGEON_ENTER_COUNT_INFO
// body from the same official announce (s1 frames 98/149 and s4 frames
// 91/142, all byte-identical). Official pushes it twice in the burst;
// one copy is replayed here - the handler is absolute state, and both
// captures carry the same bytes.
var dungeonEnterCountInfo = []byte{
	0xc4, 0x0d, 0x00, 0x00, 0x05, 0x00, 0xd5, 0xcd,
	0x28, 0xca, 0x3b, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// questClearGroupInfo is the verbatim NOTI1792 QUEST_CLEAR_GROUP_INFO body
// from the official login flood (2026-10-02 s4 frame 9 / 2026-10-03 switch
// frame 12, byte-identical, 96 bytes). It is the ONLY quest-completion data
// that reaches the client BEFORE the SELECT_CHARACTER gate: NOTI342 clears
// only after the selection (switch frame 129, t+7.1s), but the legion
// channel character-select gate ("必须完成任务『熄灭火焰的时间』") is
// evaluated while picking the character, against the clear-group state this
// frame installs (next79 §20: without it the gate pops and the client kicks
// back to town even for a max-level character). Layout is NOT interpreted -
// replayed as-is (project rule).
var questClearGroupInfo = mustHexDecode(
	"1100000035000000013600000001370000000138000000013a" +
		"000000013b000000013c000000013d000000013e000000013f" +
		"000000014000000001450000000146000000014b000000014c" +
		"000000014d000000014e000000016289eda43c0000")

// loginFloodPackets is the account-level event/weekly burst the official
// server pushes during the LOGIN flood, BEFORE the character selection. The
// 2026-10-02 21:18 capture shows every business connection repeating
// 1759 -> 708 -> 1198 -> 108 -> 1336 before the SELECT_CHARACTER(4) request
// (and a second 108 after NOTI706 post-selection). The earlier replay put
// this whole burst in the post-selection town announce; the client answered
// that announce with CMD217 ENUM_CMDPACKET_OVERFLOW_INFO (plain
// 006c000000000000, reporting NOTI108) and froze on town entry (next79 §13).
// Round 10 then replayed the official burst verbatim (108 included) at this
// exact login stage and the client STILL froze at the selection screen -
// the official 108 body is rejected by the private client's parser wherever
// it is placed. The 108 therefore left the flood entirely (next79 §15): the
// login stage is now always healthy, and the ONLY NOTI108 this server sends
// is the announce's fixed V3 table (event_info_generated.go) - the single
// count=1 raw body the round-11 experiment proved the client accepts.
// The burst is sent once per connection, right after the roster-background
// (NOTI1759) restore that precedes the selection screen.
func loginFloodPackets() []outboundPacket {
	return []outboundPacket{
		{"weekly_dungeon_config_sent", 0, 708, weeklyDungeonInfoConfig},
		{"quest_clear_group_info_sent", 0, 1792, questClearGroupInfo},
		{"integrate_event_data_sent", 0, 1198, integrateEventData},
		{"weekly_dungeon_inout_info_sent", 0, 1336, weeklyDungeonInOutInfo},
	}
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
		// The 708/1198/108/1336 burst moved from this announce to the
		// pre-selection LOGIN flood (loginFloodPackets): the official 21:18
		// capture pushes all four frames before SELECT_CHARACTER, and
		// replaying the 108 table only in this post-selection announce made
		// the client answer CMD217 ENUM_CMDPACKET_OVERFLOW_INFO and freeze on
		// entry (next79 §13).
		// NOTI706 WEEKLY_DUNGEON_INFO - the 1464-byte weekly dungeon table
		// (s1 frame 78; see weekly_dungeon_info_generated.go for why it rides
		// here even though s4 does not repeat it).
		{"weekly_dungeon_info_sent", 0, 706, weeklyDungeonInfoTable},
		// The NOTI108 EVENT_INFO table. Official post-selection order is
		// 706 -> 108 -> STAMINA(4) -> 537. The body comes from
		// event_info_generated.go: the 2026-10-04 legion/raid gate round grew
		// the round-11 probe's single-record body (54 B, "Ispins Legion Open"
		// 776) into the full 19-record gate table (1144 B). Still RAW only -
		// zlib containers trigger CMD217 ENUM_CMDPACKET_OVERFLOW_INFO and
		// freeze this client (probe V2/V5), and an over-long raw body froze it
		// too (probe V4), so the frame keeps the empty tail.
		// event_info_variant.go exposes DFO_EVENT_INFO_VARIANT for the 662
		// entry investigation; unset = exactly this table.
		{"event_info_sent", 0, 108, townEventInfoTable},
		{"dungeon_enter_count_info_sent", 0, 537, dungeonEnterCountInfo},
		{"entry_basic_probe_sent", 0, 2, p.Basic},
		{"entry_addition_sent", 0, 2, p.Addition},
		{"entry_skills_sent", 0, 19, p.Skills},
		{"skill_preset_restored", 0, 2758, p.SkillPreset},
		// 2609 必须排在 2758 **之后**：TestEntrySkillPresetFollowsSkillTree 钉死
		// 2758 紧跟 19（技能树之后立刻是技能预设），插在中间会让那条测试变红。
		{"equipment_skill_restored", 0, 2609, p.EquipmentSkill},
		// NOTI19 rebuilds the skills first; restore the combo cells afterwards.
		{"combo_skill_info_restored", 0, 433, p.ComboSkillInfo},
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
		{"knight_deck_restored", 0, 567, p.KnightDeck},
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
		outboundPacket{"boost_gift_states_restored", 0, 2265, p.BoostGifts},
		outboundPacket{"town_entry_probe_sent", 0, 24, p.Area},
		outboundPacket{"fatigue_sent", 0, 36, p.Fatigue},
		outboundPacket{"enter_gameworld_complete_sent", 0, 124, p.Complete},
		// 训练进度帧排在 124 之后：任务面板对象在进城完成前不存在，早到的 2638
		// 会被丢掉，客户端的活动关卡停在旧值（实机第三关「面板不刷新」同一坑）。
		outboundPacket{"boost_training_progress_restored", 0, 2638, p.BoostTraining},
		// NOTI398 displayValue=0 collapses the top-left Liberation Trace panel
		// (see docs/protocol/next52-liberation-trace-booster-gage-398.md). It
		// must follow 124: the panel object is not initialized before it. The
		// body is 14 bytes, not 18 — a longer frame overruns and the writer
		// swallows later CMDs (wire-overflow-report-217).
		outboundPacket{"booster_gage_hidden", 0, 398, p.BoosterGage},
		outboundPacket{"entry_experience_restored", 0, 37, p.Experience},
		outboundPacket{"odyssey_journal_restored", 0, 2856, p.OdysseyProgress},
		outboundPacket{"skill_variations_restored", 1, 29, p.SkillVariations},
		// Complete lists and visual refresh after the entry/actor initialization barrier.
		outboundPacket{"avatar_inventory_restored", 0, 13, p.AvatarReady},
	)
	// The list1 objects must exist before Clone's source lookup is restored,
	// and subsequent worn reconstruction must consume this character's table.
	out = append(out, p.CloneSources...)
	out = append(out,
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
		outboundPacket{"oath_system_info_restored", 0, 2839, p.OathSystemInfo},
		// 1361 resolves items immediately: all inventory families and skill
		// trees must already exist, after the final NOTI13/14 resynchronization.
		outboundPacket{"buff_enhancement_restored", 0, 1361, p.BuffEnhancement},
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
	// 装备库完整状态。客户端只把它当数据存进映射（handler sub_145304380 不依赖角色对象），
	// 所以放在 actor 重建之前是安全的；它在 actor_appearance_ready 之前进入同一条有序流。
	if len(p.Journal) > 0 {
		// 2610 = protocol.EquipmentJournalOpcode；本文件一律用字面量（与其它帧一致）。
		out = append(out, outboundPacket{"equipment_journal_restored", 0, 2610, p.Journal})
	}
	if len(p.AdventureCollection) > 0 {
		// NOTI2425先替换集合，窗口531已打开时才重绘；空集合也需恢复，避免换号残留。
		out = append(out, outboundPacket{"冒险图鉴登录恢复", 0, 2425, p.AdventureCollection})
	}
	out = append(out,
		outboundPacket{"actor_appearance_ready", 0, 2, p.Basic},
		// 官服入场序列在末尾的 864B USERINFO basic（s4 #116）之后才发 NOTI342
		// （#118）与 NOTI21（#121）。2.38.2 的 342 handler 只有在 QuestManager
		// （[0x14E683D40]，init batch 0x145a21e40 创建）就绪后才会把完成列表
		// 插入完成哈希集合；manager 为空时 990 个 ID 全部跳过，实机表现为
		// 「再次查看」页近乎空白、伊斯大陆门禁弹窗（2026-10-03）。因此
		// 342/21/2310 必须跟在 actor_appearance_ready 之后、2827 之前。
		outboundPacket{"completed_quests_restored", 0, 342, p.CompletedQuests},
		outboundPacket{"available_quests_restored", 0, 21, p.AvailableQuests},
		// Rebuild unread synopsis IDs after the client has its quest lists.
		outboundPacket{"synopsis_read_restored", 0, 2310, p.SynopsisRead},
		// The character option block goes after every other entry frame: this
		// client crashes on town entry when NOTI2827 arrives early.
		outboundPacket{"skill_locks_restored", 0, 2827, p.SkillLocks},
	)
	// NOTI2634（每角色一对 Set/Oath Point）排在**最后**：handler 直接写角色实体对象的
	// `实体+1872/+1876`，必须等 actor_appearance_ready 重建完实体再写，否则会被覆盖。
	// 见 docs/protocol/oath-set-points-20261004.md §7.1。
	return append(out, p.OathPartSetPoints...)
}

// Prepare every frame before the first write. In particular, a missing cipher
// must not disconnect an actor after SELECT but before scene/fatigue bootstrap.
func preparePackets(keys []byte, packets []outboundPacket) ([]preparedPacket, error) {
	var prepared []preparedPacket
	for _, p := range packets {
		// nil payload = placeholder the plan never fills: skip. A non-nil
		// zero-length slice is a genuine zero-body frame (next79 §25: the
		// Ispins stage settlement N1658, official s4 frame 493, is a bare
		// 16-byte s2c header) and must be delivered.
		if p.Payload == nil {
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
