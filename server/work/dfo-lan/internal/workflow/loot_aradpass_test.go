package workflow

import (
	"context"
	"dfolan/internal/aradpass"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// aradPassWorkflowFixture 起一个一次性 SQLite 库（OpenTestFixture 自带的临时文件，
// 不碰玩家存档），造一个角色，返回配套的 LootService 与源目录。
//
// PostgreSQL 已于 2026-10-05 移除，夹具不再需要外部 DSN，所以这里**不设环境门禁** ——
// 与 internal/database/fixture.go 的注释一致："integration tests that use it run everywhere"。
func aradPassWorkflowFixture(t *testing.T) (context.Context, *database.Store, database.Character, *LootService, *aradpass.Catalog) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "arad-pass-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := savecontract.Identity()
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "AradPassFixture", Profession: 0, Request: []byte{0},
		ConfigVersion: version, State: json.RawMessage(`{"level":115}`),
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	items := map[uint32]catalog.LootItem{}
	for _, id := range []uint32{590722601, 590722646, 590722598, 590722593, 590722589} {
		items[id] = catalog.LootItem{ID: id, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}
	}
	ls := &loot.Service{
		Catalog:  catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: items},
		BagRules: rules,
	}
	source := &aradpass.Catalog{
		Levels: []aradpass.Level{{Level: 1, Exp: 0}, {Level: 2, Exp: 600}, {Level: 3, Exp: 1200}},
		Rewards: []aradpass.Reward{
			{Track: aradpass.TrackNormal, Slot: 1, Item: 590722601, Count: 2, Level: 1},
			{Track: aradpass.TrackRoyal, Slot: 1, Item: 590722646, Count: 1, Level: 1},
		},
		RoyalFirstReward: []uint32{590722598, 590722593},
		// 源 `[daily reward list] 590722589 1`（Royal Pass 每日登录礼包）。
		DailyReward:  []uint32{590722589},
		PurchaseExp:  5000,
	}
	return ctx, store, role, &LootService{Store: store, Loot: ls}, source
}

// Royal Pass 每日登录礼包：**一天一次**（09:00 UTC 换日），同一天重点只读回原回执、不再发货。
func TestClaimAradPassRoyalDailyOncePerDay(t *testing.T) {
	ctx, store, role, lsvc, source := aradPassWorkflowFixture(t)
	role, receipt, applied, err := lsvc.ClaimAradPassRoyalDaily(ctx, role, source, "2026-10-11")
	if err != nil {
		t.Fatal(err)
	}
	if !applied || len(receipt.Items) == 0 || len(receipt.Slots) == 0 {
		t.Fatalf("applied=%v receipt=%+v", applied, receipt)
	}
	// 存档真值：换日键必须落库。
	fresh := aradPassReload(t, ctx, store, role)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if st.RoyalDailyKey != "2026-10-11" {
		t.Fatalf("状态里应记下换日键，得到 %q", st.RoyalDailyKey)
	}
	// 同一天再点（业主会连点）：不发货，但回执仍与首次一致 ⇒ 客户端不会卡在途。
	_, again, applied2, err := lsvc.ClaimAradPassRoyalDaily(ctx, role, source, "2026-10-11")
	if err != nil {
		t.Fatal(err)
	}
	if applied2 || len(again.Items) == 0 {
		t.Fatalf("同一天不该再发：applied=%v receipt=%+v", applied2, again)
	}
	// 换日（09:00 UTC 之后）：又能领一次。
	_, _, applied3, err := lsvc.ClaimAradPassRoyalDaily(ctx, role, source, "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	if !applied3 {
		t.Fatal("换日后应能再领一次")
	}
}

