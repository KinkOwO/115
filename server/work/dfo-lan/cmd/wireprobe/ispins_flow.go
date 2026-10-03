package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// 伊斯大陆（内容号 101）军团流程。全部字节契约来自官服抓包
// analysis/tasks/next78-ispins-official-capture-p0.md（session_s4，2026-10-02）；
// 语义未解字段（5B token 家族）一律回放官服观测值，不做解释。

// ispinsRun 是一次伊斯大陆挑战的会话状态。
//
// v1 限制：阶段固定按官服默认顺序（nemaug→nagor→ashcore→itrenog，即
// IspinsStageDungeons 的数组顺序）推进。CMD2047 变体 A 的选择 token 未解，
// 服务端无法从请求里还原玩家实际选择的顺序；玩家若在 UI 换序，进本阶段号
// 与服务端加载的副本会不一致，此限制待官服向量补解后再放开。
type ispinsRun struct {
	// stage 是当前进入（或下一个待进）的阶段号 0..3。
	stage int
	// cleared 记录已通关阶段。
	cleared [4]bool
	// confirmed 表示 CMD2047 变体 B（确认操作）已到达。
	confirmed bool
	// finalDone 表示终局 ACK2046 已发，后续 CMD191 走 ispins 剧情分支。
	finalDone bool
	// storyFinished is presentation completion, not dungeon/party departure.
	// Keep ownership until the actual CMD72 town exit has been prepared.
	storyFinished bool
}

// 官服 S1 局（默认顺序）各 ACK 尾 5B token，逐帧回放（next78 §5.1：语义
// 未解，推测会话/防重放 nonce；服务端无法自行生成，v1 照抄官服值）。
var ispinsStartAckNonce = [5]byte{0x3b, 0xff, 0xf7, 0x7e, 0x43}

// ACK2047-A 的 token 随每次操作选择变化；官服该局四个阶段各一次，按已
// 通关数索引。
var ispinsOperationAckANonces = [4][5]byte{
	{0xd3, 0xdc, 0x88, 0x5b, 0x3f},
	{0x39, 0xf9, 0x49, 0x92, 0x3b},
	{0xc1, 0xcc, 0x60, 0x8e, 0x3b},
	{0xf7, 0x79, 0xf5, 0xc1, 0x37},
}

// ACK2047-B 的 token 官服四阶段恒定（S3 局异常为 cda2b2c735，那是该局的
// 现象不是协议必需，v1 统一回放常量值）。
var ispinsOperationAckBNonce = [5]byte{0x9c, 0x14, 0xc6, 0xd4, 0x3c}

// ACK2045/ACK2046 的 token 按阶段回放。
var ispinsEnterAckNonces = [4][5]byte{
	{0x4a, 0x12, 0xf4, 0x2f, 0x44},
	{0xd8, 0xac, 0xf2, 0xa2, 0x38},
	{0x29, 0x8b, 0x7f, 0x8d, 0x39},
	{0xe9, 0xb0, 0x84, 0xd8, 0x33},
}
var ispinsRewardEndAckNonces = [4][5]byte{
	{0x9e, 0xa9, 0x30, 0x2b, 0x44},
	{0x54, 0x82, 0x91, 0x51, 0x3c},
	{0x29, 0x8b, 0x7f, 0x8d, 0x39},
	{0xe4, 0x59, 0x4d, 0xe8, 0x3c},
}

// 官服 N170 剧情暂停/恢复帧尾 5B token（f787 暂停、f793 恢复）。
var ispinsStoryPauseNonce = [5]byte{0x34, 0xbc, 0x2e, 0x0a, 0x42}
var ispinsStoryResumeNonce = [5]byte{0x2b, 0xbe, 0xb7, 0xd9, 0x39}

// isIspinsRequest 判别伊斯大陆族请求：CMD2047 为本族专属；CMD2043/2045/
// 2046 与末世录共用信封，由 body @13 的内容号 101 区分。判别失败的包落回
// 末世录 legion 分发。
func isIspinsRequest(id uint16, p []byte) bool {
	if id == legion.CmdIspinsOperationSelect {
		return true
	}
	if !legion.Requests(id) || len(p) < legion.EnvelopeSize+4 {
		return false
	}
	return binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) == legion.IspinsContentID
}

// ispinsClearedCount 返回已通关阶段数，也即下一个待进阶段的序号。
func (w *worldSession) ispinsClearedCount() int {
	count := 0
	for _, ok := range w.ispins.cleared {
		if ok {
			count++
		}
	}
	return count
}

