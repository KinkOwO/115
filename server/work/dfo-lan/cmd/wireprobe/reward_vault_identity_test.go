package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"

	"github.com/stretchr/testify/require"
)

// TestVaultCapacityRowMustCarryTheVaultRulesIdentity 是 2026-10-06 实机故障的回归门禁。
//
// 现场：新角色创建时脚本要用 `expand_vault` 给金库开容量，而金库行在那一刻**还不存在**
// （唯一建档时机是登录 LoadVault），所以奖励侧必须自己 Ensure 出来。如果 Ensure 用的是
// **角色存档身份**（reward.Recipient.ConfigVersion），行里的 `config_version` 就与金库入口的
// 判据（`internal/workflow/vault.go`：必须等于金库规则的 SourceSHA256）不符 ⇒ 该角色之后
// **每次入场都被判成 "vault requires configuration migration"**，玩家表现是
// **建完号进不去游戏**。实机日志：`kind=vault_entry_rejected, error=vault requires configuration migration`。
//
// 这个用例钉住两件事：
//  1. 用**规则身份**建行 ⇒ 入场检查通过（这是奖励侧现在必须走的路径）；
//  2. 用**角色身份**建行 ⇒ 入场检查必须失败（判据本身没有被放宽，别指望它容错）。
func TestVaultCapacityRowMustCarryTheVaultRulesIdentity(t *testing.T) {
	ctx := context.Background()
	fixture, err := database.OpenTestFixture(ctx)
	require.NoError(t, err)
	defer fixture.Close()
	store := fixture.Storage()
	account, err := store.DevelopmentAccount(ctx, "reward-vault-identity")
	require.NoError(t, err)

	rules := inventory.VaultRules{
		SourceSHA256:          strings.Repeat("a", 64),
		InitialSlots:          8,
		InitialSecondarySlots: 24,
		VerifiedSlots:         []uint16{8, 24, 40, 264},
	}
	svc := &workflow.VaultService{Store: store, VaultService: inventory.VaultService{Rules: rules}}

	newRole := func(name string, slot int) database.Character {
		t.Helper()
		role, err := store.CreateCharacter(ctx, database.Character{
			AccountID:     account,
			Name:          name,
			ConfigVersion: strings.Repeat("b", 64),
			Request:       []byte{1},
			State:         json.RawMessage(`{"unknown":"keep"}`),
		}, slot)
		require.NoError(t, err)
		return role
	}

	good := newRole("VaultGood", 24)
	// 夹具前提：两个身份必须真的不同 —— 否则这个用例什么也证明不了。
	require.NotEqual(t, rules.SourceSHA256, good.ConfigVersion)
	require.NoError(t, store.GrantVaultSlots(ctx, account, good.ID, rules.SourceSHA256, rules.InitialSlots, 264, false))
	if _, err := svc.BootstrapSpace(ctx, good, 2); err != nil {
		t.Fatalf("用金库规则身份建出来的行应当能过入场检查，实际被拒：%v", err)
	}

	bad := newRole("VaultBad", 25)
	require.NoError(t, store.GrantVaultSlots(ctx, account, bad.ID, bad.ConfigVersion, rules.InitialSlots, 264, false))
	if _, err := svc.BootstrapSpace(ctx, bad, 2); err == nil {
		t.Fatal("用角色存档身份建出来的行必须被入场检查拒掉 —— 这正是 2026-10-06 建完号进不去的原因")
	}
}
