package honeytime

// 活动 313 HoneyTime 的**源驱动内容**：解析 `honeytime.evt`，得到日档的行表。
//
// ## 为什么按脚本解析而不是把四个物品号写死
//
// 脚本里日档一共三块 `[reward data]`（`[loop] 3`）、外加 `[day of week reward info]`
// 与 `[day reward info]` 两套替换品，物品号随运营日变。写死等于把"这一期的运营数据"
// 当成"玩法规则"编进二进制；换期就得改代码重编。所以和签到（`internal/attendance`）
// 一样：**只解析、不硬编码**。
//
// ## ★ 关键口径：阈值是「每档时长」，不是绝对门槛
//
// 客户端 `sub_143CC72A0(obj, 档, 秒)` 算"走到第几档"时是**逐行相减**：
//
//	剩 = 秒 * 1000
//	逐行: if (剩 <= 该行阈值) 停; 剩 -= 该行阈值; step = 该行第一个值 + 1
//
// ⇒ 脚本里 `600000 1200000 1800000 3600000` 是**每档的时长**，
// 四档的**累计**门槛是 10 / 30 / 60 / 120 分钟（600 / 1800 / 3600 / 7200 秒）。
// 实机印证：进城 17 分 35 秒时客户端发的是**第 1 档**（10 分钟档）的领奖请求，
// 面板显示"第 1 档可领、进度条在走第 2 档"。
//
// ⚠️ 另有一张 **24 字节/行**的表（`sub_143CC6270` 一族，按档索引，用来夹行号）。
// 别把它和上面这张 **21 字节/行**的行表搞混 —— 两张表的行数不一样。
//
// ## 领奖行号
//
// 客户端发来的行号 = **脚本每行的第一个值（`field0`，0 起）** —— 实机两样本：
//
//	点第 1 档 → 载荷第 3 个 u32 = 0
//	点第 2 档 → 载荷第 3 个 u32 = 1
//
// ⇒ 行号 **就是 `Tier.Index`**，不需要 ±1（`TierForClaim`）。
import (
	"fmt"
	"strconv"
	"strings"

	"dfolan/internal/attendance"
)

// Reward 是一档的奖励（`[reward data]` 每行只有一件）。
type Reward struct {
	Template uint32
	Count    uint32
}

// Tier 是日档里的一档。
type Tier struct {
	// Index 是脚本每行的**第一个值**（`field0`）。它不是行号本身 ——
	// 客户端把"行号"算成 `Index + 1`，所以两者差一（见包注释）。
	Index int
	// DurationMillis 是该档的时长（毫秒，脚本每行最后一个值）。累计门槛 = 前面各档之和。
	DurationMillis uint32
	// Flag 是脚本每行第 4 个值（本机脚本里日档恒 0、期间档后两行是 1）。
	// **语义未证**（疑与 `[pc room event]` / 周期性有关），所以只留档、不参与判定。
	Flag   uint32
	Reward Reward
}