// ispinsStandbyPartyHandle 实现待机区队伍对话框（next79 §21）：
// CMD12（PARTY_CREATE）此前无应答，点「确定」后 UI 毫无反应。官服
// 10-02 s4 抓包实证（包时间戳对齐）：应答是单帧 NOTI9，客户端收到后即
// 进入已建队状态，随后直接 CMD2043 开战，无其它握手帧。但官服帧本体是
// 新版客户端布局，2.38.2 解析即闪退，故应答用黑鸦族原生语法承载伊斯
// 语义（容量 4 / 类型 0x0b / 模式 1，见 protocol.IspinsStandbyPartyReply）。
// 队长资料两个 op=2 包照黑鸦先例先行（成员列表显示数据源）。
func (w *worldSession) ispinsStandbyPartyHandle(id uint16, p []byte) (bool, []outboundPacket, error) {
	if w.channelType != 81 || w.role.ID == 0 || (id != 12 && id != 13) {
		return false, nil, nil
	}
	fail := func(err error) (bool, []outboundPacket, error) { return true, nil, err }
	if w.characters == nil {
		return fail(fmt.Errorf("伊斯待机区角色服务不可用"))
	}
	if id == 13 {
		// The live native CMD13 body is empty plus eight zero padding bytes.
		if len(p) != 0 && len(p) != 8 {
			return fail(fmt.Errorf("伊斯离队请求长度无效"))
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("不支持的伊斯离队选项"))
			}
		}
		if w.activeDungeon != nil {
			return fail(fmt.Errorf("请先返回待机区再退出伊斯队伍"))
		}
		// Native NOTI9 action3 skips roster details and clears all eight
		// member slots at1452f40fa..4132. Reuse the proven current-build
		// party-gone grammar; Ispins creates the same local party ID9999.
		gone := protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)
		w.soloPartyReady = false
		w.ispins = nil
		return true, []outboundPacket{{"ispins_party_gone", 0, 9, gone}}, nil
	}
	name, err := protocol.DecodeIspinsStandbyParty(p)
	if err != nil {
		return fail(err)
	}
	party, err := protocol.IspinsStandbyPartyReply(name, w.role.WireID, w.characters.ChannelContext)
	if err != nil {
		return fail(err)
	}
	basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
	if err != nil {
		return fail(err)
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return fail(err)
	}
	// 同黑鸦建队先例：本连接持有单成员 bootstrap 队伍，选图 Party==1
	// 归一化为 65535，不把任意队伍号当普通队伍解析。
	w.soloPartyReady = true
	return true, []outboundPacket{
		{"伊斯队长资料", 0, 2, basic},
		{"伊斯队长详细资料", 0, 2, detail},
		{"伊斯待机区队伍创建", 0, 9, party},
	}, nil
}

func appendIspinsReplays(plan []outboundPacket, names ...string) ([]outboundPacket, error) {
	frames, err := legion.IspinsReplayFrames(names...)
	if err != nil {
		return nil, err
	}
	for _, f := range frames {
		plan = append(plan, outboundPacket{"ispins_" + f.Name, 0, f.ID, f.Body})
	}
	return plan, nil
}

// handleIspins answers one Ispins command (CMD2043/2045/2046/2047, content 101).
func (w *worldSession) handleIspins(p []byte, id uint16) ([]outboundPacket, []map[string]any, error) {
	switch id {
	case legion.CmdStart:
		return w.startIspins(p)
	case legion.CmdIspinsOperationSelect:
		return w.ispinsOperation(p)
	case legion.CmdEnterDungeon:
		return w.enterIspinsStage(p)
	case legion.CmdRewardEnd:
		return w.ispinsRewardEnd(p)
	}
	return nil, nil, fmt.Errorf("ispins opcode %d is not implemented", id)
}

// startIspins handles CMD2043. Official order (§1.1): N2254 (in-run base) and
// N2255 initial precede ACK2043; the waiting state is pushed ~2.7s later with
// no command trigger (f384) — that push is scheduled by the caller.
func (w *worldSession) startIspins(p []byte) ([]outboundPacket, []map[string]any, error) {
	if _, err := legion.DecodeIspinsStart(p); err != nil {
		return nil, nil, err
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("ispins start inside an active dungeon")
	}
	w.ispins = &ispinsRun{}
	entry, err := legion.IspinsEntryCharacterInfo(false, [4]bool{}, [5]byte{})
	if err != nil {
		return nil, nil, err
	}
	info, err := legion.IspinsInfoPayload("initial", [5]byte{})
	if err != nil {
		return nil, nil, err
	}
	return []outboundPacket{
			{"ispins_entry_character_info", 0, legion.NotiIspinsEntryCharacterInfo, entry},
			{"ispins_info_initial", 0, legion.NotiIspinsInfo, info},
			{"ispins_start_ack", 1, legion.CmdStart, legion.IspinsStartAck(ispinsStartAckNonce)},
		}, []map[string]any{{
			"kind":         "ispins_started",
			"character_id": w.role.ID,
			"stage_order":  "official default (nemaug, nagor, ashcore, itrenog)",
		}}, nil
}

