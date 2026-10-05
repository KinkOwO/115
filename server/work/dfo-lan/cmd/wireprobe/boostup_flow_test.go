package main

import (
	"bytes"
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBoostClaimFramesAreObserved 钉住 662 的三条入站线路都在采样白名单里：
// 漏登记的通知第 BodySampleLimit(8) 次之后不再记录正文，诊断会静默变弱。
func TestBoostClaimFramesAreObserved(t *testing.T) {
	for _, id := range []uint16{643, 680, 681} {
		if !observedGameRequest(id) {
			t.Errorf("CMD%d 不在 observedGameRequest 里", id)
		}
	}
}

func TestBoostGiftEntryOrderingAndPerRoleRestore(t *testing.T) {
	c := &boostup.Catalog{Gifts: []boostup.Gift{{ID: 117}, {ID: 118}}}
	role := database.Character{State: json.RawMessage(`{}`)}
	gifts, e := boostGiftAvailability(c, role)
	must115(t, e)
	if !bytes.Equal(gifts, []byte{2, 0, 117, 0, 0, 118, 0, 0}) {
		t.Fatal("empty role inherited claims")
	}
	status, e := boostTrainingRestore(c, role)
	must115(t, e)
	if !bytes.Equal(status, []byte{2, 0, 0, 0, 0}) {
		t.Fatal("fresh role forced into event")
	}
	p := entryPayloads{BoostGifts: gifts, BoostTraining: status, Complete: []byte{1}}
	gi, ci, ti := -1, -1, -1
	for i, row := range p.packets() {
		switch row.ID {
		case 2265:
			gi = i
		case 124:
			ci = i
		case 2638:
			ti = i
		}
	}
	if gi < 0 || gi >= ci || ti <= ci {
		t.Fatal("event restoration order")
	}
	role.State, e = boostup.WriteState(role.State, boostup.State{Version: 1, Gifts: map[uint16]bool{117: true}, Activated: true, Training: boostup.Training{Step: 6, Phase: 2}})
	must115(t, e)
	gifts, e = boostGiftAvailability(c, role)
	must115(t, e)
	status, e = boostTrainingRestore(c, role)
	must115(t, e)
	if !bytes.Equal(gifts, []byte{2, 0, 117, 0, 1, 118, 0, 0}) || !bytes.Equal(status, []byte{0, 6, 2, 0, 1}) {
		t.Fatal("relogin lost per-role progress")
	}
}

// 第三关的完成是在 CMD29 技能事务里落地的：只有训练状态真的前进时才补发
// 一条 NOTI2638，任务面板才会刷新；同一状态重复下发会让已完成的引导再弹一次。
func TestBoostTrainingProgressFrameOnlyOnRealAdvance(t *testing.T) {
	c := &boostup.Catalog{Steps: []boostup.Step{{Number: 3, Mission: "skill vp option"}, {Number: 4, Guide: "dungeon"}}}
	before, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 3, Phase: 2, Claimed: map[byte]bool{3: true}}})
	must115(t, e)
	after, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 4, Claimed: map[byte]bool{3: true}}})
	must115(t, e)
	role := database.Character{ID: 7, State: before}
	if plan := boostTrainingProgress(c, role, database.Character{ID: 7, State: before}); len(plan) != 0 {
		t.Fatal("unchanged training state sent a frame")
	}
	if plan := boostTrainingProgress(c, role, database.Character{ID: 0, State: after}); len(plan) != 0 {
		t.Fatal("unowned character sent a frame")
	}
	plan := boostTrainingProgress(c, role, database.Character{ID: 7, State: after})
	if len(plan) != 1 || plan[0].ID != 2638 || !bytes.Equal(plan[0].Payload, []byte{0, 4, 0, 0, 1}) {
		t.Fatalf("advance frame %v", plan)
	}
	if plan := boostTrainingProgress(nil, role, database.Character{ID: 7, State: after}); len(plan) != 0 {
		t.Fatal("event disabled sent a frame")
	}
}

