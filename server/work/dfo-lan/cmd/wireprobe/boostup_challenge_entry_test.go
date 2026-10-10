package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"encoding/json"
	"testing"
	"time"
)

// boostChallengeEntryCatalog 只给一条挑战（level>=115 解锁），够覆盖「已毕业 → 登记」这条链。
func boostChallengeEntryCatalog() *boostup.Catalog {
	return &boostup.Catalog{GoalLevel: 115, Challenges: []boostup.ChallengeDefinition{{Index: 0, UnlockKind: "level", UnlockValue: 115, Kind: "clear endkeeper of order", GuideDungeon: 100005014, Goal: 10, Repeat: 1}}}
}

func boostChallengeEntryService(version string) *loot.Service {
	return &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.LootItem{6001: {ID: 6001, Kind: "stackable", StackableType: "[booster]", StackLimit: 100}}}}
}

// TestBoostChallengeEntrySyncEnrollsGraduated 钉住「进城自动登记 665」这条接线。
//
// 实机 2026-10-07 暴露的缺口：登记此前只挂在副作用上（穿脱装备 → reconcileBoostEquipment、
// 走训练步、662 领奖复核），于是**正常毕业的角色进城看不到 665 面板，必须去动一次装备才出现**。
// 而客户端全程不发 CMD681（实机会话里 681 = 0 次），面板**只由服务端推的 2722 渲染**
// ⇒「推帧」就是接线本身，进城路径必须自己对账。
//
// 判据用「已毕业」（step 与 Claimed[step-1]）而不是「满级」：满级但没走 662 的角色不应被登记。
func TestBoostChallengeEntrySyncEnrollsGraduated(t *testing.T) {
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()
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
	must115(t, store.MigrateMailbox(ctx))

	a, e := store.DevelopmentAccount(ctx, "boost-entry-sync")
	must115(t, e)
	// 存档身份 = 服务端契约版本（savecontract.Identity），不是 Checksum：
	// ArchiveSnapshot.SaveIdentity() 直接返回 Identity()，Checksum 只是形状占位；
	// 用 64-hex 当 ConfigVersion 会被 Normalize 折叠成空串，比对必然失败。
	version := savecontract.Identity()
	cat := boostChallengeEntryCatalog()
	ls := boostChallengeEntryService(version)
	cs := &character.Service{Store: store}

	// 已毕业但尚未登记：step=12 且 Claimed[11] 就是毕业证明（与发放同一笔事务提交）。
	graduated := boostup.State{Version: 1, Activated: true,
		Training: boostup.Training{Step: 12, Phase: 3, Finished: true, Claimed: map[byte]bool{11: true}}}
	raw, e := boostup.WriteState(json.RawMessage(`{"level":115,"fame":40906}`), graduated)
	must115(t, e)
	grad, e := store.CreateCharacter(ctx, database.Character{AccountID: a, Name: "EntrySyncGrad", ConfigVersion: version, Request: []byte{0}, State: raw}, 100)
	must115(t, e)

	w := &worldSession{account: a, role: grad, loot: ls, store: store, boostup: cat, characters: cs}
	conn := &gameConnection{gatewayRuntime: &gatewayRuntime{gameStore: store, lootService: ls}, worldState: w}

	role, body, e := conn.boostChallengeEntrySync(ctx, grad)
	must115(t, e)
	if len(body) != protocol.BoostChallengeBodyLen {
		t.Fatalf("毕业角色进城必须出 2722（%d B），实得 %d B", protocol.BoostChallengeBodyLen, len(body))
	}
	if body[2] != 1 {
		t.Fatal("2722 的登记位（@2）不是 1")
	}
	st, e := boostup.ReadState(role.State)
	must115(t, e)
	if st.Challenge == nil || !st.Challenge.Enrolled || !st.Challenge.Rows[0].Unlocked {
		t.Fatalf("进城未登记 665：%+v", st.Challenge)
	}

	// 幂等：已登记角色再次进城仍出恢复帧（登录恢复的既有语义），且不改动进度。
	again, againBody, e := conn.boostChallengeEntrySync(ctx, role)
	must115(t, e)
	if len(againBody) != protocol.BoostChallengeBodyLen {
		t.Fatalf("已登记角色进城仍应出恢复帧，实得 %d B", len(againBody))
	}
	replay, e := boostup.ReadState(again.State)
	must115(t, e)
	if replay.Challenge == nil || !replay.Challenge.Enrolled || replay.Challenge.Rows[0].Progress != 0 || replay.Challenge.Rows[0].UnlockClaimed {
		t.Fatalf("重复对账改动了挑战状态：%+v", replay.Challenge)
	}

	// 未毕业（没走 662）：不出帧、不写登记 ⇒ 进城序列逐字节不变。
	plain, e := boostup.WriteState(json.RawMessage(`{"level":1}`), boostup.State{Version: 1})
	must115(t, e)
	mild, e := store.CreateCharacter(ctx, database.Character{AccountID: a, Name: "EntrySyncPlain", ConfigVersion: version, Request: []byte{0}, State: plain}, 100)
	must115(t, e)
	w2 := &worldSession{account: a, role: mild, loot: ls, store: store, boostup: cat, characters: cs}
	conn2 := &gameConnection{gatewayRuntime: &gatewayRuntime{gameStore: store, lootService: ls}, worldState: w2}
	plainRole, plainBody, e := conn2.boostChallengeEntrySync(ctx, mild)
	must115(t, e)
	if plainBody != nil {
		t.Fatalf("未毕业角色不得出 2722，实得 %d B", len(plainBody))
	}
	pst, e := boostup.ReadState(plainRole.State)
	must115(t, e)
	if pst.Challenge != nil && pst.Challenge.Enrolled {
		t.Fatal("未毕业角色被登记")
	}

	// 活动未绑定（源里没有挑战）：整条链静默跳过，不失败。
	w3 := &worldSession{account: a, role: grad, loot: ls, store: store, boostup: &boostup.Catalog{GoalLevel: 115}, characters: cs}
	conn3 := &gameConnection{gatewayRuntime: &gatewayRuntime{gameStore: store, lootService: ls}, worldState: w3}
	if _, body, e := conn3.boostChallengeEntrySync(ctx, grad); e != nil || body != nil {
		t.Fatalf("无挑战目录时不得出帧：%d B %v", len(body), e)
	}
}