// ispinsWaitInfo builds the N2255 waiting-state body for the next stage.
func (w *worldSession) ispinsWaitInfo() ([]byte, error) {
	return legion.IspinsInfoPayload(fmt.Sprintf("wait%d", w.ispinsClearedCount()), [5]byte{})
}

// ispinsOperation handles CMD2047. Variant A selects an operation (official
// order: N2255 chosen precedes ACK2047-A); variant B confirms it, followed by
// the official confirm burst (N22/N1719/N1377×3, §1.2).
func (w *worldSession) ispinsOperation(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeIspinsOperationSelect(p)
	if err != nil {
		return nil, nil, err
	}
	if w.ispins == nil {
		return nil, nil, fmt.Errorf("ispins operation before start (no CMD2043 yet)")
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("ispins operation inside a dungeon")
	}
	count := w.ispinsClearedCount()
	switch req.Variant {
	case 1:
		w.ispins.confirmed = false
		info, err := legion.IspinsInfoPayload(fmt.Sprintf("chosen%d", count), [5]byte{})
		if err != nil {
			return nil, nil, err
		}
		return []outboundPacket{
				{"ispins_info_chosen", 0, legion.NotiIspinsInfo, info},
				// @12:15 是 LE unix 时间戳（语义已闭环），用当前时间。
				{"ispins_operation_ack_a", 1, legion.CmdIspinsOperationSelect, legion.IspinsOperationAckA(uint32(time.Now().Unix()), ispinsOperationAckANonces[count])},
			}, []map[string]any{{
				"kind":         "ispins_operation_selected",
				"character_id": w.role.ID,
				"token":        fmt.Sprintf("%x", req.Token),
			}}, nil
	case 2:
		w.ispins.confirmed = true
		plan, err := appendIspinsReplays([]outboundPacket{}, "confirm_echo_b", "confirm_support", "confirm_quest_a", "confirm_quest_b", "confirm_quest_c")
		if err != nil {
			return nil, nil, err
		}
		ack := outboundPacket{"ispins_operation_ack_b", 1, legion.CmdIspinsOperationSelect, legion.IspinsOperationAckB(req.Auxiliary, ispinsOperationAckBNonce)}
		return append([]outboundPacket{ack}, plan...), []map[string]any{{
			"kind":         "ispins_operation_confirmed",
			"character_id": w.role.ID,
			"counter":      req.Counter,
			"auxiliary":    req.Auxiliary,
		}}, nil
	case 4:
		// Local ACK2047 handler 142530950 reads action u32, selects action4
		// at142530a1e, resets window644 at142530a58, then opens selection.
		// This is a change-operation request, not a new difficulty value.
		w.ispins.confirmed = false
		return []outboundPacket{{"ispins_operation_reset_ack", 1, legion.CmdIspinsOperationSelect, legion.IspinsOperationResetAck(uint32(time.Now().Unix()))}},
			[]map[string]any{{"kind": "ispins_operation_reset", "character_id": w.role.ID}}, nil
	}
	return nil, nil, fmt.Errorf("ispins operation variant %d unknown", req.Variant)
}

