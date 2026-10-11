package honeytime

import (
	"testing"
)

// scriptFixture 是仓库里 `live/event/kor/2026/0326_honeytime/honeytime.evt` 的**逐字导出**
// （去掉了几处与本用例无关的日期横幅，其余原样）。用内联字面量而不是读 dump 文件：
// 解析器要能在没有归档的环境里被钉住。
//
// ⚠️ 这一份**不是**客户端实际加载的那一份：`list/event.lst` 里 313 绑的是
// `.../2023/1026_HoneyTime/HoneyTime.evt`（2026-10-11 订正，见 ResolveScriptPath）。
// 留着它是为了钉**解析器形态**（`[loop] 3` 的多块日档、`[day of week reward info]` 替换表、
// `[unlimit reward data]` —— 最容易解析错的三处）；索引版的形状由 index_test.go 覆盖。
//
// 刻意**保留**了最容易把解析器带错的三块：
//   - `[loop] 3` ⇒ 日档有三块 `[reward data]`（提取器只能取第一块）；
//   - `[unlimit reward data]`（形态和 `[reward data]` 像，但键不同）；
//   - `[day of week reward info]` / `[day reward info]`：里面**也有** `[reward data]`，
//     物品号与日档不同（10420394/95/96）。取错就会发错物品。
//
// 另外保留 `[reward info] [reward type] period` 那一段（物品 10420527 一族），同理。
const scriptFixture = `
[control panel]
` + "`event_1703_honeytime_info` `2026-03-26 06:00:00` `2026-05-07 06:00:00`" + `
[/control panel]

[reward postal type]
1 
[pc room event]
` + "`true`" + `
[ui file]
` + "`Live/Event/Kor/2026/0326_HoneyTime/body.xui`" + `
[max reward offset count]
` + "`daily` 4 `period` 5" + `
[/max reward offset count]

[inner reward ban]
` + "`daily` 1 `period` 1" + `
[/inner reward ban]

[reward info]

[reward type]
` + "`daily`" + `
[loop]
3 
[reward data]
0 10414136 1 0 600000 1 10414137 1 0 1200000 2 10414138 1 0 1800000 3 10414139 1 0 3600000 
[/reward data]

[reward data]
3 10414140 1 0 3600000 
[/reward data]

[reward data]
3 10414141 1 0 3600000 
[/reward data]

[apply unlimit flag]
0 
[unlimit reward data]
0 3033 1 0 100000 1 3034 1 0 200000 2 3035 5 0 300000 3 3036 1 0 400000 
[/unlimit reward data]

[day of week reward info]
5 
[reward data]
0 10414136 1 0 600000 1 10414137 1 0 1200000 2 10414138 1 0 1800000 3 10420394 1 0 3600000 
[/reward data]

[reward data]
3 10420395 1 0 3600000 
[/reward data]

[/day of week reward info]

[day reward info]
20260505 
[reward data]
0 10420394 1 0 600000 1 10420395 1 0 1200000 
[/reward data]

[/day reward info]

[/reward info]

[period reward weekly loop increase value]
0 1 
[reward info]

[reward type]
` + "`period`" + `
[loop]
1 
[reward data]
0 10420527 1 0 72000000 1 10421322 4 0 72000000 2 10421302 1 0 72000000 3 10420522 1 1 72000000 4 10420523 1 1 72000000 
[/reward data]

[/reward info]

[extend daily reward loop]
0 
[extend daily reward items]

[/extend daily reward items]
`

