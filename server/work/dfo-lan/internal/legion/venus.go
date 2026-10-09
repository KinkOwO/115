// Venus (美神维纳斯, content id 106) wire contract — channel 99 standby and
// run flow.
//
// Evidence sources:
//   - Live 2.38.2 client requests: CMD12 standby party (2026-10-04 10:03
//     session, type 0x22) and CMD2043 start (2026-10-04 11:11 session,
//     24B: 13B envelope + content 106 + 7 zero bytes, shape = ispins s4
//     frame).
//   - 包规格-全流程 (09-军团维纳斯专有补收 / 10-军团末世录专有) native-reader
//     evidence: N2655 85B layout, CMD2290 18B request / 15B ACK, family
//     shared ACK readers (CMD2043 = 01+4B result, CMD2045/2046 = 01+13B).
//   - PVF direct read: contents/2025/venus/etc/venus.cos (60s selection
//     window, operation index 1/2/3/5) and list/dungeon.lst mapping
//     100003921..100003924 to Venus_1Phase..4Phase.dgn.
//
// Unimplemented branches (CMD2293, in-dungeon N2655 states) must stay explicit
// refusals upstream — do not answer them with invented bodies. CMD2044 is
// implemented as the abandon path; the in-dungeon retreat button itself speaks
// CMD72 and is answered by the settlement-exit host (cmd/wireprobe card_flow),
// mirroring Ispins. CMD2290 action4 (re-select) answers with the unchosen
// N2655 followed by the action4 ACK.
package legion

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

// VenusContentID is the u32 carried at body offset 13 of the Venus
// CMD2043/2045/2046 requests. The apocalypse family carries 107 and Ispins
// 101 on the same offsets, so this value is the family discriminator.
const VenusContentID uint32 = 106

// NotiVenusInfo is the Venus-only state snapshot (85B) — the analogue of the
// apocalypse N2895 and the ispins N2255. The two layouts are NOT compatible
// (2655 doc: 不能拿N2895的206B形状或内容107前缀用于维纳斯).
const NotiVenusInfo uint16 = 2655

// VenusStageDungeons maps the run stage (0-based) to its PVF dungeon id:
// list/dungeon.lst rows Contents/2025/Venus/Dungeon/Venus_{1..4}Phase.dgn,
// the same four ids the channel directory carries as CountDungeons.
var VenusStageDungeons = [4]uint32{100003921, 100003922, 100003923, 100003924}

// IsVenusStageDungeon reports whether the id is one of the four Venus phase
// dungeons. In-dungeon retreat handling (CMD72 / CMD2044) is scoped to them.
func IsVenusStageDungeon(id uint32) bool {
	for _, v := range VenusStageDungeons {
		if v == id {
			return true
		}
	}
	return false
}

