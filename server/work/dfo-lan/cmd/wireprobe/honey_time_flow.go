package main

// 活动 313 HoneyTime（每日在线时长奖励）的**服务端进度**：读回 / 累计 / 推送 / 落库。
//
// 内容来自 PVF `live/event/kor/2026/0326_honeytime/honeytime.evt`
// （`[reward data]` = `序号 物品 数量 0 毫秒阈值` ⇒ 在线 10/20/30/60 分钟各一档；奖励走邮件）。
//
// 协议（都已取证）：
//
//	状态   ch0 **NOTI1294**（81 字节，形状见 internal/game/protocol/honey_time115.go）
//	领取   ch1 **CMD680**（通用活动领奖）：实机样本 `39010000 00000000` = `u32 313 + u32 行号 0`
//
// ## 为什么要服务端记进度（业主 2026-10-10 实机结论）
//
// 客户端的在线累计是**会话内、本地**的 —— 切换角色后阶段回到 1。所以像签到那样按**角色级**
// 持久化：登录时读回「今天的累计秒数 + 属于哪一天」推给客户端续上；
// 跨天（**06:00 UTC**，与签到同一条客户端规则）从头开始。见 internal/honeytime。
//
// ## 只写一个字段（这一点很关键）
//
// `sub_143CC72A0(obj, 轨, u32)` 第一句是 `v8 = 1000 * a3` ⇒ **那个 u32 是"秒"**，
// 客户端自己乘 1000、再用 .evt 每行的阈值算"走到第几档"。所以只要往**某一条轨**那个 u32
// 写累计秒数即可。哪条轨是"每日"还没定（4 条轨语义待实机逐格确认），所以用环境变量选轨、
// **不必重编**：
//
//	set DFO_HONEYTIME_SLOT=0   （0..3，默认 0）
//
// 其余字节一律留 0（中性态）—— 逐格语义未证前不碰，免得把没证的猜测写成契约。
import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/honeytime"
	"encoding/json"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const honeyTimeNotiID = 1294

// honeyTimePeriodWarnOnce 让"期间轨未接通"这条告警每个进程只打一次。
//
// `honeyTimeStateBody` 每跳（秒级 tick）都会被调用，不节流会把日志刷满。
var honeyTimePeriodWarnOnce sync.Once

// honeyTimePeriodUnwired 报告"期间轨（面板下面那行『累计在线时长』）是否**未接通**"。
//
// 2026-10-10 收口（业主判定）：日档已完整可用，**期间轨按"已知未接通"结案**。
//
// 证据链：
//   - 写第 1 条轨不亮；写全 3 条也不亮（实机）；
//   - 判别探针（三条轨各写一个刚跨门槛、剩余 <1000 s 的值，强行压过日档的剩余）
//     ⇒ 面板顶部那个框**出现了倒计时**（说明确实读到了某条轨），
//     但**下面那行「累计时长」一点没变** ⇒ 它不是由 1294 的 1/2/3 轨驱动的。
//   - 旁证：`1657 ENUM_CMDPACKET_HONEY_TIME_PERIOD_DONE` 是 **c→s** ⇒ 期间轨进度很
//     可能是客户端自己算、用 1657 上报，服务端该做的是"收到上报→发邮件"，而不是推秒数。
//
// ⇒ 缺省**一律不写期间轨**（`off`）：宁可少显示，也不把探针用的假秒数当内容发给玩家。
// 要继续做这条线，走 `DFO_HONEYTIME_PERIOD_SLOT=0..3|all|probe` 手动开，
// 或按 1657 路线重开（见 next194 §22/§25）。
func honeyTimePeriodUnwired() bool {
	raw := strings.TrimSpace(os.Getenv("DFO_HONEYTIME_PERIOD_SLOT"))
	if !strings.EqualFold(raw, "off") {
		// 空串 = 缺省 `period`（写真实秒数，见 next196）；显式选轨/探针也是"有写"。
		return false
	}
	honeyTimePeriodWarnOnce.Do(func() {
		fmt.Printf("honeytime: DFO_HONEYTIME_PERIOD_SLOT=off —— 本进程不写期间轨。"+
			"缺省已是 period（写真实秒数到 @26 与 @60，见 next196）。\n")
	})
	return true
}