// enterIspinsStage handles CMD2045: the 18-packet enter burst (§1.3). The
// official replay frames are interleaved with this server's real maze data
// (N2/N28/N29) exactly where the official run carried them.
func (w *worldSession) enterIspinsStage(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeIspinsEnter(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.ispins
	if run == nil {
		return nil, nil, fmt.Errorf("ispins enter before start (no CMD2043 yet)")
	}
	if !run.confirmed {
		return nil, nil, fmt.Errorf("ispins enter before the operation was confirmed (no CMD2047-B yet)")
	}
	stage := int(req.Stage)
	if stage < 0 || stage > 3 {
		return nil, nil, fmt.Errorf("ispins enter stage %d out of range", stage)
	}
	if run.cleared[stage] {
		return nil, nil, fmt.Errorf("ispins stage %d already cleared", stage)
	}
	if stage != w.ispinsClearedCount() {
		return nil, nil, fmt.Errorf("ispins v1 follows the official default order; enter stage %d, want %d", stage, w.ispinsClearedCount())
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("ispins enter while a dungeon is active")
	}
	sel := protocol.DungeonSelection{ID: legion.IspinsStageDungeons[stage], Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ispins stage %d entry unavailable: %w", stage, err)
	}
	// [ISPINS-ARENA-BOSS] 军团阶段本的单场 boss 战就发生在进图房间（源迷宫的
	// [boss map] 是未使用元数据，100002987 的 boss 坐标在 (0,0)/100006472）：
	// 官服 s4 实证进 start 房 (1,1)/100006476 开打、客户端在进图房间发 CMD117
	// （帧 337 target=255）、官服回 N115（帧 495 回显该 target）并结算。置位后
	// BossCheck/完成判定把当前房间当 boss 房，目标校验照旧（须为房内真实源领主）。
	// N28 仍回源迷宫的 boss 坐标（官服帧 449 亦然，Boss=(0,0)）。
	s.ArenaBoss = true
	noteMazeEntry(s)
	seed, err := randomSeed()
	if err != nil {
		return nil, nil, err
	}
	start, err := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: seed, Map: s.Room.Map, Monsters: s.Monsters, EncodeCreateTrigger: monsterCreateTriggerEnabled()})
	if err != nil {
		return nil, nil, err
	}
	plan, err := appendIspinsReplays(nil, "member_premium_info", "area_objects")
	if err != nil {
		return nil, nil, err
	}
	// 私有视觉/装备/宠物段（dungeonEntryPlan 同源数据，官服此处为 N2/N14/
	// N105/N102 各自携带的当次角色数据）。
	if w.characters != nil {
		visual, err := w.characters.EntryBasicProbe(w.role, [2]byte{})
		if err == nil {
			plan = append(plan, outboundPacket{"dungeon_actor_appearance_sent", 0, 2, visual})
		}
		addition, err := w.characters.EntryAddition(w.role)
		if err == nil {
			plan = append(plan, outboundPacket{"dungeon_actor_addition_sent", 0, 2, addition})
		}
		wornUpdate, err := inventory.WornSpaceUpdate(w.role.State)
		if err == nil && len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"dungeon_worn_visuals_sent", 0, 14, wornUpdate})
		}
		if inventory.HasEquippedCreature(w.role.State) {
			clPayload, err := inventory.CreatureListPayload(w.role.State)
			if err == nil {
				plan = append(plan, outboundPacket{"dungeon_creature_list_sent", 0, 105, clPayload})
				growth, err := inventory.CreatureGrowthPayload(w.role.State)
				if err == nil {
					plan = append(plan, outboundPacket{"dungeon_creature_growth_sent", 0, 102, growth})
				}
			}
		}
	}
	if w.soloPartyBootstrap {
		party, e := protocol.SoloPartyInfo(w.role.WireID)
		if e != nil {
			return nil, nil, e
		}
		plan = append(plan, outboundPacket{"solo_party_initialized", 0, 9, party})
	}
	plan, err = appendIspinsReplays(plan, "udp_host", "infinite_difficulty_user", "infinite_difficulty_charac", "enter_select_dungeon")
	if err != nil {
		return nil, nil, err
	}
	info, err := legion.IspinsInfoPayload(fmt.Sprintf("dungeon%d", stage), [5]byte{})
	if err != nil {
		return nil, nil, err
	}
	plan = append(plan, outboundPacket{"ispins_info_dungeon", 0, legion.NotiIspinsInfo, info})
	plan, err = appendIspinsReplays(plan, "fatigue_acceleration", "stackable_dungeon_limit")
	if err != nil {
		return nil, nil, err
	}
	// N28/N29 携带真实迷宫数据（副本 id / 地图 id / boss）。
	plan = append(plan, outboundPacket{"dungeon_info_sent", 0, 28, protocol.DungeonInfo(protocol.DungeonInfoState{ID: sel.ID, Difficulty: sel.Difficulty, Maze: s.Maze.Index, Boss: s.Maze.Boss})})
	plan, err = appendIspinsReplays(plan, "linked_dungeon_info")
	if err != nil {
		return nil, nil, err
	}
	plan = append(plan, outboundPacket{"dungeon_start_map_sent", 0, 29, start})
	plan, err = appendIspinsReplays(plan, "monster_move_system", "secret_shop_event", "character_buff_dungeon")
	if err != nil {
		return nil, nil, err
	}
	plan = append(plan, outboundPacket{"ispins_enter_ack", 1, legion.CmdEnterDungeon, legion.IspinsEnterAck(byte(stage), ispinsEnterAckNonces[stage])})
	plan, err = appendIspinsReplays(plan, "user_state", "noti_390", "timer_sync", "noti_30")
	if err != nil {
		return nil, nil, err
	}
	// 会话样板与主循环 pending 机制一致（main.go 4299-4323）。
	w.activeDungeon = s
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.leaveScene()
	run.stage = stage
	return plan, []map[string]any{{
		"kind":         "ispins_stage_entered",
		"character_id": w.role.ID,
		"stage":        stage,
		"dungeon":      s.Definition.ID,
		"maze":         s.Maze.Index,
		"map":          s.Room.Map,
		"monsters":     len(s.Monsters),
	}}, nil
}

