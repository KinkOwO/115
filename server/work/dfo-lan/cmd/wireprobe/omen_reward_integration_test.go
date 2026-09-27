package main

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// omenTestEnv 把 omen 集成测试需要的东西一次装好。它与
// endkeeper_reward_integration_test.go 共用同一套源目录，所以两处的加载点
// 是同一份「现实」，不会各自漂移。
type omenTestEnv struct {
	a     *loot.AttunementRewards
	boxes boosterBoxSource
	dc    catalog.DungeonCatalog
	lc    catalog.LootCatalog
	gear  *inventory.EquipmentCatalog
}

func loadOmenTestEnv(t *testing.T) omenTestEnv {
	t.Helper()
	if os.Getenv("ATTUNEMENT_REWARD_INTEGRATION") != "1" {
		t.Skip("set ATTUNEMENT_REWARD_INTEGRATION=1 to load the 295 MB dungeon catalog")
	}
	dc, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	lc, err := catalog.LoadLoot("../../configs/loot.level150.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", lc.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	a, err := loot.LoadAttunementRewards("../../configs/attunement-rewards.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ValidateOmen(); err != nil {
		t.Fatalf("the shipped table fails the omen check: %v", err)
	}
	bc, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Definitions) == 0 || len(bc.Items) == 0 {
		t.Fatalf("booster catalog came back empty (%d definitions, %d items)", len(bc.Definitions), len(bc.Items))
	}
	return omenTestEnv{a: a, boxes: boosterBoxSource{catalog: bc}, dc: dc, lc: lc, gear: gear}
}

// TestOmenStageIDsMatchTheOfficialBoxes 把「每档的奖励预览模板」钉在官方奖励表的
// 主奖励盒上（docs §38.2 的逐位对位）：神器 10416150 / 传说 10417545 / 史诗 10417552 /
// 太初 10417571。行 0 没有条目，所以是 0。
//
// 这四个 id 会进 noti 2836 的「征兆 ID 数组」，客户端拿它们做奖励预览 —— 值错了玩家
// 会看到「这个征兆会给别的东西」，而掉落本身还是对的，所以只有对表才能发现。
func TestOmenStageIDsMatchTheOfficialBoxes(t *testing.T) {
	env := loadOmenTestEnv(t)
	got := env.a.OmenStageIDs(endkeeperDungeon)
	want := []uint32{0, 10416150, 10417545, 10417552, 10417571}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("omen stage ids = %v, want %v", got, want)
	}
}

// TestOmenStagesPayTheirOwnTier 用真实目录证明四阶段各自发自己那一档：
// 每个阶段的内容集两两不相交，而实际结算只可能落在自己那一格里。
//
// 这是「掉落物准确」的核心判据。它不硬编码任何一个物品 id —— 四份内容集都
// 从表本身派生，所以源改版只会让它重新派生，不会变成同义反复。
func TestOmenStagesPayTheirOwnTier(t *testing.T) {
	env := loadOmenTestEnv(t)
	a := env.a
	stages := a.OmenStages(endkeeperDungeon)
	if len(stages) != 5 {
		t.Fatalf("omen stages = %d, want 5", len(stages))
	}
	contents := make([]map[uint32]bool, len(stages))
	for stage := range stages {
		wrappers := a.OmenStageTemplates(endkeeperDungeon, uint32(stage))
		contents[stage] = boxContents(env.boxes, wrappers)
	}
	// 四个阶段的产出必须分属四档、互不重叠；重叠就说明行与档位的对应读错了。
	for i := 1; i < len(stages); i++ {
		for j := i + 1; j < len(stages); j++ {
			for id := range contents[i] {
				if contents[j][id] {
					t.Fatalf("stage %d and stage %d can both pay %d: the rows do not line up with the four tiers", i, j, id)
				}
			}
		}
	}
	if len(contents[4]) == 0 {
		t.Fatal("the guarantee stage (4) can pay nothing")
	}
	// 第 4 阶段是保底：它的内容必须包含装备 —— 官方说这一格给太初星蕴石。
	gearAtFour := 0
	for id := range contents[4] {
		if it, ok := env.boxes.catalog.Items[id]; ok && it.Kind == "equipment" {
			gearAtFour++
		}
	}
	if gearAtFour == 0 {
		t.Fatalf("the guarantee stage reaches no equipment: %v", countByTemplate(map[uint32]int{1: len(contents[4])}))
	}

	const runs = 400
	total := map[uint32]int{}
	for stage := 1; stage < len(stages); stage++ {
		paid, empty := 0, 0
		for i := 0; i < runs; i++ {
			out, err := a.AdvanceOmen(uint32(i)*2246822519+1, endkeeperDungeon, uint32(stage))
			if err != nil {
				t.Fatalf("stage %d advance: %v", stage, err)
			}
			if !out.Paid {
				continue
			}
			paid++
			products, _, _ := loot.OpenRewardBoxes(out.Seed, env.boxes, out.Awards)
			if len(products) == 0 {
				empty++
			}
			for _, p := range products {
				if !contents[stage][p.Template] {
					t.Fatalf("stage %d paid %d, which is not one of its own contents %v",
						stage, p.Template, countByTemplate(map[uint32]int{1: len(contents[stage])}))
				}
				total[p.Template] += int(p.Amount)
			}
		}
		if paid == 0 {
			t.Fatalf("stage %d never settled in %d clears", stage, runs)
		}
		// 该行主产出的权重都在 97% 以上，所以「一次都没发出东西」只能是
		// 展开链断了；空结果本身合法（自选箱要玩家自己选，服务端代不了）。
		if empty == paid {
			t.Fatalf("stage %d settled %d time(s) and never put anything on the ground", stage, paid)
		}
	}
	if len(total) == 0 {
		t.Fatal("no omen stage ever paid a usable product")
	}
	t.Logf("omen stages 1..4 paid %d row(s) over %d template(s) %v",
		sumCounts(total), len(total), countByTemplate(total))
}

