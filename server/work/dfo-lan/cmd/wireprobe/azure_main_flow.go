package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"dfolan/internal/game/protocol"
)

// 蔚蓝号（Azure Main）单人攻坚队。
//
// 频道/城镇/副本三个坐标都由 PVF 直读给出（configs 只声明 {ID, Name}）：
//
//	频道 type 102 → town 213（contents/2025/azuremain/town/azuremain_village.map）
//	             → [guide dungeon index] 100004131（w.channelGuideDungeon，直读填入）
//
// 与沉月湖（moon_solo_flow.go）的关系：**只参考分层，不照抄常量**。差别见
// analysis/tasks/next152-蔚蓝号AzureMain-从0实现计划.md §3：
// 蔚蓝号是**征服队**（Mode 31 / Route 37，见 protocol.ConquestPartyModeForChannel115），
// 而沉月湖那条路径只认 Mode 27；C2284 的载荷值也不同（蔚蓝号 25 / 沉月湖 24）。
const (
	azureMainChannelType = 102
	azureMainTown        = 213
	// azureMainSoloPartyID：单机单人队的名册 partyID。
	// 与沉月湖单人路径取同一个值（那条路径实机验证过客户端能渲染出队伍），
	// 官服实测用的是 266（服务端分配），本地不需要真实分配器。
	azureMainSoloPartyID = 1
)

// azureMainPhasePlaying 是 N2621 阶段字段（[0:4]）的「进行中」值。
// 官服 14 帧实测：进本与整个战斗期恒为 2，只有结算段才 3/4/5。
const azureMainPhasePlaying uint32 = 2

// azureMainCountdownSeconds 是 N2621 倒计时（[9:11] u16，秒）的起点。
// 官服进本首帧实测 0x0e10 = 3600，随后单调递减。
const azureMainCountdownSeconds = 3600

// azureMainInfoBody 构造 NOTI2621 AZURE_MAIN_INFO（128B）。
//
// 布局由官服实机抓包 14 帧差分得出
// （E:/迅雷下载/20261003-214424/decoded/F16-s2c.txt
//
//	#416/#421/#488/#510/#525/#529/#556/#576/#580/#644/#648/#663/#684/#688）：
//
//	[0:4]     u32 阶段：2=进行中，3/4/5=结算
//	[4:8]     u32 0（结算段为 1）
//	[9:11]    u16 倒计时秒（进本 3600 起递减）
//	[22:26]   u32 海怪 id（进本 0，出现后 0b1dee07 / b85bc10f）
//	[32:36]   u32 = 8（官服恒定）
//	[36:68]   最多 4 个已清空房间的 (x,y) u32 对，按清空先后
//	[116:121] 5B 非单调值（官服实测 2.26e11~2.80e11，不是 Unix 秒）
//
// [22:26] 的置位时机未闭环 ⇒ 留 0（官服进本那一帧本来也是 0）；
// [36:68] 是客户端判定「本房间已打完、门可以走」的依据：官服 #488 起逐房追加，
// 私服此前一律发 0 ⇒ 0 怪的起始房能走、有怪的房间永远走不掉（实机 2026-10-04）。
// [116:121] 属于本客户端「非必需」的一类字段 —— 私服在 N29 尾部与 N38 尾部同样
// 省略同形状的 5B 值，其它副本照常推进，故留 0。
func azureMainInfoBody(phase uint32, elapsed time.Duration, cleared [][2]byte, kraken uint32, revivesLeft uint32) []byte {
	p := make([]byte, 128)
	binary.LittleEndian.PutUint32(p[0:], phase)
	left := int64(azureMainCountdownSeconds) - int64(elapsed/time.Second)
	if left < 0 {
		left = 0
	}
	if left > 0xffff {
		left = 0xffff
	}
	binary.LittleEndian.PutUint16(p[9:], uint16(left))
	// [32:36]：本局剩余复活次数（上限 8）。用币复活后要递减，客户端按它刷新计数。
	binary.LittleEndian.PutUint32(p[32:], revivesLeft)
	// [22:26]：客户端最近上报的海怪/弱点 id（CMD2274 回显）。
	binary.LittleEndian.PutUint32(p[22:], kraken)
	// [36:68]：最多 4 个「已清空房间」的 (x,y)，各按 u32 小端写，按清空先后排列。
	// 官服 #488=(1,0)、#529=+(1,1)、#580=+(1,3)、#648=+(2,2)；0 怪的房间不进来。
	for i, room := range cleared {
		if i >= 4 {
			break
		}
		binary.LittleEndian.PutUint32(p[36+8*i:], uint32(room[0]))
		binary.LittleEndian.PutUint32(p[40+8*i:], uint32(room[1]))
	}
	return p
}