// Catalog 是 `honeytime.evt` 解析出来的内容。
type Catalog struct {
	// ScriptPath 是这份内容**实际来自哪一份脚本**（由 `list/event.lst` 索引解析，见
	// ResolveScriptPath）。只作溯源：日志把它打出来，才能证明读的是索引指定的那一份。
	ScriptPath string
	// Daily 是日档的行表，按脚本顺序，取**第一块** `[reward data]`。
	Daily []Tier
	// Period 是**期间轨**（面板下面那行"累计在线时长"）的行表，取 period 段的第一块 `[reward data]`。
	// 索引版脚本（`list/event.lst` 的 313 绑定）**4 档**，阈值
	// `18000000 / 54000000 / 108000000 / 108000000` ms
	// ⇒ 累计门槛 **5 / 20 / 50 / 80 小时**；第 3、4 档的物品是 `[booster selection]`（玩家自选箱）。
	// ⚠️ 2026-10-11 之前这里读的是**另一份没被索引引用的脚本**（5 档 × 20 小时），见 ResolveScriptPath。
	Period []Tier
	// Loop 是**日档**那条 `[reward info]` 的 `[loop]`（本季 3；证明日档确实分块）。
	Loop int
	// PeriodLoop 是**期间轨**那条 `[reward info]` 的 `[loop]`（本季 1）。
	//
	// ★★ 它不是「只留档」：客户端算「档 1 该迭代几轮」取的就是**服务端从流里给的那个 u8**
	// （1294 的 `@47`，落进 `obj+80*档+140`；见 `sub_143CC55E0` / `sub_143CC7E70`）。
	// 推 0 会让客户端迭代 0 轮 ⇒ 逐档应得与剩余时间都算成 0 ⇒ 期间轨的按钮不亮。
	// 日档不用我们给：它的轮数在客户端自己的 24 字节表里。
	PeriodLoop int
	// DailyOffsetCount / PeriodOffsetCount 是 `[max reward offset count]` 的两个值。
	DailyOffsetCount  int
	PeriodOffsetCount int
	// PostalType 是 `[reward postal type]`（1 = 走邮件）。
	PostalType int
	// PCRoom 是 `[pc room event]`。
	PCRoom bool
	// UIFile 是 `[ui file]`，仅留档。
	UIFile string
	// WindowStart / WindowEnd 是 `[control panel]` 的运营窗口，仅留档。
	WindowStart string
	WindowEnd   string
	// Groups 是本脚本出现过的 `[reward type]`，**按脚本顺序**留档。
	//
	// ★ 上游那份独立反编译（同 reader）给出本活动的四个取值：`daily` / `period` /
	// `specified` / `extend_daily`（UI 四个数组 ↔ CMD680 的 action 1/2/3/5，跳过 4）。
	// 本季 evt 只有前两个 ⇒ 后两个**已登记但未建模**：出现即记在这里，由
	// `UnmodelledGroups` 标明（不静默当成 daily，也不报错）。
	Groups []string
	// UnmodelledGroups 是 Groups 里**认得但没有建模**的那几个（当前 = specified/extend_daily）。
	UnmodelledGroups []string
}

// TextSource 只要「按路径读一份脚本文本」这一件事 —— 与签到/成长胶囊共用同一个已打开的
// 归档，不第二次加载 PVF。
type TextSource interface {
	Text(string) (string, error)
}

// Load 读并解析 HoneyTime 脚本：路径**从 `list/event.lst` 的索引解析**。
func Load(src TextSource) (*Catalog, error) {
	path, err := ResolveScriptPath(src)
	if err != nil {
		return nil, err
	}
	return LoadPath(src, path)
}

// LoadPath 读并解析**指定路径**的 HoneyTime 脚本。
//
// 单独开出来是为了两件事：① 让 Load 的索引解析可以被单测喂一份假索引替换；
// ② 给"已经知道路径"的调用方/诊断工具留一条直读路（不做任何猜测）。
func LoadPath(src TextSource, path string) (*Catalog, error) {
	text, err := src.Text(path)
	if err != nil {
		return nil, fmt.Errorf("honeytime 脚本 %s: %w", path, err)
	}
	c, err := Parse(text)
	if err != nil {
		return nil, err
	}
	c.ScriptPath = path
	return c, nil
}