// TestParseRealScriptShape 钉住「日档提取到手的是什么」。
func TestParseRealScriptShape(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析真脚本失败: %v", err)
	}
	if len(c.Daily) != 4 {
		t.Fatalf("日档应恰好 4 档（[max reward offset count] daily 4），得到 %d: %+v", len(c.Daily), c.Daily)
	}
	wantTemplates := []uint32{10414136, 10414137, 10414138, 10414139}
	wantDurations := []uint32{600000, 1200000, 1800000, 3600000}
	for i, tier := range c.Daily {
		if tier.Index != i {
			t.Fatalf("第 %d 档的 Index = %d，应为 %d", i, tier.Index, i)
		}
		if tier.Reward.Template != wantTemplates[i] {
			t.Fatalf("第 %d 档物品 = %d，应为 %d（取错块会变成 10420394 那套）", i, tier.Reward.Template, wantTemplates[i])
		}
		if tier.Reward.Count != 1 {
			t.Fatalf("第 %d 档数量 = %d，应为 1", i, tier.Reward.Count)
		}
		if tier.DurationMillis != wantDurations[i] {
			t.Fatalf("第 %d 档时长 = %d，应为 %d", i, tier.DurationMillis, wantDurations[i])
		}
	}
	if c.Loop != 3 || c.DailyOffsetCount != 4 || c.PeriodOffsetCount != 5 {
		t.Fatalf("元信息不对: loop=%d daily=%d period=%d", c.Loop, c.DailyOffsetCount, c.PeriodOffsetCount)
	}
	// ★ 期间轨的 [loop]（本季 1）要发给客户端（1294 @47）：客户端拿它当「档 1 迭代几轮」，
	// 推 0 会让档 1 的逐档应得与剩余时间算成 0、按钮不亮。
	if c.PeriodLoop != 1 {
		t.Fatalf("期间轨 [loop] = %d，应为 1（1294 @47 要发它）", c.PeriodLoop)
	}
	if c.PostalType != 1 || !c.PCRoom {
		t.Fatalf("postal/pcroom 不对: %d/%v", c.PostalType, c.PCRoom)
	}
	if c.WindowStart != "2026-03-26 06:00:00" || c.WindowEnd != "2026-05-07 06:00:00" {
		t.Fatalf("运营窗口不对: %q .. %q", c.WindowStart, c.WindowEnd)
	}
	// 反面：别的块的物品号一个都不许混进来。
	for _, banned := range []uint32{10414140, 10414141, 10420394, 10420395, 10420527, 3033} {
		for _, tier := range c.Daily {
			if tier.Reward.Template == banned {
				t.Fatalf("物品 %d 属于别的块（loop 变体 / 星期替换 / 期间档 / 无限制档），不该进日档", banned)
			}
		}
	}
}

// TestStepNumberMatchesClientWalk 钉住"走到第几档"这个算法 —— 它必须与客户端
// `sub_143CC72A0` 的逐行相减**逐字一致**（包括那个 `<=` 的边界）。
//
// 客户端：
//
//	剩 = 秒*1000; 逐行 { if (剩 <= 该行时长) 停; 剩 -= 该行时长; step = 该行行首值 + 1 }
//
// ⇒ 累计门槛是 600 / 1800 / 3600 / 7200 秒，且**恰好等于门槛的那一秒还不算过**
//   （`<=` 让它停下）—— 这个差一是客户端的语义，不是我们的 bug。
func TestStepNumberMatchesClientWalk(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	cases := []struct {
		seconds uint32
		step    int
	}{
		{0, 0},
		{599, 0},
		{600, 0}, // 恰好 600 s：客户端 `<=` 停下，仍是第 0
		{601, 1}, // 第 1 档（10 分钟档）到手
		{1800, 1},
		{1801, 2},
		{3601, 3},
		{7200, 3},
		{7201, 4},
		{14399, 4},
		{14400, 4},
	}
	for _, tc := range cases {
		if got := c.StepNumber(tc.seconds); got != tc.step {
			t.Fatalf("StepNumber(%d) = %d，应为 %d", tc.seconds, got, tc.step)
		}
	}
	// 实机锚点：进城 17 分 35 秒（1055 s）时客户端发的行号就是 1。
	if got := c.StepNumber(1055); got != 1 {
		t.Fatalf("1055 秒的档号 = %d，实机客户端此时发的是 1", got)
	}
}

// TestCumulativeSeconds 钉住累计门槛（第 i 档 0 起）。
func TestCumulativeSeconds(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := []uint32{600, 1800, 3600, 7200}
	for i, w := range want {
		if got := c.CumulativeSeconds(i); got != w {
			t.Fatalf("CumulativeSeconds(%d) = %d，应为 %d", i, got, w)
		}
	}
	// 越界取最后一档，不 panic。
	if got := c.CumulativeSeconds(99); got != 7200 {
		t.Fatalf("越界应取最后一档 7200，得到 %d", got)
	}
}

