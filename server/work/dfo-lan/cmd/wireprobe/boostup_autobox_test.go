package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

// autoBoxCatalog 用生产同一个 boosterBoxSource 适配器（loot.RewardBoxSource）：
// 活动自动开盒与掉落线共用「[booster] 开一层出什么」的同一份解释，测试不另建
// 礼包目录视图（§0.2 单一规则）。内容按源的三层结构手搭，只用于隔离存储夹具。
func autoBoxCatalog() *BoosterCatalog {
	return &BoosterCatalog{
		Definitions: map[uint32]BoosterDefinition{
			9: {Template: 9, Type: "[booster]", Pools: []BoosterRewardPool{{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 20002, Weight: 1000, Count: 1}}}}},
			11: {Template: 11, Type: "[booster]", Pools: []BoosterRewardPool{
				{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 590015966, Weight: 1000, Count: 1000}}},
				{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 590015964, Weight: 1000, Count: 1}}},
				{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 3037, Weight: 1000, Count: 1000}}},
			}},
		},
		Items: map[uint32]catalog.ItemIndexEntry{
			20002:     {ID: 20002, Kind: "equipment", Path: "equipment/test.stk"},
			590015966: {ID: 590015966, Kind: "stackable", StackableType: "[material]", StackLimit: 10000},
			590015964: {ID: 590015964, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000},
			3037:      {ID: 3037, Kind: "stackable", StackableType: "[material]", StackLimit: 10000},
		},
	}
}