// VenusStageOfDungeon maps a Venus phase dungeon id back to its 0-based stage
// index. The native stage transition direct-moves (CMD2062) into the next
// phase dungeon resolved from the N2655 stage record, so the wire id is the
// authoritative stage selector on that path.
func VenusStageOfDungeon(id uint32) (int, error) {
	for i, v := range VenusStageDungeons {
		if v == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("dungeon %d is not a venus stage dungeon", id)
}

// VenusPhase1Carriers are the three relic-carrier monsters of the first phase.
//
// The four phase maps partition the five relic-carrier templates: venus_2phase
// carries a static [fixed][boss] row for 109016983, venus_3phase for
// 109016984, venus_4phase for 109016985 (Venus herself) — while venus_1phase
// has NO [monster] section at all. The seven-relic table in venus.cos binds
// the remaining templates 109016980 (scissors) / 109016981 (perfume) /
// 109016982 (tears) to no later phase, and the 2291 source evidence records
// 首关三怪生成 — three carriers spawn in the first phase. Without a server
// roster the stage enter ships an empty N29 and the room has nothing to fight
// (2026-10-04 12:27 session venus_stage_entered monsters:0).
var VenusPhase1Carriers = [3]uint32{109016980, 109016981, 109016982}

// VenusSelectionSeconds is the operation-selection window duration from the
// source table (venus.cos, 2290 doc: 源表60秒是时长，需加到冻结开始秒数形成
// 截止时间). BUG5（第三十二轮，业主口径）：实机选难度倒计时改为 15 秒
// （60 秒太长），超时仍走 venusOperationClose 的原生 close ACK 自动关窗。
const VenusSelectionSeconds uint32 = 15

// VenusPhaseLimits is the per-stage countdown limit (seconds) announced as
// NOTI1474 DUNGEON_TIMEOUT_TIME when a phase dungeon finishes loading.
// Channels 99/119 read seconds (1452AEC10 first u32); the milliseconds
// exception is bleeding mine channel 106 only. Values: 600s for
// Venus_1..3Phase ([dungeon timeout] 600 in the phase DGNs) and 900s for the
// descent 4th phase (2026-10-05 用户口径；G0721b 原生向量同样覆盖 99 的
// 600/900 秒起点)。30 秒临时测试值已于 2026-10-05 验收后改回正式值。
//
// 第四十四轮（2026-10-07）实机结论：这两个值**不能超过客户端本地 DGN 的
// [dungeon timeout] 上限（600/900）**——HUD 按「结束秒−同步服务器秒」再与
// 当前 DGN 上限取较小值（1474 规格文档 G0728b 逆向结论），发 1800/3600 会被
// 钳死在 10:00/15:00 冻结不走秒（业主实测）。第四十三轮曾按业主口径放宽到
// 1800/1800/1800/3600，实测显示冻结后于第四十四轮回退。苏醒之森 60 分钟
// 显示正常是因为森林 DGN 源上限本就是 3600（ForestStageLimits 与之一致），
// 并非客户端不钳制。改这里之前先核对对应 phase DGN 的 [dungeon timeout]。
var VenusPhaseLimits = [4]uint32{600, 600, 600, 900}

// CmdVenusPhaseRevive is CMD2059 (ENUM_CMDPACKET_PLAYER_REVIVE_WHEN_PHASE_
// CHANGE): the client's free-revive request for a phase-change kill, gated by
// the stage DGN's `[player revive when phase change tag max count]` row.
const CmdVenusPhaseRevive uint16 = 2059

// VenusPhaseReviveAck replays the 16B ACK2059 shape observed on the official
// Ispins run (legion.IspinsReviveAck: 01 … ed … 94f5c28b3c, constant across
// all eight captured frames). The same opcode serves the Venus phase-change
// revive; no Venus-specific ACK sample exists, so the family shape is reused.
func VenusPhaseReviveAck() []byte {
	return IspinsReviveAck()
}

// VenusRelicSources is the seven (relic id → carrier template) pairs from the
// 2291 source table (venus.cos Catch 目录逐对核实)：0 镜 109016984、
// 1 泪 109016982、2 经 109016984、3 冠 109016983、4 带 109016983、
// 5 剪 109016980、6 香 109016981。模板有重复（109016984/109016983 各承载两件
// 圣物），校验必须按 (模板, ID) 成对进行，不能只看模板。
var VenusRelicSources = [7]uint32{109016984, 109016982, 109016984, 109016983, 109016983, 109016980, 109016981}

// DecodeVenusRelic reads the CMD2291 request: 13B opaque envelope + u32
// monster template @13 + u8 relic id @17 (18B logical). The template is a
// monster template, not an entity id; the relic id is 0..6.
func DecodeVenusRelic(p []byte) (template uint32, relic byte, err error) {
	if len(p) < EnvelopeSize+5 {
		return 0, 0, fmt.Errorf("venus relic payload %d bytes, want at least %d", len(p), EnvelopeSize+5)
	}
	template = binary.LittleEndian.Uint32(p[EnvelopeSize:])
	relic = p[EnvelopeSize+4]
	if relic > 6 {
		return template, relic, fmt.Errorf("venus relic id %d out of range 0..6", relic)
	}
	return template, relic, nil
}

// VenusRelicAck builds the CMD2290 family's CMD2291 success reply: common 01
// plus the 5B block the reader only reads back in (template u32, relic id u8).
func VenusRelicAck(template uint32, relic byte) []byte {
	p := make([]byte, 6)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], template)
	p[5] = relic
	return p
}