// TestTierForClaimMapsRowToTier 钉住领奖行号口径：**行号 = 脚本行首值（0 起），不做 ±1**。
//
// 实机锚点（2026-10-10，同一会话两次点击）：
//
//	点第 1 档 → 680 载荷第 3 个 u32 = 0 → 应给 10414136（10 分钟档）
//	点第 2 档 → 680 载荷第 3 个 u32 = 1 → 应给 10414137（30 分钟档）
//
// ★ 为什么写死：差一在这里是**静默**的 —— 第 1 档照样对，第 2 档会被折回第 1 档、
// 又去领一遍已领过的奖励（无邮件、无报错），客户端表现是"点了没反应"。
func TestTierForClaimMapsRowToTier(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	for row := 0; row < 4; row++ {
		tier, ok := c.TierForClaim(row)
		if !ok || tier.Index != row {
			t.Fatalf("行号 %d 应映射到行首值 %d，得到 %+v ok=%v", row, row, tier, ok)
		}
	}
	// 实机锚点：第 2 档那次的载荷行号是 1，奖励必须是 10414137。
	if tier, ok := c.TierForClaim(1); !ok || tier.Reward.Template != 10414137 {
		t.Fatalf("行号 1 应给 10414137，得到 %+v ok=%v", tier, ok)
	}
	// 越界与负数必须映射不到 —— 不能"就近取一档"。
	for _, row := range []int{-1, 4, 99} {
		if _, ok := c.TierForClaim(row); ok {
			t.Fatalf("行号 %d 不该映射到任何档", row)
		}
	}
	if c.MaxStep() != 4 {
		t.Fatalf("MaxStep = %d，应为 4", c.MaxStep())
	}
}

// TestClaimedMaskOperations 钉住位图语义与拒绝条件。
func TestClaimedMaskOperations(t *testing.T) {
	st := State{Version: 1, EventID: EventID, CycleStart: 1, Day: 1}
	if st.ClaimedCount() != 0 || st.ClaimedTier(0) {
		t.Fatalf("初始应为空: %+v", st)
	}
	// 补领早档也要能记（顺序不保证），位图天然无序。
	for _, idx := range []int{2, 0, 3} {
		next, err := st.Claim(idx)
		if err != nil {
			t.Fatalf("claim %d: %v", idx, err)
		}
		st = next
	}
	if st.ClaimedCount() != 3 {
		t.Fatalf("已领数 = %d，应为 3", st.ClaimedCount())
	}
	for _, idx := range []int{0, 2, 3} {
		if !st.ClaimedTier(idx) {
			t.Fatalf("档 %d 应已领", idx)
		}
	}
	if st.ClaimedTier(1) {
		t.Fatal("档 1 不该已领")
	}
	// 重复领必须被拒（幂等第一道闸）。
	if _, err := st.Claim(2); err == nil {
		t.Fatal("同一档重复领没有被拒")
	}
	// 越界档号必须被拒。
	if _, err := st.Claim(-1); err == nil {
		t.Fatal("负档号没有被拒")
	}
	if _, err := st.Claim(MaxClaimedTiers); err == nil {
		t.Fatal("档号上界没有被拒")
	}
}

// TestNormalizeClearsClaimed 钉住"跨天/换期把已领一起清掉"。
func TestNormalizeClearsClaimed(t *testing.T) {
	st := State{Version: 1, EventID: EventID, CycleStart: 100, Day: 42}
	next, err := st.Claim(1)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	st = next
	// 同一天、同一期：不动。
	if got := st.Normalize(100, 42); got != st {
		t.Fatalf("同天同期不该变: %+v -> %+v", st, got)
	}
	// 跨天：整份换新（含已领）。
	got := st.Normalize(100, 43)
	if got.Claimed != 0 || got.Seconds != 0 || got.Day != 43 {
		t.Fatalf("跨天没有整份重置: %+v", got)
	}
	// 换期：同理。
	if got := st.Normalize(200, 42); got.Claimed != 0 {
		t.Fatalf("换期没有清掉已领: %+v", got)
	}
}