func sumCounts(m map[uint32]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// TestEndkeeperClearPaysTheOmenGuarantee 走一遍真实链路：把玩家的征兆账
// 直接放到第 4 阶段（诊断入口），打死本副本的源领主，地面产物里必须出现
// 太初那一档的装备。
//
// 判据是**派生**的：太初档的内容集来自第 4 行自己的 drop list，不是写死的
// 物品 id；同时用通用掉落池反证 —— 那些装备池子给不出来，所以它们只可能
// 来自征兆，而不可能来自杂兵/通用奖励。
func TestEndkeeperClearPaysTheOmenGuarantee(t *testing.T) {
	env := loadOmenTestEnv(t)
	a := env.a
	rules, err := loot.LoadRules("../../configs/drop.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loot.Parse(env.lc)
	if err != nil {
		t.Fatal(err)
	}
	guarantee := boxContents(env.boxes, a.OmenStageTemplates(endkeeperDungeon, 4))
	fingerprints := map[uint32]bool{}
	for id := range guarantee {
		it, ok := env.boxes.catalog.Items[id]
		if !ok || it.Kind != "equipment" {
			continue
		}
		if _, e := env.gear.Basic(id); e != nil {
			fingerprints[id] = true
		}
	}
	if len(fingerprints) == 0 {
		t.Fatal("the guarantee's contents are all reachable from the generic pool: this test would prove nothing")
	}

	ledger := loot.NewOmenLedger(a)
	if !ledger.Enabled() {
		t.Fatal("omen ledger reports itself disabled with the shipped table")
	}
	const character = int64(11)
	ledger.Set(character, 4)

	const runs = 12
	paidTemplates := map[uint32]int{}
	for i := 0; i < runs; i++ {
		// 每轮都从满阶段开始：上一轮结算后账本会归零，这是设计行为。
		ledger.Set(character, 4)
		s, err := dungeon.Select(env.dc, protocol.DungeonSelection{ID: endkeeperDungeon, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		boss := s.Definition.SourceBoss
		if boss == 0 {
			t.Fatalf("dungeon %d lost its source boss", endkeeperDungeon)
		}
		l := loot.NewSession(env.lc, tables, rules, env.gear, s.RunID, 1, character, uint16(character))
		l.Attunement = a
		l.RewardBoxes = env.boxes
		l.Omen = ledger
		var paid bool
		for _, m := range s.Monsters {
			if m.Template != boss {
				continue
			}
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			rows, e := l.Death(s, m.Entity)
			if e != nil {
				t.Fatalf("run %d: the boss paid an error: %v", i, e)
			}
			for _, r := range rows {
				if id := binary.LittleEndian.Uint32(r.Item[2:6]); fingerprints[id] {
					paid = true
					paidTemplates[id]++
				}
			}
		}
		if !paid {
			t.Fatalf("run %d: a full omen run did not drop the guaranteed tier", i)
		}
		if held := ledger.Held(character); held != 0 {
			t.Fatalf("run %d: settling the guarantee left the ledger at %d, want 0", i, held)
		}
		out, ok := ledger.Last(character)
		if !ok || !out.Paid || out.Stage != 4 {
			t.Fatalf("run %d: ledger last = %+v ok=%v", i, out, ok)
		}
	}
	t.Logf("the guarantee paid in %d/%d clears over %d reachable template(s): %v",
		runs, runs, len(fingerprints), countByTemplate(paidTemplates))
}