// ResolveScriptPath 按 `list/event.lst` 的索引解析活动 EventID（313）的脚本路径。
//
// ## 为什么必须走索引（★ 2026-10-11 的实机教训）
//
// **客户端加载哪一份 evt 由这个索引决定**，不是由目录名、日期或"看起来最像的那一份"决定。
// 本仓实测：索引里 `313` 唯一绑定到
//
//	Live/Event/Kor/2023/1026_HoneyTime/HoneyTime.evt
//
// 而仓库里另有一份 `live/event/kor/2026/0326_honeytime/honeytime.evt`，**没有任何索引引用**。
// 我们原来写死读后者 ⇒ 面板（客户端从索引那份画）与发奖（我们从 2026 那份取）用了两套物品号：
//
//	客户端（索引版，4 档）：590015973 / 590015974 / 590015976 / 590015977
//	我们（2026 版，5 档）：10420527 / 10421322 / 10421302 / 10420522 / 10420523
//
// 玩家看到的就是"面板图标与邮件奖励对不上"。三条独立旁证：① `[max reward offset count]`
// 索引版是 `period 4`、2026 版是 `period 5`；② `[ui file]` 两份不同
// （`UI/Event/PCRoomTimePoint/upgradeDailyEventWindow_20221229.xui` vs `…/0326_HoneyTime/body.xui`）；
// ③ 索引版的第 3、4 档是 `[booster selection]`（玩家自选箱），2026 版不是。
//
// ## 口径
//
// 与上游独立反编译（`activity.HoneyNativeEventPath`）逐条一致：token 两两成对 `(id, path)`；
// **重复绑定报错**；路径把反斜杠转正斜杠、转小写后必须是非绝对、无 `..` 的 `.evt`。
// 找不到绑定同样报错 —— **不设回退**：拿一份来源不明的脚本发奖，比不装配这个活动更危险。
func ResolveScriptPath(src TextSource) (string, error) {
	text, err := src.Text(EventListPath)
	if err != nil {
		return "", fmt.Errorf("honeytime 索引 %s: %w", EventListPath, err)
	}
	tokens := attendance.Tokenize(text)
	if len(tokens) == 0 || len(tokens)%2 != 0 {
		return "", fmt.Errorf("honeytime 索引 %s: 条目不是成对的 (id, path)（%d 个 token）", EventListPath, len(tokens))
	}
	found := ""
	for i := 0; i+1 < len(tokens); i += 2 {
		id, convErr := strconv.ParseUint(tokens[i], 10, 32)
		if convErr != nil {
			return "", fmt.Errorf("honeytime 索引 %s: 第 %d 项的 id %q 不是数字", EventListPath, i/2, tokens[i])
		}
		if uint32(id) != EventID {
			continue
		}
		path := strings.ToLower(strings.ReplaceAll(tokens[i+1], `\`, "/"))
		if err := validateIndexPath(path); err != nil {
			return "", fmt.Errorf("honeytime 索引 %s: 活动 %d 的路径%w", EventListPath, EventID, err)
		}
		if found != "" {
			return "", fmt.Errorf("honeytime 索引 %s: 活动 %d 有重复绑定", EventListPath, EventID)
		}
		found = path
	}
	if found == "" {
		return "", fmt.Errorf("honeytime 索引 %s: 没有活动 %d 的绑定", EventListPath, EventID)
	}
	return found, nil
}

// validateIndexPath 只接受索引里那种"仓库内的相对 .evt 路径"：非绝对、无 `..`、后缀 `.evt`。
// 三条检查与上游 `HoneyNativeEventPath` 同义。
func validateIndexPath(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("是空串")
	case strings.HasPrefix(path, "/"):
		return fmt.Errorf("是绝对路径（%s）", path)
	case strings.Contains(path, ".."):
		return fmt.Errorf("含 `..`（%s）", path)
	case !strings.HasSuffix(path, ".evt"):
		return fmt.Errorf("不是 .evt（%s）", path)
	}
	return nil
}

// Parse 解析 honeytime.evt 的文本。
//
// 脚本形态与签到同一种方言（`[tag]` + 反引号值 / 裸值段），所以直接复用
// `attendance.Tokenize` 拍平，再走一遍标签。
//
// 只认**第一块** `[reward data]` 当日档：`[loop] 3` 说明后面还有两块，它们是把
// "第 3 档（60 分钟档）"换成别的物品的运营变体（本机脚本里是 10414140 / 10414141），
// 另有 `[day of week reward info]` / `[day reward info]` 两套按星期/日期替换的表。
// 这些替换规则属于**运营日程**，本期（313 的窗口是 2026-03-26 → 05-07）用不上，
// 所以整块跳过并在下面显式记下"跳过了几块"——将来真要按星期替换，从这里接手。
func Parse(text string) (*Catalog, error) {
	tokens := attendance.Tokenize(text)
	c := &Catalog{}
	inDaily := false
	inPeriod := false
	sawRewardType := false
	skippedBlocks := 0
	for i := 0; i < len(tokens); i++ {
		tag := tokens[i]
		if !strings.HasPrefix(tag, "[") {
			return nil, fmt.Errorf("honeytime script: 意外的裸值 %q", tag)
		}
		switch tag {
		case "[reward info]":
			inDaily = false
			inPeriod = false
			sawRewardType = false
		case "[/reward info]":
			inDaily = false
			inPeriod = false
			sawRewardType = false
		case "[reward type]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [reward type] %w", err)
			}
			if sawRewardType {
				return nil, fmt.Errorf("honeytime script: 一个 [reward info] 里出现两次 [reward type]")
			}
			sawRewardType = true
			inDaily = value == "daily"
			inPeriod = value == "period"
			c.Groups = append(c.Groups, value)
			switch value {
			case "daily", "period":
			case "specified", "extend_daily":
				// 上游登记的另外两个取值：**认得、不建模**。它们的 [reward data] 由下面的
				// 分支计入 skippedBlocks（不会污染 daily/period），这里只留档、不猜语义。
				c.UnmodelledGroups = append(c.UnmodelledGroups, value)
			default:
				return nil, fmt.Errorf("honeytime script: 未知 [reward type] %q"+
					"（已知 daily/period/specified/extend_daily）", value)
			}
			i = next
		case "[loop]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [loop] %w", err)
			}
			if inDaily || inPeriod {
				n, convErr := strconv.Atoi(value)
				if convErr != nil || n <= 0 {
					return nil, fmt.Errorf("honeytime script: [loop] 取值 %q 非法", value)
				}
				if inDaily {
					c.Loop = n
				} else {
					// 期间轨的轮数要发给客户端（1294 @47）—— 见 PeriodLoop 的注释。
					c.PeriodLoop = n
				}
			}
			i = next
		case "[reward data]":
			values, next, err := until(tokens, i+1, "[/reward data]")
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [reward data] %w", err)
			}
			// 只有日档的**第一块**是我们要的；后面的块（以及星期/日期替换表里的块，
			// 那些已经在 default 分支整块跳过）一律不采。
			if inDaily && c.Daily == nil {
				tiers, parseErr := parseRewardRows(values)
				if parseErr != nil {
					return nil, parseErr
				}
				c.Daily = tiers
			} else if inPeriod && c.Period == nil {
				tiers, parseErr := parseRewardRows(values)
				if parseErr != nil {
					return nil, parseErr
				}
				c.Period = tiers
			} else if inDaily || inPeriod {
				skippedBlocks++
			}
			i = next
		case "[max reward offset count]":
			// `daily` 4 `period` 5
			values, next, err := until(tokens, i+1, "[/max reward offset count]")
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [max reward offset count] %w", err)
			}
			for k := 0; k+1 < len(values); k += 2 {
				n, convErr := strconv.Atoi(values[k+1])
				if convErr != nil {
					return nil, fmt.Errorf("honeytime script: [max reward offset count] 的 %q 不是数字", values[k+1])
				}
				switch values[k] {
				case "daily":
					c.DailyOffsetCount = n
				case "period":
					c.PeriodOffsetCount = n
				}
			}
			i = next
		case "[reward postal type]", "[ui file]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("honeytime script: %s %w", tag, err)
			}
			if tag == "[reward postal type]" {
				n, convErr := strconv.Atoi(value)
				if convErr != nil {
					return nil, fmt.Errorf("honeytime script: [reward postal type] 取值 %q 非法", value)
				}
				c.PostalType = n
			} else {
				c.UIFile = value
			}
			i = next
		case "[pc room event]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [pc room event] %w", err)
			}
			c.PCRoom = value == "true" || value == "1"
			i = next
		case "[control panel]":
			// `键` `开始` `结束`；端点文案是运营数据，只留档。
			values, next, err := until(tokens, i+1, "[/control panel]")
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [control panel] %w", err)
			}
			if len(values) >= 3 {
				c.WindowStart, c.WindowEnd = values[1], values[2]
			}
			i = next
		default:
			// 不认识的标签块整块跳过（含所有"星期/日期替换"与"横幅"块 ——
			// 它们内部也有 [reward data]，靠这一步整块吃掉，不会污染日档）。
			if next := skipBlock(tokens, i); next > i {
				skippedBlocks++
				i = next
			}
		}
	}
	if len(c.Daily) == 0 {
		return nil, fmt.Errorf("honeytime script: 日档没有任何 [reward data]")
	}
	// 自检：`[max reward offset count] daily` 说的是日档有几档，必须与第一块的行数一致。
	// 不一致说明"哪一块是日档"认错了（或脚本换了形态），宁可拒绝也不要拿错物品发奖。
	if c.DailyOffsetCount != 0 && c.DailyOffsetCount != len(c.Daily) {
		return nil, fmt.Errorf("honeytime script: 日档行数 %d 与 [max reward offset count] daily=%d 不符",
			len(c.Daily), c.DailyOffsetCount)
	}
	// 自检：期间轨行数必须等于 `[max reward offset count] period`（同日档同一条理由：
	// 不一致说明"哪一块是哪条轨"认错了，宁可拒绝也不要发错物品）。
	if c.PeriodOffsetCount != 0 && c.PeriodOffsetCount != len(c.Period) {
		return nil, fmt.Errorf("honeytime script: 期间轨行数 %d 与 [max reward offset count] period=%d 不符",
			len(c.Period), c.PeriodOffsetCount)
	}
	if c.Loop != 0 && c.Loop < 1 {
		return nil, fmt.Errorf("honeytime script: [loop] 取值非法")
	}
	return c, nil
}

// parseRewardRows 把 `[reward data]` 的值流切成行。
//
// 每行 **5 个值**：`第一个值 物品 数量 ? 时长(毫秒)`。最后那个"?"在本机脚本里恒 0，
// 语义未证（不去猜它），解析时只校验它是数字。
func parseRewardRows(values []string) ([]Tier, error) {
	const width = 5
	if len(values) == 0 || len(values)%width != 0 {
		return nil, fmt.Errorf("honeytime script: [reward data] 有 %d 个值，不是 %d 的整数倍", len(values), width)
	}
	tiers := make([]Tier, 0, len(values)/width)
	for k := 0; k < len(values); k += width {
		nums := make([]uint64, width)
		for j := 0; j < width; j++ {
			n, err := strconv.ParseUint(values[k+j], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("honeytime script: [reward data] 的 %q 不是数字", values[k+j])
			}
			nums[j] = n
		}
		if nums[0] > 255 {
			return nil, fmt.Errorf("honeytime script: [reward data] 的行首值 %d 越界（客户端把它当 u8）", nums[0])
		}
		if nums[2] == 0 {
			return nil, fmt.Errorf("honeytime script: [reward data] 第 %d 行的数量为 0", k/width)
		}
		tiers = append(tiers, Tier{
			Index:          int(nums[0]),
			DurationMillis: uint32(nums[4]),
			Flag:           uint32(nums[3]),
			Reward:         Reward{Template: uint32(nums[1]), Count: uint32(nums[2])},
		})
	}
	return tiers, nil
}

// ---- 查询 ----

// StepNumber 复刻客户端 `sub_143CC72A0` 的"走到第几档"，返回**1 起的档号**；
// 0 = 一档都没走到。这就是客户端领奖请求里那个行号。
func (c *Catalog) StepNumber(seconds uint32) int {
	if c == nil {
		return 0
	}
	remaining := uint64(seconds) * 1000 // 客户端也是先 ×1000 变毫秒再比
	step := 0
	for _, t := range c.Daily {
		if remaining <= uint64(t.DurationMillis) {
			break
		}
		remaining -= uint64(t.DurationMillis)
		step = t.Index + 1
	}
	return step
}

// CumulativeSeconds 是第 i 档（0 起）解锁所需的**累计**秒数。
// i 越界时返回最后一档的累计值。
func (c *Catalog) CumulativeSeconds(i int) uint32 {
	if c == nil || len(c.Daily) == 0 {
		return 0
	}
	if i >= len(c.Daily) {
		i = len(c.Daily) - 1
	}
	var total uint64
	for k := 0; k <= i; k++ {
		total += uint64(c.Daily[k].DurationMillis)
	}
	return uint32(total / 1000)
}

// MinutesForTier 返回某一档（按脚本行首值）解锁所需的**累计分钟数**。
//
// 只用于**面向玩家的文案**（邮件正文）：脚本里的门槛是"每档时长"，而玩家看到的里程碑是
// 累计值（10/30/60/120 分钟）。累计口径与 `StepNumber` 同一套，不会漂。
func (c *Catalog) MinutesForTier(index int) (int, bool) {
	if c == nil {
		return 0, false
	}
	var total uint64
	for _, t := range c.Daily {
		total += uint64(t.DurationMillis)
		if t.Index == index {
			return int(total / 60000), true
		}
	}
	return 0, false
}

// PeriodStepNumber 与 `StepNumber` 同构，只是走期间轨；返回 **1 起的档号**，0 = 一档都没到。
func (c *Catalog) PeriodStepNumber(seconds uint32) int {
	if c == nil {
		return 0
	}
	remaining := uint64(seconds) * 1000
	step := 0
	for _, t := range c.Period {
		if remaining <= uint64(t.DurationMillis) {
			break
		}
		remaining -= uint64(t.DurationMillis)
		step = t.Index + 1
	}
	return step
}

// PeriodCumulativeSeconds 是期间轨第 i 档（按切片下标）解锁所需的**累计**秒数。
func (c *Catalog) PeriodCumulativeSeconds(i int) uint32 {
	if c == nil || len(c.Period) == 0 {
		return 0
	}
	if i >= len(c.Period) {
		i = len(c.Period) - 1
	}
	var total uint64
	for k := 0; k <= i; k++ {
		total += uint64(c.Period[k].DurationMillis)
	}
	return uint32(total / 1000)
}

// PeriodMinutesForTier 与 `MinutesForTier` 同构（邮件文案用）。
func (c *Catalog) PeriodMinutesForTier(index int) (int, bool) {
	if c == nil {
		return 0, false
	}
	var total uint64
	for _, t := range c.Period {
		total += uint64(t.DurationMillis)
		if t.Index == index {
			return int(total / 60000), true
		}
	}
	return 0, false
}

// PeriodTierForClaim 与 `TierForClaim` 同构，但走期间轨；口径一样是**行号 = 行首值（0 起）**。
func (c *Catalog) PeriodTierForClaim(k int) (Tier, bool) {
	if c == nil || k < 0 {
		return Tier{}, false
	}
	for _, t := range c.Period {
		if t.Index == k {
			return t, true
		}
	}
	return Tier{}, false
}

// PeriodCapacity 是期间轨的封顶（按脚本算：索引版 = 5h + 15h + 30h + 30h = 80 小时 = 288000 秒）。
// 与日档同一条理由：宁可存大数也别把合法值截掉。
// 名字不叫 MaxPeriodSeconds 是为了给"不读 PVF 的工具"留一个包级常量（见 honeytime.go）。
func (c *Catalog) PeriodCapacity() uint32 {
	if c == nil {
		return 0
	}
	var total uint64
	for _, t := range c.Period {
		total += uint64(t.DurationMillis)
	}
	return uint32(total / 1000)
}

// TierForClaim 把客户端发来的领奖行号折算成日档里的一档。
//
// **行号就是脚本行首值（`Tier.Index`），0 起、不做 ±1**（见包注释；实机第 1 档发 0、第 2 档发 1）。
// 越界或没有对应行时 ok=false。
//
// ⚠️ 这里差一会**静默**出错：第 1 档照样对，第 2 档会被折回第 1 档、再去领一遍已领过的奖励
// （客户端看到的是"点了没反应"），所以值一定钉死在用例里。
func (c *Catalog) TierForClaim(k int) (Tier, bool) {
	if c == nil || k < 0 {
		return Tier{}, false
	}
	for _, t := range c.Daily {
		if t.Index == k {
			return t, true
		}
	}
	return Tier{}, false
}

// MaxStep 是日档能给出的最大档号。
func (c *Catalog) MaxStep() int {
	max := 0
	if c == nil {
		return max
	}
	for _, t := range c.Daily {
		if t.Index+1 > max {
			max = t.Index + 1
		}
	}
	return max
}

// ---- 解析小工具（标签 + 值，与 attendance 同款形态）----

func scalar(tokens []string, i int) (string, int, error) {
	if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "[") {
		return "", i, fmt.Errorf("缺少取值")
	}
	return tokens[i+1], i + 1, nil
}

func until(tokens []string, from int, terminator string) ([]string, int, error) {
	var values []string
	for i := from; i < len(tokens); i++ {
		if tokens[i] == terminator {
			return values, i, nil
		}
		if strings.HasPrefix(tokens[i], "[") {
			return nil, i, fmt.Errorf("遇到未闭合的 %s（在 %s 之前）", tokens[i], terminator)
		}
		values = append(values, tokens[i])
	}
	return nil, len(tokens), fmt.Errorf("找不到 %s", terminator)
}

// skipBlock 跳过 i 处的一个块：
//   - 有配对 `[/x]` ⇒ 跳到它之后；
//   - 没有配对 ⇒ 把它后面**连续的裸值**全部吃掉（**不止一个**）。
//
// 为什么必须吃掉"连续多个"：`[period reward weekly loop increase value]` 这种标签没有
// 收尾标签、后面还跟着**两个**裸值（`0 1`，各占一行）。只跳一个的话，第二个裸值会在下一轮
// 循环里撞上"意外的裸值"直接报错整份脚本 —— 本仓实测就是这么挂的。
func skipBlock(tokens []string, i int) int {
	open := tokens[i]
	close := "[/" + strings.TrimPrefix(open, "[")
	for j := i + 1; j < len(tokens); j++ {
		if tokens[j] == close {
			return j
		}
	}
	next := i
	for next+1 < len(tokens) && !strings.HasPrefix(tokens[next+1], "[") {
		next++
	}
	return next
}