// VenusRelicRefused builds the CMD2291 refusal: common 00 plus a neutral u16
// 0 (2291 doc: reader ignores the reason word but the pending registration is
// still cleared, so a refusal unblocks the client UI).
func VenusRelicRefused() []byte { return []byte{0, 0, 0} }

// Venus endpoint per the native destination getter 141C390D0 (2655 doc):
// Choice FF -> -1, Choice 2 (降临) -> 3, any other choice -> 2. The endpoint
// is the last stage index the run reaches: normal runs stages 0..2, descent
// adds stage 3.
func VenusEndpoint(choice byte) int {
	if choice == 2 {
		return 3
	}
	return 2
}

// VenusRequests reports whether the id belongs to the Venus command family
// beyond the shared 2043/2045/2046 envelope. The Venus-specific opcodes are
// declared in session.go next to the shared legion opcodes.
func VenusRequests(id uint16) bool {
	switch id {
	case CmdVenusOperationSelect, CmdVenusRelic, CmdVenusEndAtPhase4:
		return true
	default:
		return false
	}
}

// DecodeVenusStart reads the CMD2043 request. Live body (24B): 13B opaque
// envelope + content u32 == 106 + 7 zero bytes.
func DecodeVenusStart(p []byte) error {
	if len(p) < EnvelopeSize+4 {
		return fmt.Errorf("venus start payload %d bytes, want at least %d", len(p), EnvelopeSize+4)
	}
	if content := binary.LittleEndian.Uint32(p[EnvelopeSize:]); content != VenusContentID {
		return fmt.Errorf("venus start content %d, want %d", content, VenusContentID)
	}
	return nil
}

// VenusOperationRequest is the decoded CMD2290 body: 13B envelope + u32
// action @13 + u8 choice @17 (18B logical; transport padding allowed).
// Action 1 opens the selection window (choice 255), 2 confirms the chosen
// difficulty, 4 asks for a re-selection.
type VenusOperationRequest struct {
	Action uint32
	Choice byte
}

func DecodeVenusOperation(p []byte) (VenusOperationRequest, error) {
	if len(p) < EnvelopeSize+5 {
		return VenusOperationRequest{}, fmt.Errorf("venus operation payload %d bytes, want at least %d", len(p), EnvelopeSize+5)
	}
	return VenusOperationRequest{
		Action: binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Choice: p[EnvelopeSize+4],
	}, nil
}

// VenusOperationAck builds the CMD2290 response: common success byte plus the
// 14B block (2290 doc offsets are relative to the byte after the common 01):
// u32 action, u8 FF (reader initial, unconsumed), u8 close (1 closes the
// window), u32 deadline server-seconds, u8 submode, 3B reserved. Action 2
// never consumes close/deadline/submode — it goes straight to the confirm
// function — so zero values are exact there.
func VenusOperationAck(action uint32, close bool, deadlineSeconds uint32, submode byte) []byte {
	p := make([]byte, 15)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], action)
	p[5] = 0xff
	if close {
		p[6] = 1
	}
	binary.LittleEndian.PutUint32(p[7:], deadlineSeconds)
	p[11] = submode
	return p
}

// DecodeVenusEnter reads the CMD2045 request (ispins s4 shape): content u32
// @13, stage u32 @17 (0-based).
func DecodeVenusEnter(p []byte) (EnterDungeonRequest, error) {
	if len(p) < EnvelopeSize+8 {
		return EnterDungeonRequest{}, fmt.Errorf("venus enter payload %d bytes, want at least %d", len(p), EnvelopeSize+8)
	}
	req := EnterDungeonRequest{
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Stage:      binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}
	if req.Channel != VenusContentID {
		return EnterDungeonRequest{}, fmt.Errorf("venus enter content %d, want %d", req.Channel, VenusContentID)
	}
	return req, nil
}

