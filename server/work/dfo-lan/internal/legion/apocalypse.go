package legion

import "time"

// ApocalypseEntryNonce 是末世录 NOTI2254（LEGION_ENTRY_CHARAC_INFO）尾部的
// 5 字节 nonce。抓包 20261008-105227 的两帧 N2254（进频道、进攻坚房间各一）
// 都是 272 字节，且**逐字节相同**：内容号 107 + 七个槽（0x65..0x6b）+ 固定的
// 状态尾部，末尾就是 af 81 3d 17 3f。
//
// 本仓库 `ispinsLoginEntryCharacterInfoTemplate` 与这两帧**逐字节相等**
// （只差这个 nonce，模板里的常量正好是 af813d173f），所以末世录直接复用
// 伊斯的构造器，不新增模板。
var ApocalypseEntryNonce = [5]byte{0xaf, 0x81, 0x3d, 0x17, 0x3f}

// ApocalypseEntryCharacterInfo 构造末世录的 NOTI2254。
//
// 它描述「本军团内容里每个角色的入场槽位」（七个 0x65..0x6b），客户端在
// 进频道与进攻坚房间时各收一次；缺失时角色选择/队伍槽位界面没有数据源。
func ApocalypseEntryCharacterInfo() ([]byte, error) {
	return IspinsEntryCharacterInfo(true, [4]bool{}, ApocalypseEntryNonce)
}

// ApocalypseStageDungeons are the phase dungeons of the Apocalypse legion, in
// legionsystem.cos order (contents/system/legionsystem/legionsystem.cos,
// [Apocalypse] dungeon_info_data — the same rows the channel directory uses for
// the boss-name display, see catalog.LegionContents[Apocalypse].Dungeons).
//
// The first id is the navigation room (navigationroom_watingroom.map): the
// client enters it with CMD2062 DUNGEON_DIRECT_MOVE while the run is still at
// stage0, and the five ids after it are the combat phases in clear order.
//
// Evidence: x64 capture 20261008-105227 (both difficulty clears) and the
// apocalypse events of the US-Local clearing session
// roles_..._20261008_105231_329884_next37. In that session the navigation entry
// is dungeon=100005112 map=100016816 and the combat phases resolve to
// 100004995 / 100005057 / 100004918 / 100005111 / 100004994 in stage order.
var ApocalypseStageDungeons = [6]uint32{100005112, 100004995, 100005057, 100004918, 100005111, 100004994}

// ApocalypseNavigationDungeon is the run's waiting-room dungeon (stage0), the
// one the client loads when it asks to start the attack after the operation and
// difficulty were confirmed.
const ApocalypseNavigationDungeon uint32 = 100005112

// IsApocalypseStageDungeon reports whether the id is one of the Apocalypse
// phase dungeons (navigation room included).
func IsApocalypseStageDungeon(id uint32) bool {
	for _, v := range ApocalypseStageDungeons {
		if v == id {
			return true
		}
	}
	return false
}

// ApocalypseStageOfDungeon returns the stage index (0 = navigation room) of an
// Apocalypse dungeon id, or -1 when the id is not one of them.
func ApocalypseStageOfDungeon(id uint32) int {
	for i, v := range ApocalypseStageDungeons {
		if v == id {
			return i
		}
	}
	return -1
}

// ApocalypseCombatStageDungeons are the five combat phases in clear order; the
// navigation room is deliberately excluded because its "clear" is the
// navigation gate (portal) rather than a boss death.
func ApocalypseCombatStageDungeons() []uint32 {
	return ApocalypseStageDungeons[1:]
}

