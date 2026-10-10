package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"dfolan/internal/database/sqlcgen"
)

// vaultGrantFixture 起一个隔离 SQLite 库、一个开发账号和一个角色。
//
// 角色刚建出来时**没有**金库行（金库行唯一的建档时机是登录 LoadVault），
// 所以这个夹具正好落在奖励脚本真正遇到的场景上：目标行还不存在。
func vaultGrantFixture(t *testing.T) (*Store, context.Context, int64, Character, string) {
	t.Helper()
	s, ctx := sqlcTestStore(t)
	// SQLite 上 Migrate* 是诚实的空操作（Open 已把 schema 一次建全，见 migrations.go），
	// 这里沿用同目录既有测试的写法，只为把"本用例依赖金库表"写在代码里。
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateVault, s.MigrateAccountVault} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "vault-grant")
	if err != nil {
		t.Fatal(err)
	}
	account2, err := s.DevelopmentAccount(ctx, "vault-grant-other")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("b", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "VaultGrant",
		ConfigVersion: version, Request: []byte{1}, State: json.RawMessage(`{"unknown":"keep"}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if account2 == account {
		t.Fatal("两个开发账号拿到了同一个 id")
	}
	return s, ctx, account, role, version
}

// vaultSlots 直读某一层金库当前的 slots；行不存在时返回 ErrNotFound，
// 用来区分"只改没改值"和"到底建没建行"。
func vaultSlots(t *testing.T, ctx context.Context, s *Store, id int64, secondary bool) (uint16, error) {
	t.Helper()
	if secondary {
		row, err := s.queries.LockSecondaryVault(ctx, id)
		if err != nil {
			return 0, err
		}
		return uint16(row.Slots), nil
	}
	row, err := s.queries.LockPrimaryVault(ctx, id)
	if err != nil {
		return 0, err
	}
	return uint16(row.Slots), nil
}

func mustVaultSlots(t *testing.T, ctx context.Context, s *Store, id int64, secondary bool) uint16 {
	t.Helper()
	slots, err := vaultSlots(t, ctx, s, id, secondary)
	if err != nil {
		t.Fatalf("读取金库容量失败(secondary=%v): %v", secondary, err)
	}
	return slots
}

// 建号场景：金库行还不存在，一次调用必须把行建出来并直接落在目标容量上。
func TestGrantVaultSlotsCreatesMissingVaultRow(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	if _, err := vaultSlots(t, ctx, s, role.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("前置条件不成立：角色 %d 一开始就有金库1 行: %v", role.ID, err)
	}
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 24, false); err != nil {
		t.Fatalf("首次授予失败: %v", err)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 24 {
		t.Fatalf("金库1 容量 = %d，期望 24", got)
	}
	// 登录读路径看到的是同一个值，且 items 仍是建行默认的空数组（授予不碰物品）。
	vault, err := s.LoadVault(ctx, account, role.ID, 8, version)
	if err != nil {
		t.Fatal(err)
	}
	if vault.Slots != 24 {
		t.Fatalf("LoadVault 读到容量 %d，期望 24", vault.Slots)
	}
	if string(vault.Items) != "[]" {
		t.Fatalf("授予改了金库物品: %s", vault.Items)
	}
	if vault.ConfigVersion != version {
		t.Fatalf("授予改了配置版本: %q", vault.ConfigVersion)
	}
}

// 只升不降：先给 24，再给更小的 8，必须保持 24（且不报错、不算失败）。
func TestGrantVaultSlotsNeverLowersCapacity(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 24, false); err != nil {
		t.Fatal(err)
	}
	// 先放一件东西进去，确认"更大才写 slots"这条路径不会顺手重写 items。
	items := json.RawMessage(`[{"slot":1,"template":123}]`)
	if err := s.queries.SavePrimaryVaultItems(ctx, sqlcgen.SavePrimaryVaultItemsParams{CharacterID: role.ID, Items: items}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 8, false); err != nil {
		t.Fatalf("给小容量不该报错: %v", err)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 24 {
		t.Fatalf("降级调用把容量改成了 %d，期望仍是 24", got)
	}
	vault, err := s.LoadVault(ctx, account, role.ID, 8, version)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(t, vault.Items, items) {
		t.Fatalf("降级调用改了金库物品: %s", vault.Items)
	}
	// 更大的目标仍然要生效，否则"只升不降"就退化成"永不改"。
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 40, false); err != nil {
		t.Fatal(err)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 40 {
		t.Fatalf("升容调用后容量 = %d，期望 40", got)
	}
}

// 幂等：同一目标重复调用是同一固定点，第二次既不改值也不报错。
func TestGrantVaultSlotsIsIdempotent(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	for i := 0; i < 3; i++ {
		if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 40, false); err != nil {
			t.Fatalf("第 %d 次调用失败: %v", i+1, err)
		}
		if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 40 {
			t.Fatalf("第 %d 次调用后容量 = %d，期望 40", i+1, got)
		}
	}
}

// 非法目标：0、7、100、265、300 都要报错，并且不落盘（不给不存在的角色建行，
// 也不改动已存在的容量）。
func TestGrantVaultSlotsRejectsInvalidTarget(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	for _, target := range []uint16{0, 7, 100, 265, 300, 65535} {
		if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, target, false); err == nil {
			t.Fatalf("目标容量 %d 非法，却返回成功", target)
		}
	}
	if _, err := vaultSlots(t, ctx, s, role.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("非法目标建出了行: %v", err)
	}
	// 已存在的行也不能被非法目标改动。
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 24, false); err != nil {
		t.Fatal(err)
	}
	for _, target := range []uint16{100, 300} {
		if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, target, false); err == nil {
			t.Fatalf("目标容量 %d 非法，却返回成功", target)
		}
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 24 {
		t.Fatalf("非法目标改动了已有容量: %d", got)
	}
}

// 初始容量同样受档位约束：Ensure 会把它直接写进新行，野值必须挡在事务之前。
func TestGrantVaultSlotsRejectsInvalidInitial(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	for _, initial := range []uint16{0, 7, 100, 300} {
		if err := s.GrantVaultSlots(ctx, account, role.ID, version, initial, 264, false); err == nil {
			t.Fatalf("初始容量 %d 非法，却返回成功", initial)
		}
	}
	if _, err := vaultSlots(t, ctx, s, role.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("非法初始容量建出了行: %v", err)
	}
	if err := s.GrantVaultSlots(ctx, account, role.ID, "short", 8, 264, false); err == nil {
		t.Fatal("配置版本长度非法，却返回成功")
	}
}

// 金库 1 / 金库 2 是两张表、两套容量：一侧授予不能影响另一侧。
func TestGrantVaultSlotsSecondaryIsSeparateTable(t *testing.T) {
	s, ctx, account, role, version := vaultGrantFixture(t)
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 24, 40, true); err != nil {
		t.Fatal(err)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, true); got != 40 {
		t.Fatalf("金库2 容量 = %d，期望 40", got)
	}
	if _, err := vaultSlots(t, ctx, s, role.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("金库2 的授予建出了金库1 的行: %v", err)
	}
	if err := s.GrantVaultSlots(ctx, account, role.ID, version, 8, 56, false); err != nil {
		t.Fatal(err)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, false); got != 56 {
		t.Fatalf("金库1 容量 = %d，期望 56", got)
	}
	if got := mustVaultSlots(t, ctx, s, role.ID, true); got != 40 {
		t.Fatalf("金库1 的授予改动了金库2: %d", got)
	}
	// 金库2 走 space 45 的登录读路径，值必须一致。
	vault, err := s.LoadVault(ctx, account, role.ID, 24, version, 45)
	if err != nil {
		t.Fatal(err)
	}
	if vault.Slots != 40 {
		t.Fatalf("LoadVault(space 45) 读到 %d，期望 40", vault.Slots)
	}
}

// 账号/角色不匹配时明确报错，并且不给别人的角色建行。
func TestGrantVaultSlotsRejectsForeignCharacter(t *testing.T) {
	s, ctx, _, role, version := vaultGrantFixture(t)
	other, err := s.DevelopmentAccount(ctx, "vault-grant-other")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantVaultSlots(ctx, other, role.ID, version, 8, 264, false); err == nil {
		t.Fatal("别人的账号却授予成功")
	}
	if _, err := vaultSlots(t, ctx, s, role.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权授予建出了行: %v", err)
	}
}