// VenusInfoState selects the fields the N2655 writer controls. Every byte the
// client's constructor initialises and this server has no reason to change is
// written inside VenusInfo, not exposed here.
type VenusInfoState struct {
	// Choice is the operation selection byte (@2): FF = unselected, 0/1 =
	// normal, 2 = descent. Matching (4) stays refused upstream.
	Choice byte
	// State (@3): 2 = waiting area (documented start state), 0 = fail close.
	State uint32
	// Outcome (@7) and Stage (@11): waiting starts both at 0; Stage advances
	// as stages are published (2655 doc: boss death projects to the next
	// Stage/Target; C2045 takes its stage from this field).
	Outcome uint32
	Stage   uint32
	// Targets are the four stage records' byte0 — the content-table DGN index
	// each stage resolves to. FF = not yet published.
	Targets [4]byte
	// RelicMask (@77): the seven relic bits; 2291 sets bits, never invented.
	RelicMask uint32
}

// VenusInfo builds the 85B N2655 body. Fixed bytes mirror the client
// constructor initialisers recorded in the 2655 doc: head FFFF, Following
// and field19 FFFFFFFF, record +4 u64 FFFFFFFFFFFFFFFF, +1..3 cleared by the
// encoder, zero tail (71..76), zero flag (@81).
func VenusInfo(s VenusInfoState) []byte {
	p := make([]byte, 85)
	p[0], p[1] = 0xff, 0xff
	p[2] = s.Choice
	binary.LittleEndian.PutUint32(p[3:], s.State)
	binary.LittleEndian.PutUint32(p[7:], s.Outcome)
	binary.LittleEndian.PutUint32(p[11:], s.Stage)
	binary.LittleEndian.PutUint32(p[15:], 0xffffffff)
	binary.LittleEndian.PutUint32(p[19:], 0xffffffff)
	for i, t := range s.Targets {
		rec := p[23+12*i:]
		rec[0] = t
		for j := 4; j < 12; j++ {
			rec[j] = 0xff
		}
	}
	binary.LittleEndian.PutUint32(p[77:], s.RelicMask)
	return p
}

// VenusWaitingInfo is the post-CMD2043 waiting state (2043-VENUS_START doc:
// N2655 等待 State2／ChoiceFF／Stage0 及首目标0).
func VenusWaitingInfo() []byte {
	return VenusInfo(VenusInfoState{Choice: 0xff, State: 2, Targets: [4]byte{0, 0xff, 0xff, 0xff}})
}

// VenusFinalInfo is the terminal state that triggers the clear movie: family
// convention with the ispins N2255 "final" (State 3, Outcome 1; the venus
// N2655 has no official capture, so the values mirror the family mapping).
// Sent with the terminal CMD2046 ACK — the client then plays the clear movie.
func VenusFinalInfo(choice byte, stage int, relicMask uint32) []byte {
	return VenusInfo(VenusInfoState{Choice: choice, State: 3, Outcome: 1, Stage: uint32(stage), Targets: stageTargets(stage), RelicMask: relicMask})
}

// VenusLeaveInfo closes the operation panel once the clear movie has finished
// (family convention with the ispins N2255 "leave": State 5, Outcome 1) — the
// top-right panel and the relic display disappear for the finished run.
func VenusLeaveInfo(choice byte, stage int, relicMask uint32) []byte {
	return VenusInfo(VenusInfoState{Choice: choice, State: 5, Outcome: 1, Stage: uint32(stage), Targets: stageTargets(stage), RelicMask: relicMask})
}