// 第九关的完成是在 CMD26 分解事务之后补的（源 `[mission][type] disjoint` 没有件数条件）：
// 同一口径 —— 只有训练状态真的前进才发 2638，且帧名要能区分来源，方便实机日志对账。
func TestBoostDisjointMissionProgressFrame(t *testing.T) {
	c := &boostup.Catalog{Steps: []boostup.Step{
		{Number: 9, Guide: "normal", Mission: "disjoint"},
		{Number: 10, Guide: "normal", Mission: "transform equip journal or equip item"},
	}}
	before, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true,
		Training: boostup.Training{Step: 9, Phase: 2, Claimed: map[byte]bool{9: true}}})
	must115(t, e)
	after, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true,
		Training: boostup.Training{Step: 10, Claimed: map[byte]bool{9: true}}})
	must115(t, e)
	role := database.Character{ID: 7, State: before}
	plan := boostMissionProgress("boost_disjoint_mission_progress", c, role, database.Character{ID: 7, State: after})
	if len(plan) != 1 || plan[0].ID != 2638 || plan[0].Name != "boost_disjoint_mission_progress" ||
		!bytes.Equal(plan[0].Payload, []byte{0, 10, 0, 0, 1}) {
		t.Fatalf("disjoint advance frame: %+v", plan)
	}
	if plan := boostMissionProgress("boost_disjoint_mission_progress", c, role, role); len(plan) != 0 {
		t.Fatal("unchanged disjoint state sent a frame")
	}
	if plan := boostMissionProgress("boost_disjoint_mission_progress", nil, role, database.Character{ID: 7, State: after}); len(plan) != 0 {
		t.Fatal("event disabled sent a disjoint frame")
	}
}

