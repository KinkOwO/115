package database

import (
	"context"
	"strings"
	"testing"
)

// UnlockSkins 是奖励脚本「新角色直接全解锁」用的账号级批量口（CMD507 action 169 那条
// 逐条路径走的是 UnlockSkin，见 cmd/wireprobe/skin_storage_flow.go）。它的契约有四条：
//
//  1. 一批 N 条模板 → 返回 N，且每条都真的落库；
//  2. 重复调用（重放、脚本重跑）→ 返回 0、不报错，且不覆盖第一次存下的皮肤号；
//  3. 批里任何一条模板号/皮肤号为 0 → 整批拒绝并报出错条目，一行都不写；
//  4. 空批/nil → 0, nil，不碰盘。
//
// 这些必须在**真实 SQLite store** 上验证：要证明的正是「一个事务 + ON CONFLICT
// DO NOTHING」这两件事，用假的 query 层测只会复述实现。

// skinBatch 造一批形状合法、模板号互不相同的条目（皮肤号按顺序派生，方便断言）。
func skinBatch(templates ...uint32) []AccountSkin {
	out := make([]AccountSkin, 0, len(templates))
	for i, template := range templates {
		out = append(out, AccountSkin{SourceTemplate: template, SkinKey: 40000 + uint32(i)})
	}
	return out
}

// skinKeysByTemplate 把 ListSkins 的结果收成 template→skin_key。
// 主键是 (account_id, source_template)，所以这个映射的长度就是真实行数。
func skinKeysByTemplate(t *testing.T, ctx context.Context, store *Store, account int64) map[uint32]uint32 {
	t.Helper()
	stored, err := store.ListSkins(ctx, account)
	if err != nil {
		t.Fatalf("ListSkins: %v", err)
	}
	out := make(map[uint32]uint32, len(stored))
	for _, skin := range stored {
		out[skin.SourceTemplate] = skin.SkinKey
	}
	return out
}

func mustDevelopmentAccount(t *testing.T, ctx context.Context, store *Store, name string) int64 {
	t.Helper()
	account, err := store.DevelopmentAccount(ctx, name)
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	return account
}

// 第一半：批量插入 N 条 → 返回 N，并且每条的皮肤号都对应上了。
func TestUnlockSkinsBatchInsertsEveryRow(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	account := mustDevelopmentAccount(t, ctx, store, "unlock-skins-batch")

	batch := skinBatch(1001, 1002, 1003, 1004, 1005)
	added, err := store.UnlockSkins(ctx, account, batch)
	if err != nil {
		t.Fatalf("UnlockSkins: %v", err)
	}
	if added != len(batch) {
		t.Fatalf("UnlockSkins added %d rows, want %d", added, len(batch))
	}

	got := skinKeysByTemplate(t, ctx, store, account)
	if len(got) != len(batch) {
		t.Fatalf("stored %d rows, want %d: %v", len(got), len(batch), got)
	}
	for _, skin := range batch {
		if got[skin.SourceTemplate] != skin.SkinKey {
			t.Errorf("template %d stored skin key %d, want %d",
				skin.SourceTemplate, got[skin.SourceTemplate], skin.SkinKey)
		}
	}
}

// 第二半：同一批重放返回 0；换皮肤号重放同一模板仍返回 0，并**保留第一次**的值
// （ON CONFLICT DO NOTHING 不覆盖 ⇒ 全解锁脚本可以在每次建号时无脑重跑）。
func TestUnlockSkinsReplayIsIdempotent(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	account := mustDevelopmentAccount(t, ctx, store, "unlock-skins-replay")

	first := skinBatch(2001, 2002, 2003) // 皮肤号 40000/40001/40002
	if added, err := store.UnlockSkins(ctx, account, first); err != nil || added != 3 {
		t.Fatalf("first batch added %d rows (err %v), want 3/nil", added, err)
	}
	if added, err := store.UnlockSkins(ctx, account, first); err != nil || added != 0 {
		t.Fatalf("replayed batch added %d rows (err %v), want 0/nil", added, err)
	}

	// 同一批模板 + 一个新模板：只有新模板算新增。
	replay := []AccountSkin{
		{SourceTemplate: 2001, SkinKey: 99991},
		{SourceTemplate: 2002, SkinKey: 99992},
		{SourceTemplate: 2004, SkinKey: 99994},
	}
	added, err := store.UnlockSkins(ctx, account, replay)
	if err != nil {
		t.Fatalf("UnlockSkins replay: %v", err)
	}
	if added != 1 {
		t.Fatalf("replay added %d rows, want 1 (only template 2004 is new)", added)
	}

	got := skinKeysByTemplate(t, ctx, store, account)
	if len(got) != 4 {
		t.Fatalf("stored %d rows, want 4: %v", len(got), got)
	}
	for template, want := range map[uint32]uint32{2001: 40000, 2002: 40001, 2003: 40002, 2004: 99994} {
		if got[template] != want {
			t.Errorf("template %d stored skin key %d, want %d (DO NOTHING must keep the first)", template, got[template], want)
		}
	}
}