// VenusClosedInfo is the run-over close state: State0 is the only documented
// panel-close value (2655 doc: State0 = 失败关闭 — the mode closes). Sent on
// the post-clear town exit and on leaving the party (return-to-character-
// select / CMD13) so the top-right operation panel, the relic display and the
// manager's cached run state all reset for the abandoned/finished run.
func VenusClosedInfo() []byte {
	return VenusInfo(VenusInfoState{Choice: 0xff, State: 0, Targets: [4]byte{0, 0xff, 0xff, 0xff}})
}

// VenusChosenInfo publishes the confirmed difficulty before the CMD2290
// action2 ACK (2290 doc: 选择后的N2655先于Action2 ACK，保证原生确认回调
// 发送C2045前已拿到权威选择).
// BUG3（第二十二轮回归）：Stage 必须携带下一个待进阶段——撤退/失败保留进度
// 后重开时 C2045 确认的不是第 0 关；固定 0 会让客户端请求已通关的第 0 关
// 被拒，表现为「点进入地下城后框关了、人还在城镇」。
func VenusChosenInfo(choice byte, stage int) []byte {
	return VenusInfo(VenusInfoState{Choice: choice, State: 2, Stage: uint32(stage), Targets: stageTargets(stage)})
}

// VenusReopenInfo publishes the unchosen waiting state for CMD2290 action4
// (更改难度重选)：客户端在重选模式下展示三张卡片，权威状态先回 ChoiceFF，
// 新选择仍由后续 action2 确认（与 action2 相同的 N2655 先行时序）。
func VenusReopenInfo(stage int) []byte {
	return VenusInfo(VenusInfoState{Choice: 0xff, State: 2, Stage: uint32(stage), Targets: stageTargets(stage)})
}

// VenusStageAdvancedInfo publishes the next waiting state after a stage's
// reward end: the stage to enter next and its record target (2655 doc:
// 141C39130 按 Stage 查记录 byte0 解析 DGN).
func VenusStageAdvancedInfo(choice byte, stage int) []byte {
	return VenusInfo(VenusInfoState{Choice: choice, State: 2, Stage: uint32(stage), Targets: stageTargets(stage)})
}

// stageTargets 是阶段等待态的四个记录 byte0：当前阶段槽指向自己（解析 DGN
// 的目标索引），其余未发布（FF）；阶段 0 与等待态的 {0,FF,FF,FF} 同形。
func stageTargets(stage int) [4]byte {
	targets := [4]byte{0, 0xff, 0xff, 0xff}
	if stage >= 0 && stage < len(targets) {
		targets[stage] = byte(stage)
	}
	return targets
}

// VenusRelicInfo publishes the in-dungeon state after a relic report: the
// current stage snapshot with the relic mask merged (2655 doc +77 七位集合，
// mask |= 1 << ID；请求者先收权威 N2655 再收 2291 ACK).
func VenusRelicInfo(choice byte, stage int, mask uint32) []byte {
	return VenusInfo(VenusInfoState{Choice: choice, State: 2, Stage: uint32(stage), Targets: stageTargets(stage), RelicMask: mask})
}

// ---------------------------------------------------------------------------
// 终局翻牌奖励（N2252 / N2253）——军团家族共享形状，伊斯实机验证过
// ---------------------------------------------------------------------------

// VenusRewardItem 是一条维纳斯终局奖励（模板 + 数量）。
type VenusRewardItem struct {
	Template uint32
	Amount   uint32
}

// 翻牌材料模板（2026-10-04 用户方案，官服维纳斯翻牌口径，PVF 模板号）。
const (
	VenusRewardAbyssTicket           uint32 = 10362429 // 深渊入场 : 终末之启示
	VenusRewardLooseSilk             uint32 = 10404330 // 飘散的赞语丝绸
	VenusRewardCrackedSilk           uint32 = 10404667 // 碎裂的赞语丝绸
	VenusRewardFusionBox             uint32 = 10404730 // 维纳斯神器套装融合石自选礼盒
	VenusRewardAuctionTicket         uint32 = 10403546 // 维纳斯竞拍参与券
	VenusRewardBehemothTear          uint32 = 10404807 // 天帷巨兽的眼泪
	VenusRewardBehemothTearTradeOnce uint32 = 10404679 // 天帷巨兽的眼泪（可交易1次）
	VenusRewardWitheredPetal         uint32 = 10404346 // 凋零的纯洁花瓣
)