// honeyTimeMailSender 是系统邮件的署名。脚本里没有任何文案键（`[reward postal type] 1`
// 只说"走邮件"），所以署名与正文都由服务端给 —— 属于**运营文案**，不是玩法规则。
const honeyTimeMailSender = "Honey Time"

// honeyTimeTrackState 是"这一帧里某条轨要写的状态"。
type honeyTimeTrackState struct {
	slot    int
	seconds uint32
	mask    uint32
	count   int
	// limit 是该轨"轮数"那一格在 1294 里的偏移（0 = 该轨在流里没有这一格）；
	// loop 是写进去的值。见 honeyTimeSlotLimitOffsets 的注释（这是期间轨能不能领的关键）。
	limit int
	loop  byte
}

// honeyTimePeriodTracks 读 `DFO_HONEYTIME_PERIOD_SLOT`，返回**期间轨**（面板下面那行
// "累计在线时长"）要写到哪几条轨、每条写什么秒数。
//
//	缺省 period  —— 写第 1 格（`period` 组 @26）与尾部 PeriodSeconds（@60），都是**真实**秒数
//	off          —— **一条都不写**
//	all          —— 除日档外每条都写**真实**期间秒数
//	数字 0..3    —— 只写那一条
//	probe        —— 判别探针：三条轨各写一个不同的假秒数，看面板读的是哪一条
//
// ★ 为什么缺省是 `off`（2026-10-10 收口，业主判定）：
//
// 探针已经跑过一轮 —— 三条轨各写一个"刚跨门槛、剩余 <1000 s"的值（强行压过日档的剩余），
// 面板顶部那个框**确实出现了倒计时**（说明读到了某条轨），但**下面那行「累计时长」纹丝不动**
// ⇒ 期间轨不是由 1294 的 1/2/3 轨驱动的，很可能走 `1657`（c→s，客户端自己算、上报）。
//
// 所以缺省**不写期间轨**：探针那些秒数是**假的**，当内容发给玩家就是伪造进度。
// 要继续取证就把 `DFO_HONEYTIME_PERIOD_SLOT` 显式设成 `0..3` / `all` / `probe`。
func honeyTimePeriodTracks(st honeytime.State) ([]honeyTimeTrackState, bool) {
	raw := strings.TrimSpace(os.Getenv("DFO_HONEYTIME_PERIOD_SLOT"))
	mk := func(slot int, seconds uint32) honeyTimeTrackState {
		return honeyTimeTrackState{slot: slot, seconds: seconds,
			mask: st.PeriodClaimed, count: st.PeriodClaimedCount(),
			limit: honeyTimeSlotLimitOffsets[slot], loop: honeyTimePeriodLoopByte()}
	}
	if raw == "" {
		// ★ 缺省从 `off` 改成 `period`（2026-10-11，吸纳上游命名）：
		// 上游把 1294 尾部 `@60` 命名为 **PeriodSeconds**（= 期间轨的累计秒数），
		// 而我们 10-10 的探针里"第 3 条轨"正是 `@60` —— 两边指向同一格。
		// 更关键的是：这一格发的是**真实存档秒数**（不是探针假值），所以可以开。
		// 同时覆盖脚本里 `[reward info]` 的 `period` 组（`@26`，第 1 格）：本季 evt 的
		// 组顺序是 daily(第 0) → period(第 1)，两处都写同一个真值，等实机看哪一处驱动面板。
		raw = "period"
	}
	if strings.EqualFold(raw, "off") {
		honeyTimePeriodUnwired()
		return nil, true // 显式 off：期间轨一格都不写
	}
	if strings.EqualFold(raw, "period") {
		// 期间轨的**真实**秒数同时写进第 1 格（`period` 组 @26）与尾部 PeriodSeconds @60。
		return []honeyTimeTrackState{mk(1, st.PeriodSeconds), mk(3, st.PeriodSeconds)}, true
	}
	if strings.EqualFold(raw, "all") {
		return []honeyTimeTrackState{mk(1, st.PeriodSeconds), mk(2, st.PeriodSeconds), mk(3, st.PeriodSeconds)}, true
	}
	if strings.EqualFold(raw, "probe") {
		// **诊断专用**（不是缺省）：三条轨各写一个**刚跨过某档门槛、离下一档还差不同时间**的值，
		// 用来判别"面板下面那行读的是哪一条轨"：
		//
		//	第 1 条 → 143900 s（离 40h 门槛差 100 s = 1m40s）
		//	第 2 条 → 215800 s（离 60h 门槛差 200 s = 3m20s）
		//	第 3 条 → 287700 s（离 80h 门槛差 300 s = 5m00s）
		//
		// ★ 三个值都**必须远小于日档当时的剩余**：顶部那个框显示的是"剩余最短的那条轨"
		// （UI 填表里 `v88 = sub_143CC5DB0(档)` 取正的最小值）。第一版探针用了 1000/2000/3000 s，
		// 而日档当时剩余 1783 s ⇒ 2000/3000 那两个**根本抢不过日档**，只有第 1 条能被判别出来
		// （2026-10-10 实机：框里仍是日档的 29m43s）。现在三个都 < 1000 s，谁是真轨谁一定赢。
		//
		// 结果（2026-10-10 实机）：框里出现了倒计时，但**下面那行没有变化** ⇒ 期间轨不是
		// 这三条中的任何一条。详见 `honeyTimePeriodUnwired`。
		return []honeyTimeTrackState{
			{slot: 1, seconds: 143900},
			{slot: 2, seconds: 215800},
			{slot: 3, seconds: 287700},
		}, true
	}
	slot, err := strconv.Atoi(raw)
	if err != nil || slot < 0 || slot > 3 {
		return nil, false
	}
	return []honeyTimeTrackState{mk(slot, st.PeriodSeconds)}, true
}