// TestStateIsComparable 是**编译期**约束：`honeyTimeRestore` 里 `if st != stored` 是
// "归一化后真的变了才落库"的判据。谁给 State 加一个 slice/map 字段，这一行就编译不过 ——
// 与其等实现处报错，不如在这里先钉住。
func TestStateIsComparable(t *testing.T) {
	a := State{Version: 1, EventID: EventID, Day: 1, Seconds: 2, Claimed: 1}
	b := a
	if a != b {
		t.Fatal("相同内容的 State 必须相等")
	}
	b.Claimed = 2
	if a == b {
		t.Fatal("不同的 State 必须不相等")
	}
}

// TestParseRejectsBadRows 钉住"宁可拒绝也不猜"：行宽不是 5 的整数倍、数量为 0、
// 行首值越界，都必须报错。
func TestParseRejectsBadRows(t *testing.T) {
	bad := []string{
		// 行宽不是 5 的整数倍
		"[reward info]\n[reward type]\n`daily`\n[reward data]\n0 10414136 1 0 \n[/reward data]\n[/reward info]\n",
		// 数量为 0
		"[reward info]\n[reward type]\n`daily`\n[reward data]\n0 10414136 0 0 600000 \n[/reward data]\n[/reward info]\n",
		// 行首值越界（客户端把它当 u8 存）
		"[reward info]\n[reward type]\n`daily`\n[reward data]\n300 10414136 1 0 600000 \n[/reward data]\n[/reward info]\n",
		// 日档行数与 [max reward offset count] daily 不符
		"[max reward offset count]\n`daily` 4 \n[/max reward offset count]\n" +
			"[reward info]\n[reward type]\n`daily`\n[reward data]\n0 10414136 1 0 600000 \n[/reward data]\n[/reward info]\n",
		// 日档一个 [reward data] 都没有
		"[reward info]\n[reward type]\n`daily`\n[/reward info]\n",
	}
	for i, text := range bad {
		if _, err := Parse(text); err == nil {
			t.Fatalf("第 %d 个坏脚本没有被拒", i)
		}
	}
}

// TestMinutesForTier 钉住邮件文案用的"累计分钟"口径：脚本写的是每档时长，
// 玩家看到的里程碑是累计值（10/30/60/120 分钟）。与 StepNumber 同一套累加，不许另写一份。
func TestMinutesForTier(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := map[int]int{0: 10, 1: 30, 2: 60, 3: 120}
	for index, minutes := range want {
		got, ok := c.MinutesForTier(index)
		if !ok || got != minutes {
			t.Fatalf("第 %d 档累计分钟 = %d(ok=%v)，应为 %d", index, got, ok, minutes)
		}
	}
	if _, ok := c.MinutesForTier(9); ok {
		t.Fatal("不存在的档不该返回 ok")
	}
}

