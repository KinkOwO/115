package accountlist

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/savecontract"
	"encoding/json"
	"path/filepath"
	"testing"
)

// 概览行必须读出等级/经验/金币，并把「金币读不出来」与「金币是 0」区分开：
// 旧存档读不动时显示 null，而不是把 0 当成事实报给 GM。
func TestDescribeReadsLevelExperienceAndGold(t *testing.T) {
	role := database.Character{
		ID:   7,
		Name: "Leveler",
		State: json.RawMessage(`{"level":50,"experience":123456,` +
			`"inventory":{"version":"ordinary-bag-v1","gold":4321,"items":[]}}`),
	}
	row := describe(role)
	if row.Level != 50 || row.Experience != 123456 {
		t.Fatalf("level/experience = %d/%d, want 50/123456", row.Level, row.Experience)
	}
	if row.Gold == nil || *row.Gold != 4321 {
		t.Fatalf("gold = %v, want 4321", row.Gold)
	}

	// 没有 inventory 键的旧存档：ReadBag 认它是空背包，所以金币是 0（事实），不是未知。
	empty := describe(database.Character{ID: 8, Name: "Empty", State: json.RawMessage(`{"level":3}`)})
	if empty.Level != 3 {
		t.Fatalf("legacy level = %d, want 3", empty.Level)
	}
	if empty.Gold == nil || *empty.Gold != 0 {
		t.Fatalf("legacy gold = %v, want 0", empty.Gold)
	}

	// 形状读不动的存档：金币未知（null），等级仍然照读 —— 「读不出来」不能显示成 0。
	broken := describe(database.Character{ID: 10, Name: "Broken", State: json.RawMessage(`{"level":5,"inventory":{"version":"other-bag"}}`)})
	if broken.Level != 5 {
		t.Fatalf("broken level = %d, want 5", broken.Level)
	}
	if broken.Gold != nil {
		t.Fatalf("broken gold = %v, want nil (unknown, not zero)", *broken.Gold)
	}

	// 等级/经验写成字符串的历史存档也要读出来（只影响显示，不改存档）。
	texted := describe(database.Character{ID: 9, Name: "Texted", State: json.RawMessage(`{"level":"12","experience":"900"}`)})
	if texted.Level != 12 || texted.Experience != 900 {
		t.Fatalf("string-encoded level/experience = %d/%d, want 12/900", texted.Level, texted.Experience)
	}
}

// 读路径必须对两个引擎给出同一份结论：这里在 SQLite 上端到端跑一遍（真实驱动），
// PostgreSQL 侧由 internal/database 的 DFO_TEST_POSTGRES_DSN 用例覆盖同一批
// Store 方法（Accounts / AccountCera / AdminCharacters）。
func TestAccountsAndCharactersReadOnSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "accountlist.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(store.Close)
	account, err := store.DevelopmentAccount(ctx, "gm-list")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	if _, err := store.CreateCharacter(ctx, database.Character{
		AccountID:     account,
		WireID:        1,
		Name:          "Listed",
		ConfigVersion: savecontract.Identity(),
		Request:       []byte{0},
		State: json.RawMessage(`{"level":60,"experience":999,` +
			`"inventory":{"version":"ordinary-bag-v1","gold":250,"items":[]}}`),
	}, 8); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	accounts, err := store.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	found := false
	for _, candidate := range accounts {
		if candidate.Username != "gm-list" {
			continue
		}
		found = true
		roles, err := store.AdminCharacters(ctx, candidate.ID)
		if err != nil {
			t.Fatalf("AdminCharacters: %v", err)
		}
		if len(roles) != 1 {
			t.Fatalf("AdminCharacters returned %d roles, want 1", len(roles))
		}
		row := describe(roles[0])
		if row.Name != "Listed" || row.Level != 60 || row.Experience != 999 {
			t.Fatalf("row = %+v, want Listed/60/999", row)
		}
		if row.Gold == nil || *row.Gold != 250 {
			t.Fatalf("row gold = %v, want 250", row.Gold)
		}
	}
	if !found {
		t.Fatal("the account created by DevelopmentAccount is missing from Accounts")
	}

	// 驱动判定与启动器一致：只有 sqlite_path 的配置在两边都是 SQLite。
	if driver, err := database.EngineForConfig(database.Config{SQLitePath: "x.db"}); err != nil || driver != database.DriverSQLite {
		t.Fatalf("EngineForConfig = %q (%v), want sqlite", driver, err)
	}
}