// 领奖 → 落袋 → 标已领，状态与收据都在库里；重放不重复发货。
func TestClaimAradPassRewardPersistsAndReplays(t *testing.T) {
	ctx, store, role, lsvc, source := aradPassWorkflowFixture(t)
	saved, receipt, applied, err := lsvc.ClaimAradPassReward(ctx, role, source, aradpass.TrackNormal, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !applied || receipt.Item != 590722601 || receipt.Amount != 2 || len(receipt.Slots) != 1 {
		t.Fatalf("applied=%v receipt=%+v", applied, receipt)
	}
	// 存档真值：重读角色行，不能只看内存里的返回值。
	fresh := aradPassReload(t, ctx, store, role)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClaimed(aradpass.TrackNormal, 1) {
		t.Fatal("已领标记没有落库")
	}
	if saved.State == nil || string(fresh.State) == string(role.State) {
		t.Fatal("角色状态应当真的被写回")
	}
	// 重放：applied=false，但回执形状与首次一致（会话侧据此回成功应答而不是报错）。
	_, again, applied2, err := lsvc.ClaimAradPassReward(ctx, role, source, aradpass.TrackNormal, 1)
	if err != nil {
		t.Fatal(err)
	}
	if applied2 {
		t.Fatal("同键重放不应再次应用")
	}
	if !reflect.DeepEqual(again, receipt) {
		t.Fatalf("重放回执必须与首次一致：%+v vs %+v", again, receipt)
	}
}

// 背包满 → 整笔回滚：既不发东西，也不写"已领"标记（玩家清理后可再领）。
func TestClaimAradPassRewardRollsBackWhenBagFull(t *testing.T) {
	ctx, store, role, lsvc, source := aradPassWorkflowFixture(t)
	bag := inventory.Bag{Version: "ordinary-bag-v1"}
	for i := 0; i < 56; i++ {
		bag.Items = append(bag.Items, inventory.BagItem{Template: 590722601, Amount: 1000})
	}
	raw, err := inventory.SaveBag(role.State, bag)
	if err != nil {
		t.Fatal(err)
	}
	full, err := store.CreateCharacter(ctx, database.Character{
		AccountID: role.AccountID, Name: "AradPassFull", Profession: 0, Request: []byte{0},
		ConfigVersion: role.ConfigVersion, State: raw,
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = lsvc.ClaimAradPassReward(ctx, full, source, aradpass.TrackNormal, 1); err == nil {
		t.Fatal("背包满时必须失败")
	}
	fresh := aradPassReload(t, ctx, store, full)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if st.IsClaimed(aradpass.TrackNormal, 1) {
		t.Fatal("失败的事务不能留下已领标记")
	}
}

// 激活：置 Royal + 加购票经验 + 发首发奖励；重复调用走收据、不再发货。
func TestActivateAradPassIsIdempotent(t *testing.T) {
	ctx, _, role, lsvc, source := aradPassWorkflowFixture(t)
	saved, receipt, applied, err := lsvc.ActivateAradPass(ctx, role, source)
	if err != nil {
		t.Fatal(err)
	}
	if !applied || !receipt.Royal || receipt.PurchaseExp != 5000 || len(receipt.Items) != 2 {
		t.Fatalf("applied=%v receipt=%+v", applied, receipt)
	}
	st, err := aradpass.ReadState(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Royal || st.Exp != 5000 || !st.FirstRewards {
		t.Fatalf("state = %+v", st)
	}
	_, again, applied2, err := lsvc.ActivateAradPass(ctx, role, source)
	if err != nil {
		t.Fatal(err)
	}
	if applied2 || again.PurchaseExp != 5000 {
		t.Fatalf("重复激活必须走收据：applied=%v receipt=%+v", applied2, again)
	}
}

// 等级不足时领奖被拒（准入校验在**事务内**，不是调用方的君子协定）。
func TestClaimAradPassRewardRejectsBelowLevel(t *testing.T) {
	ctx, _, role, lsvc, source := aradPassWorkflowFixture(t)
	_ = ctx
	source.Rewards = append(source.Rewards, aradpass.Reward{Track: aradpass.TrackNormal, Slot: 2, Item: 590722601, Count: 1, Level: 3})
	if _, _, _, err := lsvc.ClaimAradPassReward(ctx, role, source, aradpass.TrackNormal, 3); err == nil {
		t.Fatal("等级不足必须被拒")
	} else if !strings.Contains(err.Error(), "below the reward level") {
		t.Fatalf("错误应说明等级不足，实际：%v", err)
	}
}

// aradPassReload 从库里重读角色行（断言读的是提交后的存档，不是内存返回值）。
func aradPassReload(t *testing.T, ctx context.Context, store *database.Store, role database.Character) database.Character {
	t.Helper()
	roles, err := store.Characters(ctx, role.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, cur := range roles {
		if cur.ID == role.ID {
			return cur
		}
	}
	t.Fatalf("character %d not persisted", role.ID)
	return database.Character{}
}