// azureMainInfo 把 N2621 打包成出站包（kind 0 = NOTI）。
func (w *worldSession) azureMainInfo(now time.Time) outboundPacket {
	var elapsed time.Duration
	if !w.azure.runStarted.IsZero() {
		elapsed = now.Sub(w.azure.runStarted)
	}
	return outboundPacket{"azure_main_info", 0, 2621, azureMainInfoBody(azureMainPhasePlaying, elapsed, w.azure.cleared, w.azure.kraken, w.azure.revivesLeft)}
}

// azureRoomClearedInfo 在「本房间刚打空」时回一帧 N2621。
// 官服正是在 #488（房间 (1,0) 最后一只怪被确认打死后、下一张 N29 之前）发的这一帧；
// 本客户端缺了它就不再走门（实机 2026-10-04）。同一张房间图只发一次（换图后
// Room.Map 变化，自然重新计）。
func (w *worldSession) azureRoomClearedInfo(now time.Time) (outboundPacket, bool) {
	if w.channelType != azureMainChannelType || w.activeDungeon == nil || !w.activeDungeon.RoomCleared() {
		return outboundPacket{}, false
	}
	if w.azure.infoRoom == w.activeDungeon.Room.Map {
		return outboundPacket{}, false
	}
	w.azure.infoRoom = w.activeDungeon.Room.Map
	w.azure.cleared = append(w.azure.cleared, [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y})
	return w.azureMainInfo(now), true
}

// azureMainState 是蔚蓝号会话状态。M2 只需要"队伍已创建"这一个事实。
type azureMainState struct {
	created bool
	// runStarted 是本趟副本的起始时间，供 N2621 的 1 小时倒计时（[9:11]）。
	runStarted time.Time
	// infoRoom 记录「已为哪张房间图发过房间清空版的 N2621」，同图只发一次。
	infoRoom uint32
	// cleared 是已清空房间的坐标，按清空先后排列 —— 写进 N2621 的 [36:68]。
	// 只有**有怪并被打空**的房间才进来（官服里 0 怪的起始房与 (1,2) 都不在内）。
	cleared [][2]byte
	// kraken 是客户端最近一次用 CMD2274 上报的海怪/弱点 id，写进 N2621 的 [22:26]。
	// 官服就是在回显客户端报上来的那个值（#426 报 0b1dee07 → #525 N2621 里就是 0b1dee07）。
	kraken uint32
	// revivesLeft 是本局**剩余复活次数**，写进 N2621 的 [32:36]。
	// 官服那个字节一直是 8（= 上限，本局没人用币）；私服此前把它写死成 8，
	// 所以用币复活之后客户端看到的次数不会减少（业主实机 2026-10-04）。
	revivesLeft uint32
}

// azureMainReviveLimit 是征讨副本的复活次数上限（官服 N2621 [32:36] 的初值 8）。
const azureMainReviveLimit = 8

// azureContext 是名册头里的 2 字节频道上下文。
//
// 官服实测 {1, 76} = {ServerID, 频道号}（抓包 E:/迅雷下载/20261003-214424/decoded/）。
// 频道身份同步（SynchronizeIdentity）打开时用角色自带的 channel context，否则按
// {ServerID, 本频道号} 回退 —— 与 moonContext 同一口径。
func (w *worldSession) azureContext() [2]byte {
	if w.characters != nil && w.characters.ChannelContext != [2]byte{} {
		return w.characters.ChannelContext
	}
	return [2]byte{byte(w.serverID), byte(azureMainChannelType)}
}

