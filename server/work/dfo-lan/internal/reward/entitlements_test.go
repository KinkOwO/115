package reward

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// entitlements_test.go —— 角色待遇（存档字段，不是物品）的 Lua 侧能力。
//
// 业主 2026-10-06 口径：**能不能改是服务端能力，改成多少由 mod 的脚本决定**。
// 所以这里钉三件事：该收集的要收集（含按位或）、没调的绝不碰、越界要报错而不是静默截断。

// TestCharacterCreateCollectsSecondBatchEntitlements：第二批能力（宠物/金库/材料仓/皮肤）
// 的收集口径 —— 参数按**模板**聚合，空间号只认 2/45/12。
func TestCharacterCreateCollectsSecondBatchEntitlements(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "kit2.lua", `
on("character_create", function(ctx)
  grant_pet(63003)
  grant_pet_item(10418035, 1000)
  grant_pet_item(10418035, 500)
  expand_vault(2, 264)
  expand_vault(45, 264)
  expand_vault(12, 320)
  grant_account_material(3033, 10000)
  grant_account_material(3033, 5000)
  grant_account_material(3262, 10000)
  unlock_skins()
end)
`)

	var got []Entitlements
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Entitlements: func(_ context.Context, _ Recipient, _ string, e Entitlements) error {
			got = append(got, e)
			return nil
		},
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})

	require.Len(t, got, 1)
	e := got[0]
	require.Len(t, e.Pets, 1)
	require.Equal(t, uint32(63003), e.Pets[0].ID)
	require.Equal(t, uint32(1), e.Pets[0].Count, "本体一次只发 1 只")
	require.Len(t, e.PetItems, 2)
	require.Equal(t, uint32(1000), e.PetItems[0].Count)
	require.NotNil(t, e.Vault1Slots)
	require.Equal(t, uint16(264), *e.Vault1Slots)
	require.NotNil(t, e.Vault2Slots)
	require.Equal(t, uint16(264), *e.Vault2Slots)
	require.NotNil(t, e.AccountVaultSlots)
	require.Equal(t, uint16(320), *e.AccountVaultSlots)
	// 同一模板两次调用要**按模板累加**（落库时再映射到固定槽位）。
	require.Equal(t, uint32(15000), e.AccountMaterialByTemplate[3033])
	require.Equal(t, uint32(10000), e.AccountMaterialByTemplate[3262])
	require.True(t, e.UnlockSkins)
}

// TestExpandVaultRejectsUnknownSpace：空间号不是 2/45/12 要报错（别默默写错层）。
func TestExpandVaultRejectsUnknownSpace(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "bad.lua", `
on("character_create", function(ctx) expand_vault(7, 264) end)
`)

	called := false
	var logged []string
	s, err := New(Options{
		Scripts:      os.DirFS(dir),
		Entitlements: func(context.Context, Recipient, string, Entitlements) error { called = true; return nil },
		Log:          func(format string, args ...any) { logged = append(logged, format) },
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.False(t, called)
	require.NotEmpty(t, logged)
}

func TestCharacterCreateCollectsEntitlements(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "kit.lua", `
on("character_create", function(ctx)
  unlock_equip_slots(1)     -- support
  unlock_equip_slots(58)    -- 其余四位：2+8+16+32
  expand_bag(2)
  expand_avatar(15)
  grant_revive_coin(100)
end)
`)

	var got []Entitlements
	var keys []string
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Entitlements: func(_ context.Context, _ Recipient, key string, e Entitlements) error {
			got = append(got, e)
			keys = append(keys, key)
			return nil
		},
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})

	require.Len(t, got, 1)
	e := got[0]
	// 按位或：两次调用叠加成 59（五个扩展位全开），不是覆盖成 58。
	require.Equal(t, byte(59), e.EquipSlotMask)
	require.NotNil(t, e.BagTier)
	require.Equal(t, byte(2), *e.BagTier)
	require.NotNil(t, e.AvatarTier)
	require.Equal(t, byte(15), *e.AvatarTier)
	require.Equal(t, uint32(100), e.ReviveCoins)
	require.Equal(t, "reward:character_create:kit.lua:7:state", keys[0])
}

// TestEntitlementsNotCollectedWhenUnused：脚本没调那几条函数就**一次回调都不发** ——
// 这是"绝不把玩家已有的待遇抹成 0"的第一道保证（零值不允许进事务）。
func TestEntitlementsNotCollectedWhenUnused(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "items.lua", `
on("character_create", function(ctx)
  grant_item(0, 1000)
end)
`)

	called := false
	s, err := New(Options{
		Scripts:      os.DirFS(dir),
		Entitlements: func(context.Context, Recipient, string, Entitlements) error { called = true; return nil },
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.False(t, called, "没写待遇就不该有待遇事务")

	// Empty 也覆盖到：零值批次必须被判为"什么都没改"。
	require.True(t, Entitlements{}.Empty())
	zero := byte(0)
	require.False(t, Entitlements{BagTier: &zero}.Empty(), "档位指针非 nil 就是一次真实的修改意图")
}

// TestGrantReviveCoinRejectsNonPositive：非法入参要报错（脚本作者能看见），不是静默丢弃。
func TestGrantReviveCoinRejectsNonPositive(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "bad.lua", `
on("character_create", function(ctx)
  grant_revive_coin(0)
end)
`)

	var logged []string
	called := false
	s, err := New(Options{
		Scripts:      os.DirFS(dir),
		Entitlements: func(context.Context, Recipient, string, Entitlements) error { called = true; return nil },
		Log:          func(format string, args ...any) { logged = append(logged, strings.TrimSpace(format)) },
	})
	require.NoError(t, err)
	require.NotPanics(t, func() { s.CharacterCreate(context.Background(), Recipient{CharacterID: 7}) })
	require.False(t, called)
	require.NotEmpty(t, logged, "越界/非法入参必须留下一条日志")
}

// TestUnlockEquipSlotsRejectsOutOfRangeMask：mask 超出字节范围时报错（不是截断成别的位）。
func TestUnlockEquipSlotsRejectsOutOfRangeMask(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "bad.lua", `
on("character_create", function(ctx)
  unlock_equip_slots(300)
end)
`)

	called := false
	var logged []string
	s, err := New(Options{
		Scripts:      os.DirFS(dir),
		Entitlements: func(context.Context, Recipient, string, Entitlements) error { called = true; return nil },
		Log:          func(format string, args ...any) { logged = append(logged, format) },
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.False(t, called)
	require.NotEmpty(t, logged)
}