// [ISPINS-AUX-REPLAY] 结算链辅助包（next79 §26，2026-10-03 七测）。官服 s4
// 的阶段结算链在 N31/N2252/N2253/N115 前后夹着整组辅助包（N2204 军团场地
// 对象、N2201 星团、N279 延迟统计、N2168 训练计数器、N14 奖励物品台账），
// 六/七测逐字节对齐主链后仍闪退，剩余偏差即这组包缺失。Body 全部从官服
// 四阶段抓包 verbatim 回放（存档 next79-aux-bodies.json）；N14 的 count 是
// 官服会话的累计值，物品本身不入库（v1 限制）。
type ispinsAuxPacket struct {
	Name string
	ID   uint16
	Body []byte
}

// decodeHexOrDie 只在包初始化时解析表内 hex；坏数据直接 panic（编程错误）。
func decodeHexOrDie(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("ispins aux table hex: " + err.Error())
	}
	return b
}

func appendIspinsAux(plan []outboundPacket, aux []ispinsAuxPacket) []outboundPacket {
	for _, a := range aux {
		plan = append(plan, outboundPacket{a.Name, 0, a.ID, a.Body})
	}
	return plan
}

var ispinsAuxPre31 = [4][]ispinsAuxPacket{
	{ // stage 0
		{"ispins_legion_field_object", 2204, decodeHexOrDie("0100000090000000cd0200000100000003000000000000880109a33a00000000")},
		{"ispins_star_cluster", 2201, decodeHexOrDie("900000000000000001000000cd02000001000000229ddd0a3e00000000000000")},
		{"ispins_lag_statistics", 279, decodeHexOrDie("e8030000d52fe4f43300000000000000")},
	},
	{ // stage 1
		{"ispins_legion_field_object", 2204, decodeHexOrDie("0100000092000000d6020000010000000300000000000041903dc03300000000")},
		{"ispins_star_cluster", 2201, decodeHexOrDie("920000000000000001000000d60200000100000048a206ea3a00000000000000")},
		{"ispins_lag_statistics", 279, decodeHexOrDie("e8030000d52fe4f43300000000000000")},
	},
	{ // stage 2
		{"ispins_legion_field_object", 2204, decodeHexOrDie("010000008f000000c802000001000000030000000000001f1a63ec3300000000")},
		{"ispins_star_cluster", 2201, decodeHexOrDie("8f0000000000000001000000c8020000010000007e9fbcdd3a00000000000000")},
		{"ispins_lag_statistics", 279, decodeHexOrDie("e8030000d52fe4f43300000000000000")},
	},
	{ // stage 3
		{"ispins_legion_field_object", 2204, decodeHexOrDie("0100000091000000d10200000100000003000000000000321c51973600000000")},
		{"ispins_star_cluster", 2201, decodeHexOrDie("910000000000000001000000d102000001000000d3d057794300000000000000")},
		{"ispins_lag_statistics", 279, decodeHexOrDie("e8030000d52fe4f43300000000000000")},
	},
}
var ispinsAuxPre2252 = [4][]ispinsAuxPacket{
	{ // stage 0
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006b27430001000000010000000000000000000000ffffffff000000000000cf4da675430000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006f27430001000000010000000000000000000000ffffffff000000000000a00ed835440000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("0000000001000000731e060001000000010000000000000000000000ffffffff000000000000a3ff1d633d0000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006e27430001000000140000000000000000000000ffffffff000000000000c90beea8390000000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d001400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000066d3c2763c000000")},
	},
	{ // stage 1
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006c27430001000000010000000000000000000000ffffffff00000000000027a9cab0370000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006f27430001000000020000000000000000000000ffffffff00000000000029aee78a350000000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d004400000000000000000000000000000000000000ca31b100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006298437242000000")},
	},
	{ // stage 2
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006a27430001000000010000000000000000000000ffffffff000000000000f00b73ed340000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006f27430001000000030000000000000000000000ffffffff000000000000bdf9feb5390000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("0000000001000000721e060001000000010000000000000000000000ffffffff0000000000007a6b509c360000000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d0074000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f38bd9f237000000")},
	},
	{ // stage 3
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006927430001000000010000000000000000000000ffffffff0000000000005ec265d93c0000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006f27430001000000040000000000000000000000ffffffff000000000000123a48b73c0000000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d00a400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000085f4061045000000")},
	},
}
var ispinsAuxPre2253 = [4][]ispinsAuxPacket{
	{ // stage 0
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006e27430001000000300000000000000000000000ffffffff00000000000000bb89e7360000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006e274300010000003c0000000000000000000000ffffffff000000000000d851ec74400000000000")},
		{"ispins_training_counter", 2168, decodeHexOrDie("00000000010000006e27430001000000640000000000000000000000ffffffff0000000000009d0ffa603f0000000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d0030000000000000000000000000000000000000008da4940bf315564a46754a4e2b0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000019c52efd37000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008000bdb19d000c000000000000000000000000000000000000008da49495aac7ed73c4120b4ecb00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000d3c3e43245000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008100f3949d0032000000000000000000000000000000000000008da494c376f444ac07266c83d500000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c84966143c000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008200f6949d001900000000000000000000000000000000000000670ce75a43b7448e5a75f46ee9000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f40100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008c5109c23a000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007d00f1949d006d00000000000000000000000000000000000000789128877bf045a2789128877b00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ad57144f45000000")},
	},
	{ // stage 1
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d0060000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000451dece135000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008000bdb19d001800000000000000000000000000000000000000fa61bd000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008a115df34000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008100f3949d006400000000000000000000000000000000000000f8fa2c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000009f6ed0633d000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007d00f1949d00d1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000edc05b243d000000")},
	},
	{ // stage 2
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d0090000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ba17e8e93a000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008000bdb19d002400000000000000000000000000000000000000f24c8400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006ccb84ce3a000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008100f3949d009600000000000000000000000000000000000000275d8f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006448662744000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007d00f1949d0035010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000b61cda1242000000")},
	},
	{ // stage 3
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007f00bcb19d00c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a7fe73d63b000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008000bdb19d0030000000000000000000000000000000000000003c8dfc0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000841ef98136000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008100f3949d00c800000000000000000000000000000000000000b1635b00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007377f5e837000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001007d00f1949d00990100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003439e0923a000000")},
		{"ispins_reward_item_granted", 14, decodeHexOrDie("0001008300200e9e000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000041b0dce03b000000")},
	},
}
var ispinsAuxPre115 = [4][]ispinsAuxPacket{
	{ // stage 0
		{"ispins_training_counter", 2168, decodeHexOrDie("000000000100000034f4530001000000070000000000000000000000ffffffff0000000000001e9c99ea340000000000")},
	},
	{ // stage 1
		{"ispins_training_counter", 2168, decodeHexOrDie("000000000100000034f4530001000000080000000000000000000000ffffffff000000000000fbf3baa53b0000000000")},
	},
	{ // stage 2
		{"ispins_training_counter", 2168, decodeHexOrDie("000000000100000034f4530001000000090000000000000000000000ffffffff0000000000002d331de7370000000000")},
	},
	{ // stage 3
		{"ispins_training_counter", 2168, decodeHexOrDie("000000000100000034f45300010000000a0000000000000000000000ffffffff000000000000249fab11440000000000")},
	},
}