// azureRoster 组本会话的名册载荷（单人：1 个席位，slot 0）。
func (w *worldSession) azureRoster(create protocol.PartyCreate115) ([]byte, error) {
	cm, ok := protocol.ConquestPartyModeForChannel115(azureMainChannelType)
	if !ok {
		return nil, fmt.Errorf("蔚蓝号频道 %d 不在征服频道表里", azureMainChannelType)
	}
	if create.Options.Mode != cm.Mode {
		return nil, fmt.Errorf("蔚蓝号建队 Mode=%d，期望 %d", create.Options.Mode, cm.Mode)
	}
	return protocol.ConquestRoster115{
		PartyID: azureMainSoloPartyID,
		Context: w.azureContext(),
		Seats:   [4]uint16{w.role.WireID},
		Leader:  w.role.WireID,
		// 攻坚队队名：官服帧实测它被写进名册 [18:18+n]（见证据文档 §7）。
		Title:   create.Name,
		Options: create.Options,
		Mode:    cm.Mode,
		Route:   cm.Route,
		// SecondBlock 留 nil ⇒ 按官服形态补 60 个 0（不写它，N/行/第二处 Mode 会错位）。
		// Counters 留 nil：官服尾部 22 字节与契约的计数区形状不符，未闭环前不追加。
		// RouteInTail 留 false ⇒ route 写 [8]，与官服帧一致。
	}.Encode()
}

// azureMainPhaseClearInfo / ClearReward / Done 是 N2621 `[0:4]` 的结算段阶段值。
// 官服实测：战斗期恒 2，进结算后才 3 → 4 → 5（#663/#684/#688）。
const (
	azureMainPhaseClearInfo uint32 = 3
	azureMainPhaseReward    uint32 = 4
	azureMainPhaseDone      uint32 = 5
)

// completeAzureMain 是蔚蓝号的清关链，照官服尾段帧序发：
//
//	N31 清关使能 → N1658 空包（向客户端索要清关信息）→ N2621 阶段 3。
//
// ⚠️ **不要在这里发 N33**：`NOTI33` 是 FAIL_CLEAR_DUNGEON，它自己的注释就写着
// 「triggers the native player death scene」—— 实机 2026-10-04 在清关那一刻塞了一发
// `DungeonFailClear(0)`，客户端当场播死亡镜头（业主报「BOSS 结算的时候还是直接死亡了」）。
// 官服的 #671 虽然也在 opcode 33 上，但那是 16 字节、与 N31 同键 0x5973 的另一种包，
// 不是 1 字节的失败原因；而且结算不需要它（去掉后照常出结算与翻牌）。
//
// 官服随后由客户端回 CMD1654，我们在这里先只把「可以结算了」这一步给出去 ——
// 奖励内容（N35）与翻牌仍走通用 CMD46/CMD69..CMD72 那条链。
func (w *worldSession) completeAzureMain() ([]outboundPacket, error) {
	return []outboundPacket{
		{"dungeon_clear_enabled", 0, 31, protocol.DungeonClearEnabled()},
		// [AZURE-1658-EMPTY] 与伊斯大陆同一条链：空 body 的非 nil 空 slice 语义 =
		// 真空包放行（preparePackets 曾静默丢弃空 body）。
		{"azure_main_req_clear_info", 0, 1658, []byte{}},
		w.azureMainInfoNow(azureMainPhaseClearInfo),
	}, nil
}

// azureClearInfo 回客户端 CMD1654（RES_DUNGEON_CLEAR_INFO）。
// 官服在 #672 N1658 之后收到它，接着发 N2621 阶段 4/5；N31 的收尾形态是全 0（#683）。
// azureClearInfo 回客户端 CMD1654（RES_DUNGEON_CLEAR_INFO）。
//
// ⚠️ 实机 2026-10-04：这条分支**没有被执行到**（日志里连阶段 4/5 的 N2621 都没有），
// 尽管 1654 已在 observedGameRequest 里登记。结算需要的那几帧因此**不能挂在这里** ——
// 实机 2026-10-04 最终结论：**把 1654 登进 dungeonRequest（本函数就会执行）本身就够了** ——
// 上面这三帧一发，`Back to Town (F12)` 就从不亮变成可点回城。
// 那之后额外加过的 NOTI70/N71/N72 既非必需、又导致客户端闪退，已整体撤销（见 cards.go 的警示块）。
func (w *worldSession) azureClearInfo() []outboundPacket {
	return []outboundPacket{
		w.azureMainInfoNow(azureMainPhaseReward),
		{"azure_main_clear_done", 0, 31, make([]byte, 16)},
		w.azureMainInfoNow(azureMainPhaseDone),
	}
}