// ApocalypseRunState is the per-connection Apocalypse run tracker. It is the
// server's own bookkeeping for the flow the client drives through CMD2043 →
// CMD2354 → CMD2062 → CMD2355 → CMD2046; every field maps onto one byte range
// of NOTI2895 LEGION_INFO (see apocalypse_info.go).
//
// The tracker is intentionally not persisted: a legion run is session state,
// and the repo's solo-party decision means one connection owns one run.
type ApocalypseRunState struct {
	// Choice is the difficulty byte of the CMD2354 confirmation (255 = none
	// selected). The client looks the CTP operation up as choice+1.
	Choice byte
	// Action is the last CMD2354 action (1 = open/refresh, 2 = confirm).
	Action uint32
	// Entered records that CMD2045 was answered for this run.
	Entered bool
	// Stage is the stage the client has actually loaded: 0 until the client asks
	// for the waiting room with CMD2062, 1 after that, +1 per later stage.
	Stage int
	// PhaseCleared is how many stages the run has reached (1 right after CMD2045
	// confirms the operation, even though no stage is loaded yet).
	PhaseCleared int
	// Cleared is the 1-based number of the last stage whose monsters were all
	// killed (0 = none). It exists because the clear frame's mark array covers
	// the stage that was just cleared, while Stage still points at the stage
	// that is loaded — the two differ by one at exactly that moment
	// (参考抓包：第 4 关清完那一帧是 stage=5 marks=[00 01 02 03 04 ff]).
	Cleared int
	// Role is the CMD2355 role value the client announced (0 = none yet).
	Role uint32
	// Seat is the party seat the role was announced for.
	Seat int
	// RoleSet records that at least one role was assigned, which is what turns
	// the role bitmask in LEGION_INFO on.
	RoleSet bool
	// Ended records a run whose reward screen was closed (CMD2046).
	Ended bool
	// ID identifies this run for idempotent reward commits. It is regenerated on
	// every CMD2043 so a second run can never reuse the first run's receipt key.
	ID string
	// Rewarded records that the terminal reward chain was already delivered for
	// ID, so a duplicated clear signal cannot pay twice.
	Rewarded bool
	// Suspended records a run that the client left with the **unfinished** CMD72
	// retreat (State1/Option2).
	//
	// 规格（D:\115US-001\moshilu 的 0072-结算离场.md「已推翻」段，G0452）：
	// 「末世录119的未完成C72撤退不再等价于cancel；保留同一在线作战，回城重建后
	// 恢复N2895 State2和原阶段，C2045继续当前Boss。主动C2044放弃才走取消」。
	// 2045 规格进一步要求「阶段须等于保存值，全员回城写入完成才重建当前未完成
	// Boss，保留RunID、难度与已清路线」。
	//
	// 因此撤退只置这一位并回城：Ticket(ID)/Choice/Stage/Cleared 全部保留，
	// 客户端回城后服务端恢复 N2895 State2 与原阶段，玩家再点开始（NPC 继续门）
	// 时用 CMD2045 带着**保存的阶段**续上，而不是从第 1 关重开。
	Suspended bool
	// ResumeStage 是本次 CMD2045 续关要重建的副本表下标（G0452「重建当前未完成
	// Boss」）。0 = 攻坚房间；= Stage 时是「当前该打的那一关」。
	// 它在 apocalypseEnter 里按 run.Stage 定下，由 apocalypseStageProjection 消费。
	ResumeStage int
	// StageClock 是各阶段倒计时（NOTI1474）的开始时刻，按副本表下标索引。
	//
	// 1474 规格（D:\115US-001\moshilu\10-军团末世录专有\1474-DUNGEONTIMEOUTTIME.md）：
	// 「真实共享加载和资源释放完成后，紧随本人的 N30 发送。后续同阶段换房使用
	//   原开始时间」「不能以当前发送时间覆盖阶段最初开始时间」。
	// 所以只在**该阶段第一次真实加载**时冻结，重进同一关（撤退/超时）复用原值，
	// 否则每次发包都会把倒计时续满，超时判死永远不成立。
	StageClock [6]time.Time
	// FinalDone 置位后表示已发出终局 State3（通关视频触发帧），
	// CMD191 的剧情暂停/恢复由末世录专属应答处理（见 apocalypseStoryPause）。
	FinalDone bool
	// StoryFinished 置位后表示通关视频已播完（客户端发过 CMD191 state=1），
	// leave 态 N2895 只发一次。
	StoryFinished bool
	// RunStartedAt 是本局**第一次真实进图**的时刻（CMD2045 载入第一间房时冻结）。
	// N2252 的 @7760 要写「通关耗时毫秒」= 结算时刻 − 本字段（规格
	// 2252-LEGIONBASICREWARD.md：@7760 8B 耗时毫秒，14250FAF0 读取后转换秒数）。
	RunStartedAt time.Time
	// WindowDeadline 是难度选择窗的截止时刻（CMD2354 ACK 里下发给客户端的
	// deadline）。军团选择窗有限时，客户端按它显示倒计时并在到点自动关窗。
	WindowDeadline time.Time
	// RevivesUsedThisStage 是**本关**已经用掉的复活币次数。
	//
	// 上限来自所选作战的 `[allow coin]`（PVF `apocalypse.ctp` 的
	// `[operation data set]`；导入见 internal/catalog/apocalypse_import.go 的
	// colAllowCoin，二进 RunPlan 见 internal/legion/phase.go 的 AllowCoin）。
	// 实测只有作战①（难度1）带该列，值 `-1, 8`；作战②/③/④ 没有该列。
	//
	// 业主 2026-10-08 定调语义：**最后一个数 = 每关上限；无该列 = 禁止复活**。
	// 于是难度1 = 每关 8 个，难度2/3 = 一个都不能用。
	//
	// 进关（攻坚房 / 之后每一关）时归零，所以是「每关」而非「每局」——
	// 归零点在 apocalypseStageEntry（CMD2045 / CMD2062 直进 / 服务端推进三条
	// 路径都经过它）。Reset 会把本字段一并清零。
	RevivesUsedThisStage int
}