// TestParsePeriodTrack 钉住期间轨（面板下面那行"累计在线时长"）的提取结果。
//
// 本机脚本：5 档、每档 72000000 ms（20 小时）⇒ 累计门槛 20/40/60/80/100 小时，
// 面板文案 "Complete 80h of Honey Time" 正好是第 4 档（索引 3）⇒ 累计 80h = 288000 s。
// 后两行的第 4 个值是 1（疑与 `[pc room event]` 有关，语义未证 ⇒ 只留档、不参与判定）。
func TestParsePeriodTrack(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(c.Period) != 5 {
		t.Fatalf("期间轨应 5 档（[max reward offset count] period 5），得到 %d: %+v", len(c.Period), c.Period)
	}
	wantTemplates := []uint32{10420527, 10421322, 10421302, 10420522, 10420523}
	wantCounts := []uint32{1, 4, 1, 1, 1}
	wantFlags := []uint32{0, 0, 0, 1, 1}
	for i, tier := range c.Period {
		if tier.Index != i || tier.Reward.Template != wantTemplates[i] ||
			tier.Reward.Count != wantCounts[i] || tier.DurationMillis != 72000000 || tier.Flag != wantFlags[i] {
			t.Fatalf("第 %d 档不对: %+v（应 template=%d count=%d flag=%d dur=72000000）",
				i, tier, wantTemplates[i], wantCounts[i], wantFlags[i])
		}
	}
	// 累计门槛：20/40/60/80/100 小时。
	wantSeconds := []uint32{72000, 144000, 216000, 288000, 360000}
	for i, want := range wantSeconds {
		if got := c.PeriodCumulativeSeconds(i); got != want {
			t.Fatalf("期间轨第 %d 档累计秒数 = %d，应为 %d", i, got, want)
		}
	}
	if got, ok := c.PeriodMinutesForTier(3); !ok || got != 4800 {
		t.Fatalf("第 4 档（80h）累计分钟 = %d ok=%v，应为 4800", got, ok)
	}
	if got := c.PeriodCapacity(); got != 360000 {
		t.Fatalf("期间轨封顶 = %d，应为 360000（100 小时）", got)
	}
	// 阶号（1 起）与累计口径：72000 s 恰好停在门槛上 ⇒ 仍是第 0 档（客户端的 `<=` 差一）。
	for _, tc := range []struct {
		seconds uint32
		step    int
	}{{0, 0}, {72000, 0}, {72001, 1}, {288001, 4}} {
		if got := c.PeriodStepNumber(tc.seconds); got != tc.step {
			t.Fatalf("PeriodStepNumber(%d) = %d，应为 %d", tc.seconds, got, tc.step)
		}
	}
	// 行号口径与日档一致：行号 = 行首值（0 起）。
	if tier, ok := c.PeriodTierForClaim(0); !ok || tier.Reward.Template != 10420527 {
		t.Fatalf("期间轨行号 0 应给 10420527，得到 %+v ok=%v", tier, ok)
	}
	if _, ok := c.PeriodTierForClaim(5); ok {
		t.Fatal("期间轨行号 5 越界，不该映射到任何档")
	}
}

// ★ 2026-10-11（吸纳上游证据）：本活动的 `[reward type]` 共四个 —— daily / period /
// specified / extend_daily（上游从 UI 虚拟回调 14412c770 读出四个数组对应 CMD680 的
// action 1/2/3/5，**跳过 4**）。**本季脚本只有前两个** ⇒ Groups 必须是 [daily period]，
// UnmodelledGroups 必须为空。
//
// 将来哪个赛季真用上后两个，这条会红 —— 提醒去按源实现建模，而不是把它们的
// `[reward data]` 静默当成 daily 发奖。
func TestParseRegistersRewardGroups(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析真脚本失败: %v", err)
	}
	if len(c.Groups) != 2 || c.Groups[0] != "daily" || c.Groups[1] != "period" {
		t.Fatalf("Groups = %v，应为 [daily period]（顺序即轨序 / action 序）", c.Groups)
	}
	if len(c.UnmodelledGroups) != 0 {
		t.Fatalf("本季不该出现未建模分组：%v", c.UnmodelledGroups)
	}
}

// 已登记但未建模的分组（specified / extend_daily）必须**认得、不报错、也不当成 daily**：
// 它们的 `[reward data]` 不采，只记进 UnmodelledGroups。
func TestParseKnownButUnmodelledGroupIsRecorded(t *testing.T) {
	text := "[reward info]\n[reward type]\n`specified`\n" +
		"[reward data]\n0 111 1 0 1000 \n[/reward data]\n[/reward info]\n" + scriptFixture
	c, err := Parse(text)
	if err != nil {
		t.Fatalf("已登记的分组不该让整份脚本解析失败: %v", err)
	}
	if len(c.UnmodelledGroups) != 1 || c.UnmodelledGroups[0] != "specified" {
		t.Fatalf("UnmodelledGroups = %v", c.UnmodelledGroups)
	}
	if len(c.Daily) != 4 {
		t.Fatalf("未建模分组的行不该混进日档：%d", len(c.Daily))
	}
}

// 完全没听过的分组名必须报错（不静默吞掉）。
func TestParseRejectsUnknownRewardGroup(t *testing.T) {
	text := "[reward info]\n[reward type]\n`who_knows`\n[/reward info]\n" + scriptFixture
	if _, err := Parse(text); err == nil {
		t.Fatal("未知 [reward type] 必须报错")
	}
}