// azureMainInfoNow 是 azureMainInfo 的时间便捷包装。
func (w *worldSession) azureMainInfoNow(phase uint32) outboundPacket {
	var elapsed time.Duration
	if !w.azure.runStarted.IsZero() {
		elapsed = time.Now().Sub(w.azure.runStarted)
	}
	return outboundPacket{"azure_main_info", 0, 2621, azureMainInfoBody(phase, elapsed, w.azure.cleared, w.azure.kraken, w.azure.revivesLeft)}
}

// azureHandle 是蔚蓝号频道的请求分派。**只服务 channelType 102**，
// 其它频道一律 return false（让调用方继续往下分派）。
func (w *worldSession) azureHandle(id uint16, p []byte, now time.Time, event func(map[string]any)) (bool, []outboundPacket, error) {
	if w.role.ID == 0 || w.channelType != azureMainChannelType {
		return false, nil, nil
	}
	switch id {
	case 12:
		return w.azureCreateParty(p)
	case 13:
		return w.azureLeaveParty(p)
	case 2284:
		return w.azureStartDungeon(p, now)
	case 2274:
		return w.azureKrakenData(p)
	}
	return false, nil, nil
}

// azureKrakenData 处理 CMD2274（AZURE_MAIN_KRAKEN_DATA）：BOSS 的弱点/海怪数据上报。
//
// 官服帧（F16-s2c c2s 方向）两次都是 24B，第 2 个 u32 才是那个 id：
//
//	e2797f06 0b1dee07 00000000 01000000 01000000
//	aa7b7f06 ad3ed307 00000000 01000000 03000000
//
// 而官服 N2621 的 [22:26] 随后就出现 0b1dee07（#525/#529/#556）—— **服务端在回显它**。
// 私服此前完全没接这个包（只在 request_scope 里登记过），客户端上报后拿不到回显，
// 弱点状态对不上，于是「打弱点反而把自己打死」（业主实机 2026-10-04）。
//
// 这里只做两件不改写内容的事：记下 id、把 N2621 立刻回一帧（[22:26] 带上它），
// 并回一个 ACK。id 为 0 的上报按「没有弱点」处理，不覆盖已有值。
func (w *worldSession) azureKrakenData(p []byte) (bool, []outboundPacket, error) {
	if len(p) < 8 {
		return true, nil, fmt.Errorf("蔚蓝号 2274 载荷只有 %d 字节，至少要 8", len(p))
	}
	id := uint32(p[4]) | uint32(p[5])<<8 | uint32(p[6])<<16 | uint32(p[7])<<24
	if id == 0 {
		return true, []outboundPacket{{"azure_kraken_ack", 1, 2274, []byte{1}}}, nil
	}
	w.azure.kraken = id
	return true, []outboundPacket{
		{"azure_kraken_ack", 1, 2274, []byte{1}},
		w.azureMainInfoNow(azureMainPhasePlaying),
	}, nil
}

// azureCreateParty 处理 CMD12（建攻坚队）。
//
// 判别依据是**选项 Mode**：蔚蓝号 = 31（协议表给出），沉月湖 = 27。
// Mode 不匹配时返回 false —— 这不是蔚蓝号的建队请求，不该由这里应答。
func (w *worldSession) azureCreateParty(p []byte) (bool, []outboundPacket, error) {
	if w.activeDungeon != nil {
		return true, nil, fmt.Errorf("蔚蓝号建队：副本进行中")
	}
	if w.state.Position.Town != azureMainTown {
		// 不在蔚蓝号专属城镇里 —— 交给别的分派（例如普通频道建队）。
		return false, nil, nil
	}
	create, err := protocol.DecodePartyCreate115(p)
	if err != nil {
		return true, nil, err
	}
	cm, ok := protocol.ConquestPartyModeForChannel115(azureMainChannelType)
	if !ok {
		return true, nil, fmt.Errorf("蔚蓝号频道 %d 不在征服频道表里", azureMainChannelType)
	}
	if create.Options.Mode != cm.Mode {
		return false, nil, nil
	}
	roster, err := w.azureRoster(create)
	if err != nil {
		return true, nil, err
	}
	// 与沉月湖同一帧序（2026-10-02 实测：把 N9 提前会让客户端闪退）——
	// 同一个客户端 build，M2 先沿用这个顺序；蔚蓝号自己的顺序若不同，实机会暴露。
	basic, err := w.characters.EntryBasicProbe(w.role, w.azureContext())
	if err != nil {
		return true, nil, err
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return true, nil, err
	}
	w.azure.created = true
	plan := []outboundPacket{
		{"azure_party_actor", 0, 2, basic},
		{"azure_party_details", 0, 2, detail},
		{"azure_party_created", 0, 9, roster},
	}
	// 建队正好在「开始」之前，**順手把奖励/次数快照再下发一次**，
	// 保证每次尝试进本前都有新鲜计数（否则被客户端扣完后再无刷新点）。
	if snap, snapErr := w.azureRewardSnapshot(); snapErr == nil {
		plan = append(plan, snap...)
	}
	return true, plan, nil
}

