package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

type boostTestGroups map[uint32][]int32

func (g boostTestGroups) ForTemplate(id uint32) []int32 { return g[id] }

// Called inside the existing random isolated schema, not a new storage shape.
func assertBoostWearSQL(t *testing.T, ctx context.Context, store *database.TestFixture, account int64) {
	t.Helper()
	prof, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	must115(t, e)
	eq, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", prof.Source.Checksum)
	must115(t, e)
	rules, e := inventory.LoadWearRules("../../configs/equipment-wear.current35.json", prof.Source.Checksum)
	must115(t, e)
	bagRules, e := inventory.LoadBagRules("../../configs/inventory.next29.json")
	must115(t, e)
	ws := &workflow.WearService{Store: store.Storage(), WearService: inventory.WearService{Catalog: eq, Professions: prof, Rules: rules, BagRules: bagRules}}
	b := inventory.Bag{Version: "ordinary-bag-v1", Gold: 345, Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}, {Slot: 10, Template: 24002}}}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":6,"advancement":0,"unrelated":42}`), b)
	must115(t, e)
	raw, e = boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 2, Claimed: map[byte]bool{1: true}}})
	must115(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "boostwear", Profession: 0, Request: []byte{0}, ConfigVersion: prof.Source.SaveIdentity(), State: raw}, 100)
	must115(t, e)
	c := &boostup.Catalog{Town: 222, Groups: boostTestGroups{20002: {77, 161}}, Steps: []boostup.Step{
		{Number: 1, Mission: "equip item", MissionCells: []pvf.Token{{Type: 3, Text: "[equip grouping]"}, {Type: 3, Text: "[count]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[index]"}, {Type: 0, Value: 77}, {Type: 3, Text: "[index]"}, {Type: 0, Value: 161}, {Type: 3, Text: "[/equip grouping]"}}},
		{Number: 2, Guide: "normal", Mission: "none"},
	}}
	ls := &loot.Service{Equipment: eq, Catalog: catalog.LootCatalog{Source: prof.Source}}
	w := &worldSession{account: account, role: role, loot: ls, store: store.Storage(), boostup: c}
	if p := w.reconcileBoostEquipment(); len(p) != 0 {
		t.Fatal("bag-only item counted as worn")
	}
	var n int
	{
		v, e := store.EventCount(ctx, role.ID)
		must115(t, e)
		n = int(v)
	}
	if n != 0 {
		t.Fatal("unsatisfied mission consumed receipt")
	}
	p := make([]byte, 32)
	binary.LittleEndian.PutUint16(p[1:], 9)
	binary.LittleEndian.PutUint32(p[3:], 20002)
	binary.LittleEndian.PutUint32(p[7:], 1)
	p[11] = 3
	binary.LittleEndian.PutUint16(p[12:], 19)
	binary.LittleEndian.PutUint32(p[22:], ^uint32(0))
	var move equipmentSession
	plan, e := move.handle(ws, w, p, append([]byte("owned-frame"), p...))
	must115(t, e)
	if len(plan) < 2 || plan[0].ID != 19 || plan[len(plan)-1].ID != 2638 || plan[len(plan)-1].Payload[1] != 2 {
		t.Fatal("real move did not publish mission progression", plan)
	}
	state, e := boostup.ReadState(w.role.State)
	must115(t, e)
	if state.Training.Step != 2 || !state.Training.Claimed[1] {
		t.Fatal(state)
	}
	if p := w.reconcileBoostEquipment(); len(p) != 0 {
		t.Fatal("duplicate advance")
	}
	{
		v, e := store.EventCountByKey(ctx, role.ID, "boostup-mission:1")
		must115(t, e)
		n = int(v)
	}
	if n != 1 {
		t.Fatal("receipt missing/duplicated", n)
	}
	bag, e := inventory.ReadBag(w.role.State)
	must115(t, e)
	if bag.Gold != 345 || len(bag.Worn) != 1 || bag.Worn[0].Template != 20002 {
		t.Fatal("mission changed assets")
	}
	// A failed source proof keeps the successful equipment response, and no
	// mission receipt is created. Entry/query can retry once its source exists.
	badRaw, e := boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 2, Claimed: map[byte]bool{1: true}}})
	must115(t, e)
	bad, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "boostwearretry", Profession: 0, Request: []byte{0}, ConfigVersion: prof.Source.SaveIdentity(), State: badRaw}, 100)
	must115(t, e)
	cMissing := *c
	cMissing.Groups = nil
	w.role = bad
	w.boostup = &cMissing
	plan, e = move.handle(ws, w, p, append([]byte("second-owned-frame"), p...))
	must115(t, e)
	if len(plan) == 0 || plan[0].ID != 19 || plan[len(plan)-1].ID == 2638 {
		t.Fatal("event failure swallowed committed move")
	}
	w.boostup = c
	if retry := w.reconcileBoostEquipment(); len(retry) != 1 || retry[0].ID != 2638 {
		t.Fatal("durable worn proof cannot recover", retry)
	}
	// Point mission's four numbers are PREMIUM identities, not item groups.
	// 套装积分不再由活动自带第二张表（§0.2 单一规则）：真源是名望侧的
	// character.BoostWornSetPoints，测试用同一 provider 签名注入积分。
	pointCatalog := &boostup.Catalog{Town: 222, Groups: c.Groups, Steps: []boostup.Step{
		{Number: 1, Mission: "partset point only equip item and contract", MissionCells: []pvf.Token{
			{Type: 3, Text: "[condition]"}, {Type: 0, Value: 22}, {Type: 0, Value: 27}, {Type: 0, Value: 79}, {Type: 0, Value: 92}, {Type: 3, Text: "[/condition]"},
			{Type: 3, Text: "[condition2]"}, {Type: 0, Value: 1265}, {Type: 3, Text: "[/condition2]"},
		}}, {Number: 2, Guide: "normal", Mission: "none"},
	}}
	pointRaw, e := boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 2, Claimed: map[byte]bool{1: true}}})
	must115(t, e)
	pointRole, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "boostsetpoint", Profession: 0, Request: []byte{0}, ConfigVersion: prof.Source.SaveIdentity(), State: pointRaw}, 100)
	must115(t, e)
	w.role = pointRole
	pointProvider := func(database.Character) (map[int32]uint32, error) {
		return map[int32]uint32{16204: 1265}, nil
	}
	pointAdvance := func() (sent int) {
		t.Helper()
		next, applied, err := (&workflow.LootService{Store: store.Storage(), Loot: ls}).ReconcileBoostEquipment(ctx, w.role, pointCatalog, pointProvider)
		must115(t, err)
		if applied {
			w.role = next
			sent = 1
		}
		return sent
	}
	if p := pointAdvance(); p != 0 {
		t.Fatal("points alone skipped four contracts")
	}
	now := time.Now()
	for _, typ := range []int{22, 27, 79, 92} {
		end := now.Add(time.Hour).Unix()
		if typ == 92 {
			end = now.Add(-time.Minute).Unix()
		}
		must115(t, store.SeedAccountPremium(ctx, account, typ, end))
	}
	if p := pointAdvance(); p != 0 {
		t.Fatal("expired cube contract accepted")
	}
	{
		v, e := store.EventCount(ctx, pointRole.ID)
		must115(t, e)
		n = int(v)
	}
	if n != 0 {
		t.Fatal("failed contract proof left completion receipt")
	}
	must115(t, store.SetAccountPremiumEnd(ctx, account, 92, now.Add(time.Hour).Unix()))
	if p := pointAdvance(); p != 1 {
		t.Fatal("source points plus four active contracts did not advance", p)
	}
	// 2638 由网关侧 reconcileBoostEquipment 补发；provider 桩走 workflow 同一条事务，
	// 这里断言它据以发帧的事实：积分加四张合约把训练推进到第二步。
	pointState, e := boostup.ReadState(w.role.State)
	must115(t, e)
	if pointState.Training.Step != 2 {
		t.Fatal("point mission advanced without stepping", pointState.Training)
	}
}

