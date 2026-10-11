package aradpass

import (
	"os"
	"testing"
)

// 夹具是**本仓 PVF 里当前在期的那份**战令脚本：
// `live/else/univ/2026/0804_2026_aradpass_season3/newaradpassevent.evt`
// （用 `dfo-tool pvfinspect -files <该路径>` 导出，见 runtime/aradpass/）。
//
// ⚠️ 与本功能的**移植来源**（另一棵树，脚本是 `live/event/kor/2026/0326_newaradpass/`）
// 是**不同赛季**：那边的赛季是 2026-03-26 → 06-04、奖励模板 10419765 一族、普通轨 26 档；
// 我们这份是 2026-08-04 → 10-20、模板 590722601 一族、两条轨各 25 档。
// 段名 / 结构 / 等级表 / 任务表 / 经验口径**逐条一致**，只有"赛季数据"不同 ⇒
// 本文件的断言按**我方赛季**写死（换赛季时这里必须跟着改，这正是它的作用）。
func loadFixture(t *testing.T) *Catalog {
	t.Helper()
	raw, err := os.ReadFile("testdata/newaradpassevent.evt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	c, err := Parse(string(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

func TestParseAradPassSourceCatalog(t *testing.T) {
	c := loadFixture(t)

	if c.DBTable != "event_new_arad_pass_2026" || c.CharTable != "event_new_arad_pass_charac_2026" {
		t.Fatalf("db table names: %q / %q", c.DBTable, c.CharTable)
	}
	if c.PeriodStart != "2026-08-04 09:00:00" || c.PeriodEnd != "2026-10-20 09:00:00" {
		t.Fatalf("period: %q..%q", c.PeriodStart, c.PeriodEnd)
	}
	// 源里的经验口径：满级 50000、每周上限 7150、购票赠送 5000。
	if got := c.MaxExp(); got != 50000 {
		t.Fatalf("max exp = %d（源 [pass level info] 最后一级应为 50000）", got)
	}
	if c.WeeklyGainableExp != 7150 {
		t.Fatalf("weekly gainable exp = %d", c.WeeklyGainableExp)
	}
	if c.PurchaseExp != 5000 {
		t.Fatalf("purchase pass exp = %d", c.PurchaseExp)
	}
	if len(c.Levels) != 50 || c.Levels[0] != (Level{Level: 1, Exp: 0}) || c.Levels[1] != (Level{Level: 2, Exp: 600}) {
		t.Fatalf("levels head: %+v（共 %d 条）", c.Levels[:min(3, len(c.Levels))], len(c.Levels))
	}
	// 任务：普通 16 条、赛季 12 条 —— 与 0x08A1 里两条任务列表的条数（16/12）一致。
	if len(c.NormalMissions) != 16 || len(c.SeasonMissions) != 12 {
		t.Fatalf("missions: normal=%d season=%d", len(c.NormalMissions), len(c.SeasonMissions))
	}
	if c.NormalMissions[0] != (Mission{Index: 1, Key: "daily_mission", Count: 4, Exp: 300}) {
		t.Fatalf("normal mission 1: %+v", c.NormalMissions[0])
	}
	if c.SeasonMissions[0] != (Mission{Index: 1, Key: "proper_dungeon_clear", Count: 100, Exp: 800}) {
		t.Fatalf("season mission 1: %+v", c.SeasonMissions[0])
	}
	// 奖励表：我方赛季两条轨**各 25 档**（普通 1,3,…,49 + 皇家 2,4,…,50）。
	normal, royal := c.RewardsFor(0), c.RewardsFor(2)
	if len(normal) != 25 || len(royal) != 25 {
		t.Fatalf("rewards: normal=%d royal=%d（共 %d）", len(normal), len(royal), len(c.Rewards))
	}
	if normal[0] != (Reward{Track: 0, Slot: 1, Item: 590722601, Count: 1, Level: 1, Title: "AradPass_lettertitle_000"}) {
		t.Fatalf("normal reward 1: %+v", normal[0])
	}
	if normal[24].Item != 590722645 || normal[24].Level != 49 {
		t.Fatalf("normal reward 25: %+v", normal[24])
	}
	if royal[0].Item != 590722602 || royal[0].Level != 2 {
		t.Fatalf("royal reward 1: %+v", royal[0])
	}
	if royal[24].Item != 590722646 || royal[24].Level != 50 {
		t.Fatalf("royal reward 25: %+v", royal[24])
	}
	// 四列一行（`<物品> <数量> <标题> <文案>`）的段**只取物品号**：数量列（=1）不能当物品。
	// 实机 2026-10-04 就因为取"所有数字"多发了几枚金币（模板 1 = 金币）。
	if len(c.RoyalFirstReward) != 4 || c.RoyalFirstReward[0] != 590722598 || c.RoyalFirstReward[3] != 590722594 {
		t.Fatalf("royal first reward = %v（应为 4 件、首个 590722598）", c.RoyalFirstReward)
	}
	if len(c.DailyReward) != 1 || c.DailyReward[0] != 590722589 {
		t.Fatalf("daily reward list = %v（应为 [590722589]）", c.DailyReward)
	}
	if len(c.SeasonReward) != 1 || c.SeasonReward[0] != 590722591 {
		t.Fatalf("season reward list = %v（应为 [590722591]）", c.SeasonReward)
	}
	// 通行证本体（"拥有即激活"靠它识别）。
	if len(c.PassItem) != 1 || c.PassItem[0] != 590722579 {
		t.Fatalf("pass item index = %v（应为 [590722579]）", c.PassItem)
	}
}

// 等级换算必须与抓包实证逐条吻合（18 份 0x08A1 里出现的 (level, exp) 组合）。
func TestLevelForMatchesCaptureSamples(t *testing.T) {
	c := loadFixture(t)
	for _, tc := range []struct {
		exp   uint32
		level uint32
	}{
		{0, 1}, {300, 1}, {600, 2}, {700, 2},
		{1200, 3}, {50000, 50},
	} {
		if got := c.LevelFor(tc.exp); got != tc.level {
			t.Fatalf("LevelFor(%d) = %d，抓包/源表应为 %d", tc.exp, got, tc.level)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestParseAradPassNormalDungeons 钉住 `[normal dungeon index list]` 的解析
// （普通轨任务 `normal_dungeon_2` 只数这些图）。
//
// 源里那一行是 `` `normal_dungeon_2` 100004140 … `` —— 首 token 是**键名**不是数字，
// `uints()` 会自然跳过它；断言里显式写出来，免得将来有人"顺手"把键名也当图号放进去。
func TestParseAradPassNormalDungeons(t *testing.T) {
	c := loadFixture(t)
	if len(c.NormalDungeons) != 14 {
		t.Fatalf("normal dungeon index list = %d 条，应为 14: %v", len(c.NormalDungeons), c.NormalDungeons)
	}
	if c.NormalDungeons[0] != 100004140 || !c.IsNormalDungeon(100005065) || c.IsNormalDungeon(999) {
		t.Fatalf("normal dungeon 判定不对: %v", c.NormalDungeons)
	}
}