// honeyTimeSlotOffsets 是 4 条轨道那个「累计秒数」u32 在 1294 载荷里的偏移
// （逐字节布局见 internal/game/protocol/honey_time115.go 的头注释）。
var honeyTimeSlotOffsets = [4]int{4, 26, 49, 60}

// honeyTimeSlot 读环境变量 `DFO_HONEYTIME_SLOT`（0..3）—— 选"哪一条轨是每日轨"。
// 默认 0：实机已经证实**第 0 条就是每日轨**（我们推第 0 条的秒数把面板从"阶段 4"退回了
// "阶段 1"，别的轨不会有这个效果）。
func honeyTimeSlot() (int, bool) {
	raw := strings.TrimSpace(os.Getenv("DFO_HONEYTIME_SLOT"))
	if raw == "" {
		return 0, true
	}
	slot, err := strconv.Atoi(raw)
	if err != nil || slot < 0 || slot > 3 {
		return 0, false
	}
	return slot, true
}

// honeyTimeSlotSeqOffsets 是 4 条轨道那个「已领档数」u8 在 1294 载荷里的偏移。
//
// ★ 为什么是"已领档数"而不是"行号索引"：逐条反编译核对下来，客户端对每档存两个数 ——
// `obj + 80*档 + 148` 是**它自己按秒数算出来的档号**（`sub_143CC72A0` 写），
// `obj + 80*档 + 136` 是**服务端从流里给的那个小整数**（`sub_143CC7740` 写，且会被夹到
// "行数-1"，典型的"计数型"字段）。UI 判"这一档还有没有东西可领"用的是
// `sub_143CC6CD0`，里面就把它当**下界**比（`+148 >= 行数 && +136 >= 下界`）⇒
// 它表达的是"服务端认为已经领到第几档"，所以推 `len(Claimed)`。
//
// 第 3 条轨在流里**没有**这个字段（它的那个 u32 是尾部位图），所以 offset = 0 表示"无处可写"。
var honeyTimeSlotSeqOffsets = [4]int{8, 30, 53, 0}

// honeyTimeSlotFlagOffsets 是「标志」字节在该轨里的偏移（0 = 该轨在流里没有这一格）。
//
// ★★ 这是**"这一档参不参与"的总开关**（L0，2026-10-10 实机定案）：
// UI 填表循环第一句就是
//
//	if (*(_BYTE *)sub_143CC62F0(obj, j) == 0) continue;      // sub_143CC62F0 返回 obj + 80*j + 96
//
// 而客户端对**第 0 和第 3 条轨是常数 1**（handler 里 `v22 = 0x01000101`，只有 v8∈{1,2}
// 才从流里读那一格）⇒ **只有第 1/2 条轨能被服务端关掉**。
// 我们一开始整帧全 0 发下去 ⇒ 第 1 条轨的标志 = 0 ⇒ 面板下面那行（期间轨）**整档被跳过**、
// 点不动（业主实机："下面一行无法点击"）。标志置 1 = 参与。
var honeyTimeSlotFlagOffsets = [4]int{0, 25, 48, 0}