// 用改变源任务/步骤和领取事实验证分派，不把旧样本 panel=36 当规则。
func TestBoostJournalWindowRoutesToTransform(t *testing.T) {
	inTrack, e := boostup.WriteState(json.RawMessage(`{"level":115}`),
		boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 2, Claimed: map[byte]bool{1: true}}})
	must115(t, e)
	cat := &boostup.Catalog{Steps: []boostup.Step{{Number: 1, Mission: "transform equip journal or equip item"}}}
	w := &worldSession{boostup: cat, role: database.Character{ID: 1, State: inTrack}}
	win := protocol.EquipmentCraftRequest{Action: 0}
	for _, header := range []uint32{36, 46, 164, 0, ^uint32(0)} {
		win.Panel, win.Context = header, header*17
		if !w.boostJournalSwap(win) {
			t.Fatalf("已领取的源变换任务被请求头 %d 阻断", header)
		}
	}
	cat.Steps[0].Mission = "equip item"
	if w.boostJournalSwap(win) {
		t.Fatal("改变源任务后仍被当成变换")
	}
	cat.Steps[0].Mission = "transform equip journal or equip item"
	for _, training := range []boostup.Training{
		{Step: 1, Phase: 0}, {Step: 1, Phase: 1}, {Step: 1, Phase: 2},
		{Step: 2, Phase: 2, Claimed: map[byte]bool{2: true}},
		{Step: 1, Phase: 2, Finished: true, Claimed: map[byte]bool{1: true}},
	} {
		w.role.State, e = boostup.WriteState(json.RawMessage(`{"level":115}`), boostup.State{Version: 1, Activated: true, Training: training})
		must115(t, e)
		if w.boostJournalSwap(win) {
			t.Fatalf("未领取/非任务期不应改派：%+v", training)
		}
	}
	w.role.State, e = boostup.WriteState(json.RawMessage(`{"level":115}`), boostup.State{Version: 1, Training: boostup.Training{Step: 1, Phase: 2, Claimed: map[byte]bool{1: true}}})
	must115(t, e)
	if w.boostJournalSwap(win) {
		t.Fatal("未激活角色不应改派")
	}
	w.role.State = inTrack
	w.boostup = nil
	if w.boostJournalSwap(win) {
		t.Fatal("活动关闭时不应改派")
	}
}