// completeIspinsStage is the per-stage settlement chain (§1.4), entered from
// completeDungeon. The chain is byte-aligned with official s4 stage0 frames
// 468-496 (next79 §24/§26/§27): aux(2204/2201/279) → N31 → N2256(stage≠2) →
// aux(2168×n/14) → N2252 → aux(2168×n/14×n) → N2 (settlement character info,
// locally rebuilt with the role name/WireID, §27) → N2253 → N2254 (stage flags
// lit) → N9 (steady party re-affirmation, §27) → N2255 clear → N1658 (empty) →
// aux(2168) → N115 (stage0/3 only). The official N2/N9 bodies embed
// official-session actor/role names and the new-client NOTI9 layout, so both
// are locally rebuilt (protocol.IspinsSettlementCharacterInfo / SoloPartyInfo);
// the reward items in the N14 echoes are not persisted in v1.
func (w *worldSession) completeIspinsStage() ([]outboundPacket, error) {
	if !w.activeDungeon.Completed() || w.completionSent {
		return nil, nil
	}
	run := w.ispins
	if run == nil {
		return nil, nil
	}
	stage := run.stage
	basic, err := legion.IspinsBasicClearReward(stage)
	if err != nil {
		return nil, err
	}
	additional, err := legion.IspinsAdditionalClearReward(stage)
	if err != nil {
		return nil, err
	}
	clearInfo, err := legion.IspinsInfoPayload(fmt.Sprintf("clear%d", stage), [5]byte{})
	if err != nil {
		return nil, err
	}
	run.cleared[stage] = true
	entry, err := legion.IspinsEntryCharacterInfo(false, run.cleared, [5]byte{})
	if err != nil {
		return nil, err
	}
	// 链序逐字节对齐官服 s4（帧 468-496，next79 §26/§27）。官服在主链包之间
	// 还夹着整组辅助包（2204/2201/279、2168、14），此前缺失即七测仍闪退
	// 的剩余偏差；链尾 N2/N9（内嵌官服会话 actor/角色名）八测证明同样
	// 不可省，本地构造见下（§27）。
	plan := appendIspinsAux(nil, ispinsAuxPre31[stage])
	// 主循环按这个名字置 completionSent（与通用 N31 同名）。
	plan = append(plan, outboundPacket{"dungeon_clear_enabled", 0, 31, legion.IspinsDungeonClearEnabled(stage)})
	// 官服 N2256 只在 stage 0/1/3 出现，stage2 直接 2168（帧 656→657）。
	if stage != 2 {
		plan = append(plan, outboundPacket{"ispins_operation_notice", 0, legion.NotiIspinsOperation, legion.IspinsOperationNotice()})
	}
	plan = appendIspinsAux(plan, ispinsAuxPre2252[stage])
	plan = append(plan, outboundPacket{"ispins_basic_clear_reward", 0, legion.NotiIspinsBasicClearReward, basic})
	plan = appendIspinsAux(plan, ispinsAuxPre2253[stage])
	// The newer official N2 template is incompatible with the local equipment
	// reader (145639906 ->1459a0220 ->146d77f50 ->146ea0be0). The live
	// next79 §30 stack proves it reads beyond that body. Refresh through the
	// same native basic/detail encoders already accepted at party creation
	// and dungeon entry, using this role's persisted data.
	if w.characters != nil {
		basicInfo, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if err != nil {
			return nil, err
		}
		detailInfo, err := w.characters.EntryAddition(w.role)
		if err != nil {
			return nil, err
		}
		plan = append(plan,
			outboundPacket{"ispins_settlement_character_info", 0, 2, basicInfo},
			outboundPacket{"ispins_settlement_character_detail", 0, 2, detailInfo},
		)
	}
	plan = append(plan,
		outboundPacket{"ispins_additional_clear_reward", 0, legion.NotiIspinsAdditionalClearReward, additional},
		outboundPacket{"ispins_entry_character_info", 0, legion.NotiIspinsEntryCharacterInfo, entry},
	)
	// 官服帧 491：N2254 与 N2255 之间还有一帧稳态 N9（176B 新版布局，
	// p[4:6]=105，verbatim 回放触发 §21.6 成员数越界崩溃），本地改发
	// SoloPartyInfo——进图时客户端已接受的同型帧，重申作战中队伍状态。
	party, err := protocol.SoloPartyInfo(w.role.WireID)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"ispins_settlement_party_steady", 0, 9, party})
	plan = append(plan,
		outboundPacket{"ispins_info_clear", 0, legion.NotiIspinsInfo, clearInfo},
		// [ISPINS-1658-EMPTY-BODY] 官服 s4 帧 493：N1658 是 16B 纯头空包
		//（body=0，checksum=0x18）。preparePackets 旧版静默丢弃一切空 body，
		// 客户端在官服链的 2255 之后、N115 之前等不到 1658（next79 §25
		// 六测仍闪退的根因之一）；非 nil 空 slice 语义 = 真空包放行。
		outboundPacket{"ispins_req_dungeon_clear_info", 0, 1658, []byte{}},
	)
	plan = appendIspinsAux(plan, ispinsAuxPre115[stage])
	// [ISPINS-ARENA-BOSS] 官服 s4 的阶段结算链以 N115 收尾（帧 495
	// 16B `01 01 <target> <5B token> <7B零>`，回显客户端 CMD117 报的
	// 完成目标），位置在整个 N31 家族之后、客户端 CMD2046（帧 496）之前。
	// 官服只在 stage0/3 有 CMD117→N115（c2s 帧 337/488），stage1/2 不发
	//（IspinsBossCheckConfirmed 返回 nil）。CompletionTarget 优先取
	// CMD117 上报值，死亡驱动结算时回退为房内 rank3 领主实体。
	if stage == 0 || stage == 3 {
		if target := w.activeDungeon.CompletionTarget(); target != 0 {
			confirmed, err := protocol.IspinsBossCheckConfirmed(target, stage)
			if err != nil {
				return nil, err
			}
			if confirmed != nil {
				plan = append(plan, outboundPacket{"boss_check_confirmed", 0, 115, confirmed})
			}
		}
	}
	return plan, nil
}