// 同一批里的重复条目：主键含 source_template，所以数据库只有一行，返回值也必须是
// 1（按模板计），否则调用方会以为多写了一行。
func TestUnlockSkinsCountsTemplatesNotEntries(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	account := mustDevelopmentAccount(t, ctx, store, "unlock-skins-duplicates")

	batch := []AccountSkin{
		{SourceTemplate: 3001, SkinKey: 40001},
		{SourceTemplate: 3002, SkinKey: 40002},
		{SourceTemplate: 3001, SkinKey: 40099}, // 同模板重复：先到先得，第二次不写
	}
	added, err := store.UnlockSkins(ctx, account, batch)
	if err != nil {
		t.Fatalf("UnlockSkins: %v", err)
	}
	if added != 2 {
		t.Fatalf("batch with a duplicate template added %d rows, want 2", added)
	}
	got := skinKeysByTemplate(t, ctx, store, account)
	if len(got) != 2 {
		t.Fatalf("stored %d rows, want 2: %v", len(got), got)
	}
	if got[3001] != 40001 {
		t.Errorf("template 3001 stored skin key %d, want the first one (40001)", got[3001])
	}
}

// 非法条目：整批拒绝、报错带上出错的模板号，而且**一行都不写**（合法的那两条也不行）。
func TestUnlockSkinsRejectsInvalidBatchWithoutWriting(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	account := mustDevelopmentAccount(t, ctx, store, "unlock-skins-invalid")

	cases := []struct {
		name    string
		bad     AccountSkin
		wantErr string
	}{
		{"zero template", AccountSkin{SourceTemplate: 0, SkinKey: 40000}, "template=0"},
		{"zero skin key", AccountSkin{SourceTemplate: 2002, SkinKey: 0}, "template=2002"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			batch := []AccountSkin{
				{SourceTemplate: 2001, SkinKey: 40001},
				tc.bad,
				{SourceTemplate: 2003, SkinKey: 40003},
			}
			added, err := store.UnlockSkins(ctx, account, batch)
			if err == nil {
				t.Fatalf("UnlockSkins accepted %+v", tc.bad)
			}
			if added != 0 {
				t.Errorf("a rejected batch reported %d added rows, want 0", added)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not name the offending template (want %q)", err, tc.wantErr)
			}
		})
	}
	if got := skinKeysByTemplate(t, ctx, store, account); len(got) != 0 {
		t.Fatalf("a rejected batch wrote %d rows: %v", len(got), got)
	}
}

// 账号 0：同样整批拒绝（连空批之外的任何批次都不该落到某个账号名下）。
func TestUnlockSkinsRejectsZeroAccount(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	if added, err := store.UnlockSkins(ctx, 0, skinBatch(4001)); err == nil {
		t.Fatalf("UnlockSkins accepted account 0 and reported %d added rows", added)
	}
}

// 空批 / nil：0, nil，不改盘（账号 0 也要直接放行，因为根本没写东西）。
func TestUnlockSkinsEmptyBatchIsANoOp(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	account := mustDevelopmentAccount(t, ctx, store, "unlock-skins-empty")

	for _, batch := range [][]AccountSkin{nil, {}} {
		added, err := store.UnlockSkins(ctx, account, batch)
		if added != 0 || err != nil {
			t.Fatalf("empty batch returned %d/%v, want 0/nil", added, err)
		}
	}
	if added, err := store.UnlockSkins(ctx, 0, nil); added != 0 || err != nil {
		t.Fatalf("empty batch on account 0 returned %d/%v, want 0/nil", added, err)
	}
	if got := skinKeysByTemplate(t, ctx, store, account); len(got) != 0 {
		t.Fatalf("an empty batch wrote %d rows: %v", len(got), got)
	}
}