// azureLeaveParty 处理 CMD13（解散 / 离队）。名册形状取自契约 §6.3（12 字节）。
func (w *worldSession) azureLeaveParty(p []byte) (bool, []outboundPacket, error) {
	if !w.azure.created {
		return false, nil, nil
	}
	for _, b := range p {
		if b != 0 {
			return true, nil, fmt.Errorf("蔚蓝号退队：载荷非全零")
		}
	}
	cm, _ := protocol.ConquestPartyModeForChannel115(azureMainChannelType)
	body, err := protocol.ConquestPartyGone115(azureMainSoloPartyID, w.azureContext(), cm.Route)
	if err != nil {
		return true, nil, err
	}
	w.azure = azureMainState{}
	return true, []outboundPacket{{"azure_party_gone", 0, 9, body}}, nil
}

// azureMainRewardRemaining 是发给蔚蓝号的「还能领几次奖励」值。
//
// 官服实测 100004131 → **2**（F16-s2c.txt #245 的第一张表），
// 也就是面板上「Integrated Reward Count: 0/2」的分母。
const azureMainRewardRemaining byte = 2

// azureMainWeeklyEntries 是发给蔚蓝号的「本周剩几次入场」。
//
// 官服帧 F16-s2c.txt #391 里 100004131 的计数是 0，但那是**进本后**的值（#391 < N27 #410 < 首张 StartMap #417，
// 而 N27 已经是选图进入阶段）。按 N1726 同一规律（进本前 2 → 进本后 1）反推，
// 进本前至少为 1。本地单机要反复进，而客户端会逐次扣减，
// 故取 **255**（与沉月湖 `moonSoloTestRemaining` 同口径，那条路径已实机验证）。
//
// 实机 2026-10-04：照抄进本后的 0 会被客户端判为「本周入场次数已用完」
// （DSTR 100088500，sub_14165B950 的第四道门）。
//
// **而且这个计数会被客户端按次扣减**：官服进本后那帧正是 0。
// 实机 2026-10-04 02:0x：发 1 的话建队那一次能过，但随后点「开始」就报
// 「Used up all your Weekly Entry count」—— 计数已经被扣到 0，而我们只在进频道时发过一次。
// （客户端是在**发 C2284/C15 之前**就本地判定的 —— 红门也被同样拦。）
// （DSTR 100088500，sub_14165B950 的第四道门）。
const azureMainWeeklyEntries uint32 = 255

// azureMainStartMode 是 CMD2284（NPC「开始」）载荷里的「内容开始值」。
//
// 官服实测 `F16-c2s.txt #325` = `19 00 00 00` → **25**；沉月湖同字段 = 24。
// 它不是队伍模式、也不是副本 ID，只是“哪个内容的开始按钮”。
const azureMainStartMode uint32 = 25

// azureMainCompanionDungeon 是蔚蓝号在沉月湖那种“同一内容两个变体”里的另一个 id。
//
// 依据：官服 N1726 表1（`F16-s2c.txt #245`）里 100004131 与 100004134 同值（2），
// 而沉月湖那边实机可行的形态正是 100004137 + 100004134（同样差 3）。
const azureMainCompanionDungeon uint32 = 100004134