// ContinueStage 是撤退挂起后「继续作战」时**客户端会带来的 CMD2045 @17 值**。
//
// ★ 刻度只此一套，内部值与发布值**完全一致**：
//
//	run.Stage == NOTI2895 @13 == CMD2045 @17 == 「已进入过的房间数」
//	  = 0 尚未进图（2045 那一帧）
//	  = 1 已进攻坚房（副本表下标 0）
//	  = 2 已进第 1 关（下标 1）
//	  = 3 已进第 2 关（下标 2）
//
// 要载入的**副本表下标 = ContinueStage() - 1**（攻坚房=0、第1关=1、第2关=2）。
//
// 参考抓包锚点（105227）：2045 那帧 @13=0、攻坚房载入后 @13=1、
// 第 1 关载入后 @13=2、第 2 关载入后 @13=3 —— 与上面的对照完全吻合。
//
// ⚠️ 这个函数前后错过两次，根因都是把「刻度」与「下标」混用：
//
//	16:33 返回 Stage 却按下标去载入 → 每撤退重进往前跳一间（第1关→第2关）
//	16:43 返回 Stage-1 而客户端发的就是 Stage → 直接拒绝
//	      （`resume stage 1 does not match the saved stage 0`）
//
// 现在「内部即发布值」，换算只在载入处做一次（-1），并由
// TestApocalypseResumeStageScale 把三个锚点全部钉住。
func (r *ApocalypseRunState) ContinueStage() int {
	if r.Stage < 0 {
		return 0
	}
	if r.Stage > len(ApocalypseStageDungeons) {
		return len(ApocalypseStageDungeons)
	}
	return r.Stage
}

// MarkCount reports the value ApocalypseMarksFor consumes for the current run:
// how many leading mark slots are filled (see its comment for the byte layout).
//
//	not entered                → 0（全 0xff）
//	已进入副本流程              → max(2, Cleared+2)
//
// Cleared 是「刚清掉的那一关」的 1 基号（0 表示还没清过任何一关，攻坚房间
// 不算关）。marks 要覆盖到刚清掉的那一关，所以用 Cleared+2 —— slot0/1 成对
// 出现，之后每清一关多一格。载入某一关本身不改变已经清掉的关数，因此 marks
// 不会因为「载入下一关」而抖动，这正是参考抓包的形状。
//
// 参考锚点：CMD2045 之后 = 2（[00 01 ff…]）；第 1 关清完 = 3（[00 01 02 ff…]）；
// 第 2 关载入/清完 = 4（[00 01 02 03 ff…]）；第 5 关清完 = 6（[00 01 02 03 04 05]）。
func (r *ApocalypseRunState) MarkCount() int {
	if !r.Entered {
		return 0
	}
	filled := r.Cleared + 2
	if filled < 2 {
		filled = 2
	}
	if filled > 6 {
		filled = 6
	}
	return filled
}

// ApocalypseEndpoint 是某难度在**终局奖励门**上要投影到的固定终点
// （规格 G0454：「将线级 Stage 投影到该难度**固定终点 3/5**」）。
//
// 规格 2252-LEGIONBASICREWARD.md 的实现段给出确切值：
//
//	「末世录模式消费者 140696EA0 先以虚表+80→140696D50 检查模式当前阶段是否
//	  等于难度终点（**第一档 3，第二档 5**）」
//
// choice 是客户端的难度键（0 = 第一档、1 = 第二档）。越界时回退到「最后一关的
// 下标」，不编造终点。
func ApocalypseEndpoint(choice byte) int {
	switch choice {
	case 0:
		return 3
	case 1:
		return 5
	}
	return len(ApocalypseStageDungeons) - 1
}

// ApocalypseSelectionSeconds 是军团作战选择窗的时长（秒），与维纳斯
// VenusSelectionSeconds 同值同义：CMD2354/CMD2290 的 ACK 里下发 deadline，
// 客户端按它显示倒计时并在到点自动关窗（业主 2026-10-08 要求「选择难度界面
// 倒计时」，并指定「最好按维纳斯的来」）。
//
// ⚠️ 与源表的差异**是有意保留的**（业主 2026-10-09 明确拍板）：
// 真源 `legionsystem.cos` 的 `operation select limit time` 对末世录是 **40 秒**
// （已解析进 `catalog.LegionContent.SelectLimit`，见 legion_contents.go 与
// entry_plan.go 的 SelectionTimeout）。本实现按业主指定沿用维纳斯口径 15 秒。
// **不要**把它当成「PVF 读得到却写死」的缺陷去"修"成 40 —— 改了会改变业主
// 已实机验收过的倒计时表现（15→0 到点自动关窗）。
const ApocalypseSelectionSeconds uint32 = 15

// NewApocalypseRunState returns a run with no difficulty selected. The zero
// value is not usable because choice 0 means "difficulty 1" to the client.
func NewApocalypseRunState() *ApocalypseRunState {
	r := &ApocalypseRunState{}
	r.Reset()
	return r
}

// Reset drops the run back to its pre-entry state, both for a fresh CMD2043 on
// the same connection and for an abandoned run.
//
// A new ID is minted here rather than by the caller: the id is the basis of the
// terminal reward's idempotency key, so "there is a run" and "the run has an id"
// must never diverge (a run that Reset forgot to re-id would reuse the previous
// run's receipt key and silently swallow the second clear's reward).
func (r *ApocalypseRunState) Reset() {
	*r = ApocalypseRunState{Choice: 0xff}
	r.ID = NewApocalypseRunID()
}