// honeyTimeSlotBlockOffsets 是 16 字节块在该轨里的起始偏移（**只有第 0/1 条轨在流里有这个块**；
// 第 2/3 条轨那 16 字节的位置换成了一个 u32）。0 表示"该轨没有块"。
//
// ★★ 这就是"已领取"的开关（L0，2026-10-10 定案）：
//
//	`sub_143CC7E70(obj, 档)` 按该档的行表逐行累加门槛，把"这一行（按行首值分组）该领几次"
//	写进 `obj + 40*档 + 464 + 4*行首值`；
//	`sub_143CC6F30(obj)`（"还有没有可领"的谓词）再拿这个数组**逐字节**跟我们发来的 16 字节块比：
//
//	    while (应得[v5] - 块[v5] <= 0) ++v5;      // 应得 <= 已领 ⇒ 这一行没得领
//	    if (v5 < 10 && 应得[v5] > 块[v5]) return 1;  // 应得 > 已领 ⇒ 还有可领
//
//	⇒ **块的第 k 个字节 = 行首值为 k 的那一行"已领了几次"**。全 0 = 一行都没领（面板会一直
//	把已领过的行当成可领，这就是 2026-10-10 实机"领完按钮还能点、只能靠拒绝帧弹提示"的根因）。
var honeyTimeSlotBlockOffsets = [4]int{9, 31, 0, 0}

// honeyTimeSlotLimitOffsets 是「该轨轮数（loop）」那一格在 1294 里的偏移
// （0 = 该轨在流里没有这一格）：格 1 ← `@47`、格 2 ← `@58`，格 0 / 格 3 不在流里。
//
// ★★ **这一格是期间轨能不能领的关键**（2026-10-11，反编译 `sub_143CC55E0` / `sub_143CC7E70`）：
// 客户端算"档 1 该迭代几轮"取的是 `obj + 80*档 + 140`，而 +140 由 `sub_143CC7740` 从
// **流里的 u8** 写入（格 1 ← `@47`）；档 0 的轮数在客户端自己的 24 字节表里，所以日档
// 不需要服务端给。我们此前**从不写 @47**（恒 0）⇒ 档 1 迭代 0 轮 ⇒ 逐档应得与剩余时间
// 都算成 0 ⇒ 按钮不亮。源值 = 脚本 `[reward info]`(period) 的 `[loop]`（本季 1）。
var honeyTimeSlotLimitOffsets = [4]int{0, 47, 58, 0}

// honeyTimePeriodLoop 是期间轨要写进 `@47` 的轮数；缺省 1（= 本季脚本值），
// 由 `SetHoneyTimePeriodLoop`（bootstrap 装配目录后）或 `DFO_HONEYTIME_PERIOD_LOOP` 覆盖。
var honeyTimePeriodLoop byte = 1

// SetHoneyTimePeriodLoop 由 bootstrap 在装配 313 目录后调用（目录里 `[loop]` 的值）。
func SetHoneyTimePeriodLoop(n int) {
	if n > 0 && n <= 255 {
		honeyTimePeriodLoop = byte(n)
	}
}