// VenusFlipGearCount 是第一排随机装备位数（前 5 位）。
const VenusFlipGearCount = 5

// VenusBasicRewardMaterials 是第一排第 6-8 位按难度（choice 0/1/2 = 卡片
// I/II/III）给的材料：深渊入场券 10/15/25、飘散的赞语丝绸 60/100/150、
// 碎裂的赞语丝绸 16/20/25。
func VenusBasicRewardMaterials(choice byte) []VenusRewardItem {
	var abyss, loose, cracked uint32
	switch choice {
	case 2:
		abyss, loose, cracked = 25, 150, 25
	case 1:
		abyss, loose, cracked = 15, 100, 20
	default:
		abyss, loose, cracked = 10, 60, 16
	}
	return []VenusRewardItem{
		{Template: VenusRewardAbyssTicket, Amount: abyss},
		{Template: VenusRewardLooseSilk, Amount: loose},
		{Template: VenusRewardCrackedSilk, Amount: cracked},
	}
}

// VenusAdditionalRewardItems 是第二排固定 5 件（不分难度，各 1）：融合石
// 自选礼盒、竞拍参与券、天帷巨兽的眼泪、天帷巨兽的眼泪（可交易1次）、
// 凋零的纯洁花瓣。
func VenusAdditionalRewardItems() []VenusRewardItem {
	return []VenusRewardItem{
		{Template: VenusRewardFusionBox, Amount: 1},
		{Template: VenusRewardAuctionTicket, Amount: 1},
		{Template: VenusRewardBehemothTear, Amount: 1},
		{Template: VenusRewardBehemothTearTradeOnce, Amount: 1},
		{Template: VenusRewardWitheredPetal, Amount: 1},
	}
}

// VenusFlipGearPool 是 configs/venus-flip-gear.generated.json 的导出物：
// 全量装备目录筛出的 115 级魔法(2)/神器(3)常规部位装备，供第一排随机
// 装备位 roll。source 只作溯源记录，不做运行时绑定——发放时 Awarder 逐件
// 走全量装备目录校验，模板失效会在 Grant 处报错而不是在这里。
type VenusFlipGearPool struct {
	Source struct {
		Format   string `json:"format"`
		Checksum string `json:"checksum"`
		Origin   string `json:"origin"`
	} `json:"source"`
	Level     int      `json:"level"`
	Rarity    []int    `json:"rarity"`
	Templates []uint32 `json:"templates"`
}

// LoadVenusFlipGearPool 读已导出的翻牌装备池子。
func LoadVenusFlipGearPool(path string) (VenusFlipGearPool, error) {
	var p VenusFlipGearPool
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	if len(p.Templates) == 0 {
		return p, fmt.Errorf("venus flip gear pool %s has no templates", path)
	}
	return p, nil
}

// VenusRollFlipGear 从池子随机抽 n 件不重复装备（部分 Fisher-Yates，
// crypto/rand）。池子为空或 n<=0 时返回 nil，第一排降级为仅材料位。
func VenusRollFlipGear(pool []uint32, n int) []uint32 {
	if len(pool) == 0 || n <= 0 {
		return nil
	}
	if n > len(pool) {
		n = len(pool)
	}
	buf := append([]uint32(nil), pool...)
	for i := 0; i < n; i++ {
		var v uint32
		if err := binary.Read(rand.Reader, binary.LittleEndian, &v); err != nil {
			return append([]uint32(nil), buf[:n]...)
		}
		j := i + int(v%uint32(len(buf)-i))
		buf[i], buf[j] = buf[j], buf[i]
	}
	return append([]uint32(nil), buf[:n]...)
}