// azureSpecialRewardKeys 是官服 N1726 第一张表的**完整 key 清单**
// （逐字节取自 `F16-s2c.txt #245`，44 行）。
//
// 为什么用它：客户端在小队框 tooltip 里用一个**全局“当前内容 id”**
// （IDA: `sub_1446669A0` 的 key = `dword_14DC6664C`，初值 0xFFFFFFFF）去查这张表；
// 对不上就返回 0xFFFFFFFF → tooltip 归 0 。
// 沉月游同一客户端 build 能显示 255，说明它的 key 在表里；
// 蔚蓝号的 key 我们尚未确定 ⇒ **直接把官服整张表的 key 都发出去**，命中为止。
// （值统一取 255：官服表里本来就有多行 255，且本仓沉月游用 255 已实机可行。）
var azureSpecialRewardKeys = []uint32{100000002, 100000007, 100000176, 100000177, 100000178, 100000179, 100000213, 100000334, 100000521, 100000522, 100000523, 100000524, 100000525, 100000526, 100000527, 100002627, 100002663, 100002669, 100003219, 100003221, 100003312, 100003315, 100003318, 100003630, 100003689, 100003694, 100004131, 100004134, 100004137, 100004140, 100004141, 100004142, 100004143, 100004144, 100004145, 100004149, 100004151, 100004520, 100004521, 100004698, 100005013, 100005064, 100005065, 400001392}

// azureAccountCounts1195 / azureAccountSpecialRewards1930 是官服会话里两个
// **账号级计数** 包的逐字节原文（L0′ 重放，同本仓 N537 固定表的既有做法）：
//
//	N1195 ACCOUNT_DUNGEON_INOUT_COUNT        —— 名字就是「账号副本进/出次数」
//	N1930 ACCOUNT_DUNGEON_SPECIAL_REWARD_INFO —— 面板表头叫 「Advanced Dungeon **Special Reward** Count」
//
// 为什么要它们（IDA 已证）：窗口侧的周次数存在**按内容 id 索引的账号级红黑树**里，
// 读取者 `sub_14520E4E0(map, key)` 在 **找不到 key 时返回 0xFFFFFFFF**，而工具提示把 <0 归 0
// （= 「次数 0」）⇒ 本地被判「周次数已用完」。这两个包正是向该表写行的候选。
//
// ⚠️ 它们是**官服账号的计数快照**（key 是该系统自己的账号级内容 id）；
// 本地单机先直接重放以验证“这两个包里有没有蔚蓝号的计数”。
var azureAccountCounts1195 = []byte{0x23, 0x00, 0x00, 0x00, 0x9a, 0x01, 0x00, 0x00, 0x01, 0x9b, 0x01, 0x00, 0x00, 0x01, 0x9c, 0x01, 0x00, 0x00, 0x01, 0x3e, 0x13, 0x00, 0x00, 0x03, 0x3f, 0x13, 0x00, 0x00, 0x03, 0x40, 0x13, 0x00, 0x00, 0x03, 0x41, 0x13, 0x00, 0x00, 0x03, 0x42, 0x13, 0x00, 0x00, 0x03, 0x43, 0x13, 0x00, 0x00, 0x03, 0x44, 0x13, 0x00, 0x00, 0x03, 0x45, 0x13, 0x00, 0x00, 0x01, 0x46, 0x13, 0x00, 0x00, 0x01, 0x47, 0x13, 0x00, 0x00, 0x01, 0x80, 0xb5, 0x01, 0x00, 0x03, 0x81, 0xb5, 0x01, 0x00, 0x03, 0x82, 0xb5, 0x01, 0x00, 0x03, 0xae, 0x17, 0x00, 0x00, 0x03, 0x0f, 0x1c, 0x00, 0x00, 0x03, 0x10, 0x1c, 0x00, 0x00, 0x01, 0x8d, 0xb5, 0x01, 0x00, 0x05, 0x21, 0x65, 0xcd, 0x1d, 0x01, 0x22, 0x65, 0xcd, 0x1d, 0x01, 0x23, 0x65, 0xcd, 0x1d, 0x01, 0x24, 0x65, 0xcd, 0x1d, 0x01, 0x25, 0x65, 0xcd, 0x1d, 0x01, 0x26, 0x65, 0xcd, 0x1d, 0x01, 0x27, 0x65, 0xcd, 0x1d, 0x01, 0x28, 0x65, 0xcd, 0x1d, 0x01, 0x29, 0x65, 0xcd, 0x1d, 0x01, 0x2a, 0x65, 0xcd, 0x1d, 0x01, 0x2b, 0x65, 0xcd, 0x1d, 0x01, 0x2c, 0x65, 0xcd, 0x1d, 0x01, 0x2d, 0x65, 0xcd, 0x1d, 0x01, 0x2e, 0x65, 0xcd, 0x1d, 0x02, 0x30, 0x65, 0xcd, 0x1d, 0x01, 0x23, 0x5a, 0x14, 0x90, 0x35}