// honeyTimePeriodLoopByte 取这一帧要写的轮数（环境变量优先，便于不重编试别的值）。
func honeyTimePeriodLoopByte() byte {
	raw := strings.TrimSpace(os.Getenv("DFO_HONEYTIME_PERIOD_LOOP"))
	if raw == "" {
		return honeyTimePeriodLoop
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 255 {
		return honeyTimePeriodLoop
	}
	return byte(n)
}

// honeyTimeTailCountOffsets 是尾部"每档已领数"那四个 u32 的偏移（第 i 个对应第 i 条轨）。
//
// ★ 为什么盯上它：handler 结尾对**每个档**读一个 u32，把它存进 `obj+424` 的向量
// （`sub_143CC7A00`），并按下标在**同一张位图**里置位 `[档*步长, 档*步长+该值)`，
// 最后把位图当"格 3 的那个 u32"回填 —— 这是报文里**唯一**成体系表达"某档已领了几行"的字段。
var honeyTimeTailCountOffsets = [4]int{64, 68, 72, 76}

// honeyTimeClaimedMode 读环境变量 `DFO_HONEYTIME_CLAIMED`，返回要写的格子集合：
//
//	block —— 16 字节块（**按钮"可领/已领"的判据**，L0：`sub_143CC6F30` 逐字节比"应得"）
//	seq   —— 该轨的序号字段（u8@8/@30/@53）＝"已领几个"，疑为**打勾**的个数
//	tail  —— 尾部"每档已领数"（u32@64/@68/@72/@76）
//
// 支持逗号分隔组合；`all` = 三者都写（**当前缺省**），`off` = 都不写（对照）。
//
// 为什么是"集合"而不是单选：2026-10-10 实机把三格的**分工**分开了 —— 只写 `seq` 时按钮仍然
// 可点（证明 seq 不管按钮），只写 `block` 时按钮变只读但**产物框不打勾**（证明 block 管按钮、
// 不管打勾）⇒ 两者都要写。`tail` 默认不写：实测它在客户端里**只有一个读者、且只处理第 3 条轨**
// （`cmp eax,3 / jnz` 那个调用点），与日档（第 0 条）无关；但**尾部那四个数会被展开成一张位图**
// 填进第 3 条轨的 u32（handler 结尾），而位图是**全局按档分段**的 ⇒ 仍可能是打勾的来源，
// 所以缺省一并写上（见上）。
//
// 非法值**必须报错**、不能静默回落（否则会得到"换了开关却没变"的假结论，本仓踩过）；
// 而因为已领状态是**落库**的、每次进城都推，换组合只要重起看面板，不必等下一个门槛。
func honeyTimeClaimedMode() (map[string]bool, error) {
	raw := strings.TrimSpace(os.Getenv("DFO_HONEYTIME_CLAIMED"))
	if raw == "" {
		// 缺省把**三种候选都写上**：实机已经把分工拆开了（块管按钮、序号/尾部都还没验出打勾），
		// 一次重启就能看到"哪些格真的对外观有副作用"；定案后再把默认收窄成必需的那几格。
		raw = "all"
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		switch key {
		case "block", "seq", "tail":
			out[key] = true
		case "all":
			out["block"], out["seq"], out["tail"] = true, true, true
		case "off":
			// 空的集合
		default:
			return nil, fmt.Errorf("DFO_HONEYTIME_CLAIMED 取值非法（要 block/seq/tail 的逗号组合，或 all/off，收到 %q）", raw)
		}
	}
	return out, nil
}

// honeyTimeTrackByte 算"这一格在某条轨上该写什么"。ok=false = 这条轨不管这一格。
//
// 一条轨在 1294 里的四类格子（偏移表见上面）：
//
//	秒数        —— 该轨已累计的在线秒数（L0：`sub_143CC72A0` 里 `v8 = 1000 * a3`）
//	16 字节块   —— 每个"行"已领几次（L0：`sub_143CC6F30` 拿它逐字节比"应得"）
//	序号 u8     —— "已领几个"（推断，疑为打勾个数；第 3 条轨在流里没有这一格）
//	尾部位图 u32—— 每档已领数（推断；第 3 条轨那一格被 handler 填成全局位图）
func honeyTimeTrackByte(at, slot int, seconds, claimedMask uint32, claimedCount int, mode map[string]bool) (byte, bool) {
	if slot < 0 || slot > 3 {
		return 0, false
	}
	// 「标志」：这条轨只要有进度就置 1（= 参与）。不置的话整档被 UI 跳过（见该表的注释）。
	if flagOff := honeyTimeSlotFlagOffsets[slot]; flagOff != 0 && at == flagOff {
		if seconds != 0 || claimedCount != 0 {
			return 1, true
		}
		return 0, true
	}
	if sec := honeyTimeSlotOffsets[slot]; at >= sec && at < sec+4 {
		return byte(seconds >> (8 * uint(at-sec))), true
	}
	if mode["block"] && claimedCount != 0 {
		if base := honeyTimeSlotBlockOffsets[slot]; base != 0 && at >= base && at < base+16 {
			if claimedMask&(uint32(1)<<uint(at-base)) != 0 {
				return 1, true
			}
			return 0, true
		}
	}
	if mode["seq"] && claimedCount != 0 {
		if seq := honeyTimeSlotSeqOffsets[slot]; seq != 0 && at == seq {
			return byte(claimedCount), true
		}
	}
	if mode["tail"] {
		if tail := honeyTimeTailCountOffsets[slot]; at >= tail && at < tail+4 {
			return byte(uint32(claimedCount) >> (8 * uint(at-tail))), true
		}
	}
	return 0, false
}

// honeyTimeStateBody 把**日档 + 期间轨候选**的状态写进 1294，其余字节留 0：
//
//	日档           —— `DFO_HONEYTIME_SLOT`（默认 0）= 面板上面那行"每天的"
//	期间轨（候选） —— `DFO_HONEYTIME_PERIOD_SLOT`（见 `honeyTimePeriodTracks`）
//
// 只写这几格是有意的：其余字段的语义还没证，写进去等于把猜测变成契约。
// 返回的 int 是**日档**的槽位（调用方历史上只关心它）。
func honeyTimeStateBody(st honeytime.State) ([]byte, int, error) {
	slot, ok := honeyTimeSlot()
	if !ok {
		return nil, 0, fmt.Errorf("DFO_HONEYTIME_SLOT 取值非法（要 0..3）")
	}
	periodTracks, ok := honeyTimePeriodTracks(st)
	if !ok {
		return nil, 0, fmt.Errorf("DFO_HONEYTIME_PERIOD_SLOT 取值非法（要 0..3 或 all/probe）")
	}
	mode, err := honeyTimeClaimedMode()
	if err != nil {
		return nil, 0, err
	}
	body := protocol.HoneyTimeEventInfo115(uint16(honeytime.EventID), func(at int, value byte) byte {
		if v, hit := honeyTimeTrackByte(at, slot, st.Seconds, st.Claimed, st.ClaimedCount(), mode); hit {
			return v
		}
		for _, track := range periodTracks {
			if track.slot == slot {
				continue // 日档和期间轨选中同一条时以日档为准
			}
			// ★ 轮数那一格（`@47`）：不写就是 0，客户端会把这条轨当成"迭代 0 轮" ⇒ 按钮不亮。
			if track.limit != 0 && at == track.limit {
				return track.loop
			}
			if v, hit := honeyTimeTrackByte(at, track.slot, track.seconds, track.mask, track.count, mode); hit {
				return v
			}
		}
		return value
	})
	return body, slot, nil
}

// honeyTimeSave 落库（非事件键的周期写入）并把 w.role 换成刚写回的那一份。
func (w *worldSession) honeyTimeSave(ctx context.Context, st honeytime.State) error {
	if w.store == nil {
		return nil
	}
	saved, err := w.store.SaveCharacterState(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		func(current database.Character) (json.RawMessage, error) {
			return honeytime.WriteState(current.State, st)
		})
	if err != nil {
		return err
	}
	w.role = saved
	return nil
}

// honeyTimeRestore 是**登录**路径：读回存档 → 对齐到「本期 + 今天」→（真变了才落库）→ 编码。
func (w *worldSession) honeyTimeRestore(ctx context.Context, now time.Time) (honeytime.State, []byte, error) {
	if w == nil || w.role.ID == 0 {
		return honeytime.State{}, nil, fmt.Errorf("honeytime restore requires a persisted character")
	}
	stored, err := honeytime.ReadState(w.role.State)
	if err != nil {
		return honeytime.State{}, nil, err
	}
	st := stored.Normalize(int64(honeyTimeEventStart), honeytime.Today(now.Unix()))
	if st != stored {
		// 跨天 / 换期 / 首次：把归一化后的状态写回，让"今天从 0 开始"这件事也持久化。
		if saveErr := w.honeyTimeSave(ctx, st); saveErr != nil {
			return st, nil, saveErr
		}
	}
	// 锚点 = 本次上线时刻：tick 用它与 now 的差算在线秒数。
	w.honeyTimeAnchor = now
	body, _, err := honeyTimeStateBody(st)
	return st, body, err
}

// honeyTimeTick 是**周期**路径（挂在 30 秒的 dailyTicker 上）：把这段时间的在线秒数累计进去、
// 落库，并在值真的变了时回一帧 1294 让面板跟着走。
//
// log 把「这一跳发生了什么」记进会话日志（可为 nil）。**每一跳都记**，不管有没有推帧 ——
// 这一点是踩过坑之后加的：tick 推的 1294 走 `sendPlan`，而 `sendPlan` **不逐包记日志**
// （进城那条 `honey_time_state_sent` 是 client_entry.go 自己打的），所以"日志里没有
// honey_time_state_sent ⇒ tick 没跑"是个**假结论** —— 2026-10-10 实测：tick 一直在跑、
// 存档里 seconds 已经涨到 933，而会话日志里一条相关记录都没有，白查了一轮。
func (w *worldSession) honeyTimeTick(ctx context.Context, now time.Time, log func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, nil
	}
	elapsed := int64(0)
	if !w.honeyTimeAnchor.IsZero() {
		elapsed = int64(now.Sub(w.honeyTimeAnchor).Seconds())
	}
	w.honeyTimeAnchor = now
	trace := func(kind string, st honeytime.State) {
		if log == nil {
			return
		}
		log(map[string]any{
			"kind":         kind,
			"character_id": w.role.ID,
			"elapsed":        elapsed,
			"day":            st.Day,
			"seconds":        st.Seconds,
			"claimed":        st.ClaimedCount(),
			"period_seconds": st.PeriodSeconds,
			"period_claimed": st.PeriodClaimedCount(),
		})
	}
	if elapsed <= 0 {
		// 锚点刚设过（登录）或时钟回拨：不累计。记下来，否则"为什么没涨"无从查。
		trace("honey_time_tick_idle", honeytime.State{})
		return nil, nil
	}
	stored, err := honeytime.ReadState(w.role.State)
	if err != nil {
		return nil, err
	}
	st := stored.Normalize(int64(honeyTimeEventStart), honeytime.Today(now.Unix()))
	// 两条轨都累计：日档按天重来，期间轨是整期累计（跨天不清，见 honeytime.Normalize）。
	periodCap := uint32(0)
	if w.honeyTime != nil {
		periodCap = w.honeyTime.PeriodCapacity()
	}
	dailyChanged := st.AddSeconds(uint32(elapsed))
	periodChanged := st.AddPeriodSeconds(uint32(elapsed), periodCap)
	if !dailyChanged && !periodChanged {
		trace("honey_time_tick_idle", st) // 未变化（或两条轨都封顶）：不推、不写
		return nil, nil
	}
	if err = w.honeyTimeSave(ctx, st); err != nil {
		return nil, err
	}
	body, _, err := honeyTimeStateBody(st)
	if err != nil {
		return nil, err
	}
	trace("honey_time_tick_sent", st)
	return []outboundPacket{{"honey_time_state_sent", 0, honeyTimeNotiID, body}}, nil
}

// logHoneyTimeState 把这一帧记进会话日志（秒数与选中的轨道，便于与面板逐一对照）。
func (client *gameConnection) logHoneyTimeState(roleID int64, st honeytime.State, body []byte) {
	slot, _ := honeyTimeSlot()
	client.event(map[string]any{
		"kind":         "honey_time_state_sent",
		"id":           honeyTimeNotiID,
		"character_id": roleID,
		"bytes":        len(body),
		"event_id":     honeytime.EventID,
		"day":            st.Day,
		"seconds":        st.Seconds,
		"claimed":        st.ClaimedCount(),
		"period_seconds": st.PeriodSeconds,
		"period_claimed": st.PeriodClaimedCount(),
		"slot":           slot,
		"plain_hex":    hex.EncodeToString(body),
	})
}

// honeyTimeSecondsAt 读回某一轨的 u32（小端），自检与用例用。
func honeyTimeSecondsAt(body []byte, slot int) uint32 {
	if slot < 0 || slot > 3 {
		return 0
	}
	offset := honeyTimeSlotOffsets[slot]
	if offset+4 > len(body) {
		return 0
	}
	return binary.LittleEndian.Uint32(body[offset:])
}