// 原生实机向量：034251_994078_next37/events.jsonl:966。直接验证网关选择了
// 变换分支；observe 仅使测试不写库，生产仍走同一分支的现有原子变换事务。
func TestBoostJournalCapturedConfirmRoutesToTransform(t *testing.T) {
	p, e := hex.DecodeString("2e00000000000000f0d7830000010000002e0000ffffffff2e0000ffffffff030e0059a9f6052e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff00010000000000000000000000")
	must115(t, e)
	r, e := protocol.DecodeEquipmentCraftRequest(p)
	must115(t, e)
	slots, templates := r.Wanted()
	if r.Panel != 46 || r.Action != 0 || len(slots) != 1 || slots[0] != 14 || templates[0] != 100051289 {
		t.Fatal("实机向量解析不符", r, slots, templates)
	}
	raw, e := boostup.WriteState(json.RawMessage(`{"level":115}`), boostup.State{Version: 1, Activated: true,
		Training: boostup.Training{Step: 10, Phase: 2, Claimed: map[byte]bool{10: true}}})
	must115(t, e)
	cat := &boostup.Catalog{Steps: make([]boostup.Step, 10)}
	cat.Steps[9] = boostup.Step{Number: 10, Mission: "transform equip journal or equip item"}
	w := &worldSession{boostup: cat, role: database.Character{ID: 3, State: raw}}
	before := equipmentTransformApply
	equipmentTransformApply = "observe"
	t.Cleanup(func() { equipmentTransformApply = before })
	var kind string
	plan, e := w.equipmentCraft(p, func(event map[string]any) { kind, _ = event["kind"].(string) })
	must115(t, e)
	if kind != "equipment_craft_transform_planned" || len(plan) != 1 || plan[0].ID != protocol.EquipmentCraftOpcode {
		t.Fatal("确认请求仍走生成分支", kind, plan)
	}
}