// VenusDungeonClearEnabled builds the 16B N31 with the stage token that the
// N2252 tail must echo. 军团家族共用伊斯的阶段 token 表（官服 s4 抓包值）；
// 客户端只校验 N31 头与 N2252 尾 @7760 的 2B 一致，不校验具体值。
func VenusDungeonClearEnabled(stage int) []byte {
	return IspinsDungeonClearEnabled(stage)
}

// VenusBasicClearReward builds the native 7772B N2252 payload for the Venus
// terminal flip-card first row (8 items). 行布局与 protocol.LegionBasicRewards115
// 完全一致（本地客户端 1424FE3F0 直读 7772B，14250FAF0 解析）：
//   - 行 0-1: @0 / @40，每行 40B（前两行走 item branch）
//   - 行 2+: @1600 + 44*(i-2)，每行 44B（额外行，field+40=0 选原生未合并物品分支）
//   - 每行内：@0 template u32，@4 value u32，@12 u16（置0），@14 metadata 21B（置0）
//   - @7760: 阶段 token（2B，低字节），高 6B 为 0（elapsedMS=0），与 N31 头呼应
//
// 前 VenusFlipGearCount 位 = 随机装备（Value=1，实机证明客户端把 value 当
// 数量显示——999999999 会显示为 "x 99..."；装备数量恒为 1）；后 3 位 = 按难度材料。
func VenusBasicClearReward(choice byte, stage int, gear []uint32) ([]byte, error) {
	if stage < 0 || stage > 3 {
		return nil, fmt.Errorf("venus reward stage %d out of range", stage)
	}
	if len(gear) > VenusFlipGearCount {
		return nil, fmt.Errorf("venus flip gear roll %d exceeds %d slots", len(gear), VenusFlipGearCount)
	}
	// 组装 8 行：前 5 装备 + 后 3 材料。
	type row struct {
		template uint32
		value    uint32
	}
	rows := make([]row, 0, VenusFlipGearCount+3)
	for _, t := range gear {
		rows = append(rows, row{template: t, value: 1}) // 装备数量恒为1
	}
	for _, m := range VenusBasicRewardMaterials(choice) {
		rows = append(rows, row{template: m.Template, value: m.Amount})
	}

	raw := make([]byte, 7772)
	for i, r := range rows {
		var off int
		if i < 2 {
			off = 40 * i // 前两行 40B 布局
		} else {
			off = 1600 + 44*(i-2) // 第三行起 44B 布局
		}
		binary.LittleEndian.PutUint32(raw[off:], r.template)
		binary.LittleEndian.PutUint32(raw[off+4:], r.value)
		// @12 u16 和 @14 metadata 21B 保持全零（材料行无需元数据；
		// 装备行客户端按 Template 从 PVF 取渲染数据，实例值已在 @4）。
	}
	// @7760 阶段 token（低 2B），高 6B 为 0（elapsedMS=0，客户端显示通关时间 00:00）。
	copy(raw[7760:], IspinsStageTokens[stage][:])
	return raw, nil
}

// VenusAdditionalClearReward builds the native fixed 2405B N2253 payload for
// the Venus terminal flip-card second row. 形状照军团家族 40B 记录步长
// {flag u8 @0, item u32 @1, count u8 @5, const 03 @9}；官服维纳斯抓包结论
// （2026-10-04 21:43-21:46 结算序列）：flag=01 为展示行。固定写第二排 5 件
// （VenusAdditionalRewardItems，各 1），全部置展示位。
func VenusAdditionalClearReward() ([]byte, error) {
	raw := make([]byte, 2405)
	for i, rec := range VenusAdditionalRewardItems() {
		off := 40 * i
		raw[off] = 1
		binary.LittleEndian.PutUint32(raw[off+1:], rec.Template)
		if rec.Amount > 255 {
			return nil, fmt.Errorf("venus additional reward count %d exceeds u8", rec.Amount)
		}
		raw[off+5] = byte(rec.Amount)
		raw[off+9] = 3
	}
	return raw, nil
}