// ispinsRewardEnd handles CMD2046. Non-final stages answer with ACK2046 plus
// the next waiting state; the client then leaves via CMD72 (the generic
// settlement-exit path). The final stage pushes N2255 final before the ack
// (§1.5) and arms the CMD191 story branch.
func (w *worldSession) ispinsRewardEnd(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, next, err := legion.DecodeIspinsRewardEnd(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.ispins
	if run == nil {
		return nil, nil, fmt.Errorf("ispins reward end without a run (no CMD2043 yet)")
	}
	stage := int(req.Stage)
	if stage < 0 || stage > 3 || !run.cleared[stage] {
		return nil, nil, fmt.Errorf("ispins reward end stage %d is not a cleared stage", req.Stage)
	}
	// 官服 next 恒为 (stage+2)%4（next78 §1.4；文档 §1.4 表格的 (stage+1)%4
	// 为笔误，四帧实测均为 +2）。
	ackNext := byte((uint32(stage) + 2) % 4)
	all := run.cleared[0] && run.cleared[1] && run.cleared[2] && run.cleared[3]
	note := map[string]any{
		"kind":         "ispins_reward_end",
		"character_id": w.role.ID,
		"stage":        stage,
		"request_next": next,
	}
	if stage == 3 && all {
		run.finalDone = true
		final, err := legion.IspinsInfoPayload("final", [5]byte{})
		if err != nil {
			return nil, nil, err
		}
		return []outboundPacket{
			{"ispins_info_final", 0, legion.NotiIspinsInfo, final},
			{"ispins_reward_end_ack", 1, legion.CmdRewardEnd, legion.IspinsRewardEndAck(byte(stage), ackNext, ispinsRewardEndAckNonces[stage])},
		}, []map[string]any{note}, nil
	}
	wait, err := w.ispinsWaitInfo()
	if err != nil {
		return nil, nil, err
	}
	return []outboundPacket{
		{"ispins_reward_end_ack", 1, legion.CmdRewardEnd, legion.IspinsRewardEndAck(byte(stage), ackNext, ispinsRewardEndAckNonces[stage])},
		{"ispins_info_wait", 0, legion.NotiIspinsInfo, wait},
	}, []map[string]any{note}, nil
}

// ispinsStoryPause answers CMD191 during the Ispins finale. The generic 191
// handler requires an active dungeon, which is already gone by then; the
// official N170 bodies are the ispins family's own (f787/f793). On resume the
// leave state and the all-clear N2254 follow (f792/f813); the official run
// spaced them across the cutscene, which this server cannot time, so they are
// delivered with the resume.
func (w *worldSession) ispinsStoryPause(p []byte) ([]outboundPacket, []map[string]any, error) {
	r, err := protocol.DecodeStoryPause(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.ispins
	if run == nil || !run.finalDone {
		return nil, nil, fmt.Errorf("ispins story pause outside the finale")
	}
	resumed := r.State == 1
	nonce := ispinsStoryPauseNonce
	if resumed {
		nonce = ispinsStoryResumeNonce
	}
	plan := []outboundPacket{{"ispins_story_pause", 0, 170, legion.IspinsStoryPause(resumed, nonce)}}
	if resumed {
		leave, err := legion.IspinsInfoPayload("leave", [5]byte{})
		if err != nil {
			return nil, nil, err
		}
		entry, err := legion.IspinsEntryCharacterInfo(false, run.cleared, [5]byte{})
		if err != nil {
			return nil, nil, err
		}
		plan = append(plan,
			outboundPacket{"ispins_info_leave", 0, legion.NotiIspinsInfo, leave},
			outboundPacket{"ispins_entry_character_info", 0, legion.NotiIspinsEntryCharacterInfo, entry},
		)
		// CMD191 only finishes the movie. Clearing ispins here sends the
		// subsequent CMD72 through ordinary cardsReady, which this legion
		// never uses. Release it only on town departure or party leave.
		run.storyFinished = true
	}
	return plan, []map[string]any{{
		"kind":         "ispins_story_pause",
		"character_id": w.role.ID,
		"state":        r.State,
	}}, nil
}