var azureAccountSpecialRewards1930 = []byte{0x09, 0x50, 0x8c, 0xd7, 0x17, 0x01, 0x51, 0x8c, 0xd7, 0x17, 0x01, 0x52, 0x8c, 0xd7, 0x17, 0x01, 0x53, 0x8c, 0xd7, 0x17, 0x01, 0x58, 0x8c, 0xd7, 0x17, 0x02, 0x17, 0x65, 0xcd, 0x1d, 0x02, 0x18, 0x65, 0xcd, 0x1d, 0x02, 0x19, 0x65, 0xcd, 0x1d, 0x02, 0x1a, 0x65, 0xcd, 0x1d, 0x02, 0xb3, 0xbe, 0x0e, 0x92, 0x35, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

// azureRewardSnapshot 组蔚蓝号的 N1726 + N537。
//
// 为什么要它（2026-10-04 实机）：在蔚蓝号专属城镇里点「创建攻坚队」，
// 客户端弹「You've received all the available rewards. Can't enter anymore.」
// —— N1726 的第一张表就是「可再领 N 次奖励」的来源（本仓 moon_solo_flow.go 同款注释：
// 发 255 时右下角 System 写 'You can receive Advanced Dungeon rewards 255 more times.'），
// 而本仓原先**只在月湖路径**下发它 ⇒ 102 频道上这张表是空的，
// 客户端就判定「奖励已领完」。
//
// 副本 id 取**直读**来的 w.channelGuideDungeon，不写死。
func (w *worldSession) azureRewardSnapshot() ([]outboundPacket, error) {
	d := w.channelGuideDungeon
	if d == 0 {
		return nil, fmt.Errorf("蔚蓝号频道没有 [guide dungeon index]，无法组奖励快照")
	}
	// 照沉月湖（同一客户端 build，实机可行）的形态拄齐：
	//   表1 = **2 行**（guide dungeon + 它的同族 id）—— 沉月湖是 100004137+100004134；
	//   表2 = **1 行**（沉月湖也是 1 行；我们原来发空，实机上 HUD 那格就是脏值）。
	// 同族 id 取 100004134：官服 `#245` 的表1 里 100004131 与 100004134 同值（2）同土，
	// 正是沉月湖 100004137/100004134 那种“同一内容的两个变体”对。
	first := make([]protocol.SpecialRewardRemaining115, 0, len(azureSpecialRewardKeys)+1)
	first = append(first, protocol.SpecialRewardRemaining115{Dungeon: d, Remaining: azureMainRewardRemaining})
	for _, k := range azureSpecialRewardKeys {
		if k == d {
			continue
		}
		first = append(first, protocol.SpecialRewardRemaining115{Dungeon: k, Remaining: azureMainRewardRemaining})
	}
	second := append([]protocol.SpecialRewardRemaining115{}, first...)
	body, err := protocol.DungeonSpecialRewardInfo115(first, second)
	if err != nil {
		return nil, err
	}
	// N537：每个 key 各一条（u32 副本 + 两个相同字节 = 本周剩几次入场）。
	// 同样是为了命中客户端认定的那个 key（同上面的理由）。
	entries := make([][]byte, 0, len(azureSpecialRewardKeys)+1)
	for _, k := range append([]uint32{d}, azureSpecialRewardKeys...) {
		one, oneErr := protocol.DungeonTestRemaining(k, azureMainWeeklyEntries)
		if oneErr != nil {
			continue
		}
		entries = append(entries, one)
	}
	plan := []outboundPacket{
		{"azure_main_rewards", 0, 1726, body},
	}
	for i, one := range entries {
		plan = append(plan, outboundPacket{"azure_main_entries", 0, 537, one})
		_ = i
	}
	plan = append(plan,
		outboundPacket{"azure_account_dungeon_inout", 0, 1195, azureAccountCounts1195},
		outboundPacket{"azure_account_special_rewards", 0, 1930, azureAccountSpecialRewards1930},
	)
	return plan, nil
}

// azureStartDungeon 处理 CMD2284（在专属城镇里点「开始」）。
//
// 设计（与月湖只共享分层，不照抄常量）：
//
//	分流：载荷 u32 == 25 才是蔚蓝号（月湖 = 24），否则 return false 交给别家。
//	机械：复用**红门（C15）那条通用 SemiRaid 进场路径**（`prepareDungeonEntry` +
//	      `dungeonEntryPlanImpl`）—— 它已经走的是 `w.channelGuideDungeon`（蔚蓝号 = 100004131），
//	      并且 2026-10-03 为止黑屏修过（`client_dispatch_world.go` 的红门分支）。
//	      两边同样在 N28 之前插 `N3(角色状态→副本态) + N27(选图上下文)`。
//	回执：ACK 2284（kind=1，单字节 `{1}`）—— 与月湖 `SemiRaidStartReply115(true)` 同形。
//	      官服帧里的 2284 回执是 16 字节（`F16-s2c.txt #395`，首字节 01），多出的 15 字节
//	      语义未知；先用单字节形态，实机不行再补。
//
// 未发 N2621（AZURE_MAIN_INFO 128B）：它的 `[0:4]`=阶段、`[32:68]`=9 个逐房间 u32、
// `[116:124]`=一个**语义未知且不单调**的 u64（官服实测 2.3e11~2.8e11，不是 Unix 秒）。
// 不编未知值，先看实机是否需要它。
func (w *worldSession) azureStartDungeon(p []byte, now time.Time) (bool, []outboundPacket, error) {
	refuse := func(reason string) (bool, []outboundPacket, error) {
		// 失败必须回拒绝包，否则客户端会一直等待（同源码红门分支的做法）。
		log.Printf("蔚蓝号 2284 拒绝：%s", reason)
		return true, []outboundPacket{{"azure_start_refused", 1, 2284, protocol.Refusal(4)}}, nil
	}
	mode, err := protocol.DecodeSemiRaidStart115(p)
	if err != nil {
		return false, nil, nil
	}
	if mode != azureMainStartMode {
		// 不是蔚蓝号的开始按钮（例如月湖 24）。
		return false, nil, nil
	}
	if w.activeDungeon != nil {
		return refuse("副本进行中")
	}
	if w.state.Position.Town != azureMainTown {
		return refuse(fmt.Sprintf("不在专属城镇 %d，当前 %d", azureMainTown, w.state.Position.Town))
	}
	if w.channelGuideDungeon == 0 {
		return refuse("频道没有 [guide dungeon index]")
	}
	sel := protocol.DungeonSelection{ID: w.channelGuideDungeon, Difficulty: 0, Party: 65535}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sess, _, err := w.prepareDungeonEntry(sel)
	if err != nil {
		return refuse("prepareDungeonEntry: " + err.Error())
	}
	frames, err := w.dungeonEntryPlanImpl(ctx, "azure_start_ack", 2284, sel, sess, &w.characters.ChannelContext)
	if err != nil {
		return refuse("dungeonEntryPlanImpl: " + err.Error())
	}
	// 与红门分支同序：N28 之前插 N3 与 N27。
	// N27 会切换客户端的场景上下文，必须在 N28 之前（同源码红门备注）。
	actorState, stateErr := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	plan := make([]outboundPacket, 0, len(frames)+2)
	inserted := false
	for _, pkt := range frames {
		if pkt.ID == 28 && !inserted {
			if stateErr == nil && actorState != nil {
				plan = append(plan, outboundPacket{"azure_actor_state_dungeon", 0, 3, actorState})
			}
			plan = append(plan, outboundPacket{"azure_dungeon_selection", 0, 27, protocol.EnterDungeonSelection()})
			inserted = true
		}
		plan = append(plan, pkt)
	}
	// 副本会话落地（照红门分支）。
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.resetCards()
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.leaveScene()
	w.activeDungeon = sess
	return true, plan, nil
}