func assertBoostAutoSQL(t *testing.T, ctx context.Context, store *database.TestFixture) {
	t.Helper()
	must115(t, store.MigrateAccountMaterials(ctx))
	must115(t, store.MigrateMailbox(ctx))
	account, e := store.DevelopmentAccount(ctx, "boost-auto-fixture")
	must115(t, e)
	prof, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	must115(t, e)
	eq, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", prof.Source.Checksum)
	must115(t, e)
	rules := inventory.BagRules{Source: prof.Source.Checksum, EquipmentSlots: [2]uint16{9, 9}, Slots: map[string][2]uint16{"[material]": {121, 121}, "[booster]": {65, 65}}, MissingStackLimit: 10000}
	// donor 基线的 loot.Service.AccountMaterials 开关位在本树不存在：
	// 本树自动开盒的共享材料路径直接经 Store 落账，无需开关。
	ls := &loot.Service{Equipment: eq, BagRules: rules, Catalog: catalog.LootCatalog{Source: prof.Source, Items: map[uint32]catalog.LootItem{
		590015966: {ID: 590015966, Script: catalog.ScriptRecord{Path: "stackable/test-token.stk"}, Kind: "stackable", StackableType: "[material]", StackLimit: 10000}, 590015964: {ID: 590015964, Script: catalog.ScriptRecord{Path: "stackable/test-box.stk"}, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000}, 3037: {ID: 3037, Kind: "stackable", StackableType: "[material]", StackLimit: 10000},
	}}}
	ls.RewardBoxes = boosterBoxSource{catalog: autoBoxCatalog()}
	c := &boostup.Catalog{Town: 222}
	for i := byte(1); i <= 11; i++ {
		c.Steps = append(c.Steps, boostup.Step{Number: i, Area: i - 1, Guide: "normal", Mission: "none"})
	}
	c.Steps[8].AutoOpen = true
	c.Steps[8].Mission = "disjoint"
	c.Steps[8].Rewards = []boostup.Reward{{Item: 9, Count: 1}}
	c.Steps[10].AutoOpen = true
	c.Steps[10].Rewards = []boostup.Reward{{Item: 11, Count: 1}}
	create := func(name string, step byte, bag inventory.Bag) database.Character {
		raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"unknown":42}`), bag)
		must115(t, err)
		raw, err = boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: step, Phase: 1}})
		must115(t, err)
		r, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, Request: []byte{0}, ConfigVersion: prof.Source.Checksum, State: raw}, 100)
		must115(t, err)
		return r
	}
	request := func(step byte) []byte {
		p := make([]byte, 8)
		binary.LittleEndian.PutUint32(p, 662)
		binary.LittleEndian.PutUint32(p[4:], uint32(step))
		return p
	}
	r := create("boost-auto-nine", 9, inventory.Bag{Version: "ordinary-bag-v1"})
	w := &worldSession{account: account, role: r, loot: ls, store: store.Storage(), boostup: c, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 8}}}
	plan, e := w.boostStepRequest(ctx, request(9), 680)
	must115(t, e)
	if plan[0].ID != 14 || binary.LittleEndian.Uint32(plan[0].Payload[5:]) != 20002 {
		t.Fatal("auto gear encoded as an empty stack slot", plan)
	}
	bag, e := inventory.ReadBag(w.role.State)
	must115(t, e)
	if len(bag.Equipment) != 1 || len(bag.Items) != 0 {
		t.Fatal("outer box granted instead of equipment")
	}
	_, e = w.boostStepRequest(ctx, request(9), 680)
	must115(t, e)
	bag, e = inventory.ReadBag(w.role.State)
	must115(t, e)
	if len(bag.Equipment) != 1 {
		t.Fatal("auto gear duplicated")
	}
	// 装备槽 9 已被占用：本树的自动开盒与 PrepareBoostGift 同一条约定 —— 装不下
	// 就整步回滚、不发邮寄（donor 基线的邮寄兜底不在本树，见
	// internal/loot/boostup_autobox.go）。回滚既不能消耗领奖标记，也不能留下事件行。
	full := create("boost-auto-full", 9, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}}})
	w.role = full
	if _, e = w.boostStepRequest(ctx, request(9), 680); e == nil {
		t.Fatal("full gear bag accepted")
	}
	messages, e := store.Mailbox(ctx, account, full.ID)
	must115(t, e)
	if len(messages) != 0 {
		t.Fatal("full bag fell back to mail", messages)
	}
	var events int
	{
		v, e := store.EventCount(ctx, full.ID)
		must115(t, e)
		events = int(v)
	}
	if events != 0 {
		t.Fatal("rolled-back step left an event row")
	}
	// 清包后重放同一步：发放正常发生且只有一份。
	cleared := create("boost-auto-retry", 9, inventory.Bag{Version: "ordinary-bag-v1"})
	w.role = cleared
	if _, e = w.boostStepRequest(ctx, request(9), 680); e != nil {
		t.Fatal(e)
	}
	bag, e = inventory.ReadBag(w.role.State)
	must115(t, e)
	if len(bag.Equipment) != 1 {
		t.Fatal("cleared bag retry did not grant exactly once", bag.Equipment)
	}
	r = create("boost-auto-final", 11, inventory.Bag{Version: "ordinary-bag-v1"})
	w.role = r
	w.state.Position.Area = 10
	plan, e = w.boostStepRequest(ctx, request(11), 680)
	must115(t, e)
	bag, e = inventory.ReadBag(w.role.State)
	must115(t, e)
	if len(bag.Items) != 2 {
		t.Fatal("final outer box or cube was kept in normal bag", bag.Items)
	}
	for _, i := range bag.Items {
		if i.Template == 3037 || i.Template == 11 {
			t.Fatal("wrong final inventory container")
		}
	}
	counts, e := store.AccountMaterials(ctx, account)
	must115(t, e)
	m, e := inventory.ReadAccountMaterials(counts)
	must115(t, e)
	if m.Count(3037) != 1000 {
		t.Fatal("missing shared cube grant")
	}
	state, e := boostup.ReadState(w.role.State)
	must115(t, e)
	if !state.Training.Finished || state.Training.Step != 12 {
		t.Fatal("graduation not committed with final rewards")
	}
	shared, done := false, false
	for _, p := range plan {
		if p.ID == 13 && len(p.Payload) > 0 && p.Payload[0] == 35 {
			shared = true
		}
		if p.ID == 2638 && p.Payload[0] == 2 && p.Payload[1] == 12 {
			done = true
		}
	}
	if !shared || !done || plan[len(plan)-1].ID != 680 {
		t.Fatal("final shared/state/ACK order")
	}
	_, e = w.boostStepRequest(ctx, request(11), 680)
	must115(t, e)
	counts, e = store.AccountMaterials(ctx, account)
	must115(t, e)
	m, e = inventory.ReadAccountMaterials(counts)
	must115(t, e)
	if m.Count(3037) != 1000 {
		t.Fatal("shared reward duplicated")
	}
	// A late shared-ledger failure rolls back ordinary rewards and graduation.
	// donor 基线的 CommitAccountMaterialEvent 回调可同事务改写角色状态；本树回调
	// 只产（材料正文, 回执），此处 fixture 本就不动角色（返回 cur.State 原样）。
	overflowRole := create("boost-auto-overflow", 11, inventory.Bag{Version: "ordinary-bag-v1"})
	_, _, _, e = store.CommitAccountMaterialEvent(ctx, account, overflowRole.ID, prof.Source.Checksum, "fixture-fill-shared", "fixture-v1", func(cur database.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		m, err := inventory.ReadAccountMaterials(raw)
		if err != nil {
			return nil, nil, err
		}
		m, _, err = m.Add(3037, ^uint32(0)-m.Count(3037))
		if err != nil {
			return nil, nil, err
		}
		next, err := m.Save()
		return next, json.RawMessage(`{}`), err
	})
	must115(t, e)
	w.role = overflowRole
	if _, e = w.boostStepRequest(ctx, request(11), 680); e == nil {
		t.Fatal("shared overflow accepted")
	}
	roles, e := store.Characters(ctx, account)
	must115(t, e)
	for _, cur := range roles {
		if cur.ID == overflowRole.ID {
			b, err := inventory.ReadBag(cur.State)
			must115(t, err)
			st, err := boostup.ReadState(cur.State)
			must115(t, err)
			if len(b.Items) != 0 || st.Training.Finished || st.Training.Claimed[11] {
				t.Fatal("failed final step left assets/graduation")
			}
		}
	}
}

// TestBoostAutoGrantsActualSource 用当前内层归档回答「活动礼盒到底开出什么」：
// 解析、展开与发放全走生产实现（catalog.ImportBoosters + boosterBoxSource +
// loot.BoostAutoGrants），不引入测试专用的第二份礼包目录视图。
// 没设 DFO_PVF_CORE_TEST_ARCHIVE 时跳过；设了就必须通过。
func TestBoostAutoGrantsActualSource(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the Starter Boost auto-box source check")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	must115(t, e)
	defer a.Close()
	index, e := catalog.ImportItemIndex(a)
	must115(t, e)
	defs, e := catalog.ImportBoosters(a, index)
	must115(t, e)
	s := &loot.Service{RewardBoxes: boosterBoxSource{catalog: &BoosterCatalog{Definitions: defs, Items: index.Items}}}
	role := loot.Role{ID: 1, AccountID: 1}

	// 第九关的两个外层盒各开一层都是一件装备（donor 基线实机确认过的内容）。
	for _, v := range []struct{ box, want uint32 }{{590015951, 100051285}, {590015952, 100051289}} {
		got, e := s.BoostAutoGrants(role, 9, []boostup.Reward{{Item: v.box, Count: 1}})
		must115(t, e)
		if len(got) != 1 || got[0].Template != v.want || got[0].Amount != 1 {
			t.Fatalf("step9 box %d: %+v", v.box, got)
		}
	}
	// 第十一关：材料 + **未开封的选择箱** + 账号共享材料。递归展开选择箱等于
	// 替玩家做选择，发下去的就不是源里那份奖励了。
	got, e := s.BoostAutoGrants(role, 11, []boostup.Reward{{Item: 590015965, Count: 1}})
	must115(t, e)
	amounts := map[uint32]uint32{}
	for _, g := range got {
		amounts[g.Template] += g.Amount
	}
	if len(got) != 3 || amounts[590015966] != 1000 || amounts[590015964] != 1 || amounts[3037] != 1000 {
		t.Fatalf("step11 box: %+v", got)
	}
}