func TestBoostGiftSQLRealEntryReplayIsolationAndRollback(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit isolated database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	fixture, e := database.OpenTestFixture(ctx)
	must115(t, e)
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := fixture.Storage()
	must115(t, store.Migrate(ctx))
	must115(t, store.MigrateCharacterEvents(ctx))
	// assertBoostWearSQL seeds account_premiums directly for the point-mission
	// contract proof; the isolated schema must carry the premiums table too.
	must115(t, store.MigrateCashShop(ctx))
	must115(t, store.MigrateGrants(ctx))
	account, e := store.DevelopmentAccount(ctx, "boost-fixture")
	must115(t, e)
	assertBoostWearSQL(t, ctx, fixture, account)
	// [GAP] donor 基线的 assertBoostContractsSQL / assertBoosterOverflowSQL /
	// assertBoosterMultiEquipmentSQL 只存在于未交付的 donor main.go，调用的
	// useBooster、cashshop.BoosterCatalog、loot.PackageGrant 在本树已被
	// openBoosterItem 取代（契约回执/溢出/多装备的等价回归由 booster_flow_test.go
	// 覆盖），这三路活动侧专属断言因此没有可迁移的实现，接入时按本树 API 重写。
	// [GAP] assertBoostBeadSQL/assertBoostJournalFlowSQL 依赖 donor 基线的
	// loot.Service.EnchantByBead + inventory.BeadCatalog（PVF 直读宝珠目录）与
	// journal 子系统，本树附魔走 workflow.WearService.ApplyEnchantByBead（实机已验证）。
	// 对应测试没有可编译的调用侧，保留这条记录。
	assertBoostRosterSQL(t, ctx, store)
	assertBoostAutoSQL(t, ctx, fixture)
	assertBoostMailSQL(t, ctx, store)
	version := savecontract.Identity()
	ls := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.LootItem{1: {ID: 1, Kind: "stackable", StackableType: "[waste]", StackLimit: 1}}},
		BagRules: inventory.BagRules{Source: version, Slots: map[string][2]uint16{"[waste]": {65, 65}}, MissingStackLimit: 1}}
	lsvc := &workflow.LootService{Store: store, Loot: ls}
	gift := boostup.Gift{ID: 117, Event: 10017, Direct: true, Trigger: "click button", Items: []uint32{1}}
	create := func(name string, raw json.RawMessage) database.Character {
		r, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, Request: []byte{0}, ConfigVersion: version, State: raw}, 100)
		must115(t, err)
		return r
	}
	role := create("boostone", json.RawMessage(`{"level":1,"unknown":{"keep":42}}`))
	w := &worldSession{account: account, role: role, loot: ls, store: store, boostup: &boostup.Catalog{Gifts: []boostup.Gift{gift}}}
	plan, e := w.claimBoostGift(ctx, []byte{117, 0, 1, 0, 0, 0, 0, 0})
	must115(t, e)
	if len(plan) != 3 || plan[0].ID != 14 || plan[1].ID != 2265 || plan[2].ID != 643 {
		t.Fatal("inventory/state must precede claim ACK")
	}
	stale := role
	replay, e := w.claimBoostGift(ctx, []byte{117, 0, 1, 0})
	must115(t, e)
	if replay[0].ID != 13 {
		t.Fatal("retry published stale historical slot")
	}
	bag, e := inventory.ReadBag(w.role.State)
	must115(t, e)
	if len(bag.Items) != 1 || bag.Items[0].Amount != 1 {
		t.Fatal("duplicate grant")
	}
	capsuleCatalog := &boostup.Catalog{GoalLevel: 115, UsableLevel: 115, Capsules: map[uint32]boostup.Capsule{1: {Item: 1, Variant: 0}}, Steps: []boostup.Step{{Number: 1, Guide: "none", Mission: "equip item"}}}
	growCalls := 0
	grow := func(r database.Character, target byte) (database.Character, error) {
		growCalls++
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(r.State, &fields); err != nil {
			return r, err
		}
		fields["level"], _ = json.Marshal(target)
		r.State, _ = json.Marshal(fields)
		return r, nil
	}
	if _, _, _, err := lsvc.UseBoostCapsule(ctx, w.role, capsuleCatalog, 65, 0, grow, func(database.Character, loot.BoostCapsuleReceipt) error {
		return fmt.Errorf("injected packet preflight failure")
	}); err == nil {
		t.Fatal("capsule preflight failure accepted")
	}
	var capsuleReceipts int
	{
		v, e := fixture.EventCountByKey(ctx, role.ID, "boostup-capsule")
		must115(t, e)
		capsuleReceipts = int(v)
	}
	if capsuleReceipts != 0 {
		t.Fatal("failed capsule left receipt")
	}
	saved, receipt, used, e := lsvc.UseBoostCapsule(ctx, w.role, capsuleCatalog, 65, 0, grow, nil, json.RawMessage(`{"town":38,"area":1,"x":200,"y":200}`))
	must115(t, e)
	if !used || receipt.BeforeLevel != 1 || receipt.Level != 115 {
		t.Fatal("capsule growth receipt mismatch")
	}
	st, e := boostup.ReadState(saved.State)
	must115(t, e)
	b, e := inventory.ReadBag(saved.State)
	must115(t, e)
	if !st.Activated || st.Training.Step != 1 || len(b.Items) != 0 {
		t.Fatal("capsule state/item not atomic")
	}
	capsuleCatalog.Town = 222
	capsuleCatalog.Steps[0].Rewards = []boostup.Reward{{Item: 1, Count: 1}}
	stepWorld := &worldSession{account: account, role: saved, loot: ls, store: store, boostup: capsuleCatalog, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 0}}}
	claimBody := []byte{0x96, 2, 0, 0, 1, 0, 0, 0}
	stepPackets, e := stepWorld.boostStepRequest(ctx, claimBody, 680)
	must115(t, e)
	if len(stepPackets) != 3 || stepPackets[0].ID != 14 || stepPackets[1].ID != 2638 || stepPackets[2].ID != 680 {
		t.Fatal("real step claim order")
	}
	stepPackets, e = stepWorld.boostStepRequest(ctx, claimBody, 680)
	must115(t, e)
	if stepPackets[0].ID != 13 {
		t.Fatal("step retry replayed old slot")
	}
	assertBoostWorldSQL(t, ctx, store, stepWorld.role)
	assertBoostGuideClearSQL(t, ctx, fixture, stepWorld.role)
	// [GAP] assertBoostMapPickupSQL（donor 地图箱子系统）见 boostup_dungeon.go。
	_, _, used, e = lsvc.UseBoostCapsule(ctx, w.role, capsuleCatalog, 65, 0, grow, nil)
	must115(t, e)
	if used || growCalls != 2 {
		t.Fatal("retry consumed or grew twice", growCalls)
	}
	// Concurrent sockets with the original stale role still cannot grant twice.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, applied, err := lsvc.ClaimBoostGift(ctx, stale, gift)
			if err == nil && applied {
				err = fmt.Errorf("duplicate concurrent grant")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		must115(t, err)
	}
	other := create("boosttwo", json.RawMessage(`{}`))
	_, _, applied, e := lsvc.ClaimBoostGift(ctx, other, gift)
	must115(t, e)
	if !applied {
		t.Fatal("same-account second role denied")
	}
	racing := create("boostrace", json.RawMessage(`{}`))
	firstClaims := make(chan bool, 2)
	raceErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, won, err := lsvc.ClaimBoostGift(ctx, racing, gift)
			firstClaims <- won
			raceErrors <- err
		}()
	}
	wg.Wait()
	close(firstClaims)
	close(raceErrors)
	winners := 0
	for won := range firstClaims {
		if won {
			winners++
		}
	}
	for err := range raceErrors {
		must115(t, err)
	}
	if winners != 1 {
		t.Fatal("concurrent first claims must have exactly one winner", winners)
	}
	queryState, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 0}})
	must115(t, e)
	queryRole := create("boostquery", queryState)
	queryCatalog := &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1, Guide: "normal", Mission: "none", Rewards: []boostup.Reward{{Item: 1, Count: 1}}}}}
	queryWorld := &worldSession{account: account, role: queryRole, loot: ls, store: store, boostup: queryCatalog, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 0}}}
	queryPackets, e := queryWorld.boostStepRequest(ctx, claimBody, 681)
	must115(t, e)
	if len(queryPackets) != 2 || queryPackets[0].ID != 2638 || queryPackets[0].Payload[2] != 1 || queryPackets[1].ID != 681 {
		t.Fatal("normal guide query did not persist claimable state")
	}
	queryPackets, e = queryWorld.boostStepRequest(ctx, claimBody, 680)
	must115(t, e)
	if len(queryPackets) != 4 || queryPackets[1].ID != 2639 || queryPackets[2].ID != 2638 || queryPackets[2].Payload[0] != 2 || queryPackets[3].ID != 680 {
		t.Fatal("final claim did not close training/update roster")
	}
	full, e := inventory.SaveBag(json.RawMessage(`{}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 1, Amount: 1}}})
	must115(t, e)
	blocked := create("boostfull", full)
	// 本树没有 inventory.ErrStackBagFull 哨兵：满包由 bag.go 的
	// "bag category is full" 直接返回，这里按同一文案断言真实回滚。
	if _, _, _, err := lsvc.ClaimBoostGift(ctx, blocked, gift); err == nil || !strings.Contains(err.Error(), "bag category is full") {
		t.Fatal("expected actual full-bag rollback", err)
	}
	var n int
	{
		v, e := fixture.EventCount(ctx, blocked.ID)
		must115(t, e)
		n = int(v)
	}
	if n != 0 {
		t.Fatal("failed claim left receipt")
	}
	reopened, e := fixture.Reopen(ctx)
	must115(t, e)
	defer reopened.Close()
	roles, e := reopened.Characters(ctx, account)
	must115(t, e)
	for _, r := range roles {
		st, err := boostup.ReadState(r.State)
		must115(t, err)
		if st.Gifts[117] != (r.ID == role.ID || r.ID == other.ID || r.ID == racing.ID) {
			t.Fatal("reopen entitlement mismatch")
		}
	}
}
