package reward

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type grantCall struct {
	key   string
	items []ItemGrant
}

type mailCall struct {
	key  string
	mail MailReward
}

func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

func TestLevelUpGrantsAndMails(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "level.lua", `
on("level_up", function(ctx)
  if ctx.level == 10 then
    grant_item(10000100, 5)
    send_mail("升级奖励", "恭喜达到 10 级，奖励已发放。")
  end
end)
`)

	var grants []grantCall
	var mails []mailCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
		Mail: func(_ context.Context, _ Recipient, key string, mail MailReward) error {
			mails = append(mails, mailCall{key: key, mail: mail})
			return nil
		},
	})
	require.NoError(t, err)

	s.LevelUp(context.Background(), Recipient{AccountID: 3, CharacterID: 7, Name: "hero", Level: 10})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 10000100, Count: 5}}, grants[0].items)
	require.Len(t, mails, 1)
	require.Equal(t, "升级奖励", mails[0].mail.Subject)
	require.Empty(t, mails[0].mail.Attachments)

	// Keys are stable, distinct per capability and carry the level.
	require.Equal(t, "reward:level_up:level.lua:10:item", grants[0].key)
	require.Equal(t, "reward:level_up:level.lua:10:mail", mails[0].key)

	// A non-matching level must not invoke either capability.
	grants, mails = nil, nil
	s.LevelUp(context.Background(), Recipient{Level: 11})
	require.Empty(t, grants)
	require.Empty(t, mails)
}

func TestQuestCompleteHonorsQuestID(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "quest.lua", `
on("quest_complete", function(ctx)
  if ctx.quest_id == 3149 then
    grant_item(10000100, 1)
  end
end)
`)

	var grants []grantCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
	})
	require.NoError(t, err)

	s.QuestComplete(context.Background(), Recipient{CharacterID: 7, Level: 40}, 3149)
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 10000100, Count: 1}}, grants[0].items)
	require.Equal(t, "reward:quest_complete:quest.lua:3149:item", grants[0].key)

	grants = nil
	s.QuestComplete(context.Background(), Recipient{Level: 40}, 123)
	require.Empty(t, grants)
}

func TestCharacterCreateGrants(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "char.lua", `
on("character_create", function(ctx)
  grant_item(10418035, 1000)
end)
`)

	var grants []grantCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
	})
	require.NoError(t, err)

	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 10418035, Count: 1000}}, grants[0].items)
	// The key carries the script filename and the character id.
	require.Equal(t, "reward:character_create:char.lua:7:item", grants[0].key)
}

// TestCharacterCreateEventLoads pins that on("character_create", ...) passes
// the knownEvents gate; without the registration New would reject the script.
func TestCharacterCreateEventLoads(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "char.lua", `
on("character_create", function(ctx) grant_item(10418035, 1000) end)
`)
	_, err := New(Options{Scripts: os.DirFS(dir)})
	require.NoError(t, err)
}

// TestCharacterCreateAcceptsGoldGrant pins that grant_item(0, ...) is accepted
// and reaches the Grant callback: id 0 is the character gold stack convention.
func TestCharacterCreateAcceptsGoldGrant(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "char.lua", `
on("character_create", function(ctx)
  grant_item(0, 10000000)
end)
`)

	var grants []grantCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
	})
	require.NoError(t, err)

	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 0, Count: 10000000}}, grants[0].items)
}

// TestCharacterCreateGrantsCera pins the grant_cera capability and its key.
func TestCharacterCreateGrantsCera(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "char.lua", `
on("character_create", function(ctx)
  grant_cera(100000)
end)
`)

	var amount uint64
	var key string
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Cera: func(_ context.Context, _ Recipient, k string, a uint64) error {
			key, amount = k, a
			return nil
		},
	})
	require.NoError(t, err)
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 7})
	require.Equal(t, uint64(100000), amount)
	require.Equal(t, "reward:character_create:char.lua:7:cera", key)
}

func TestGrantCeraRejectsNonPositive(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "char.lua", `
on("character_create", function(ctx)
  grant_cera(0)
end)
`)

	var logged []string
	var amount uint64
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Cera: func(_ context.Context, _ Recipient, _ string, a uint64) error {
			amount = a
			return nil
		},
		Log: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) },
	})
	require.NoError(t, err)
	require.NotPanics(t, func() { s.CharacterCreate(context.Background(), Recipient{CharacterID: 7}) })
	require.Zero(t, amount)
	require.Len(t, logged, 1)
	require.Contains(t, logged[0], "char.lua")
}

// TestCtxCarriesProfessionOnlyWhenKnown pins the两个职业字段的曝光口径：
//   - 知道职业时（character_create）ctx.profession / ctx.advancement 是数，
//     哪怕是 0（基础职业 0 = 鬼剑士是合法值）；
//   - 不知道时不写进 ctx（Lua 侧是 nil），规则才能用 `if not ctx.profession then return end`
//     干净跳过 —— 而不是把 0 当成"没职业"。
func TestCtxCarriesProfessionOnlyWhenKnown(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "job.lua", `
on("character_create", function(ctx)
  if ctx.profession == nil or ctx.advancement == nil then
    grant_item(1, 1)          -- 拿不到：发 1 号标记
    return
  end
  grant_item(ctx.profession + 100, ctx.advancement + 1)
end)
`)

	var grants []grantCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
	})
	require.NoError(t, err)

	// 知道职业（0/0 也是合法值）：规则读到的是数。
	s.CharacterCreate(context.Background(), Recipient{
		CharacterID: 7, Profession: 0, Advancement: 0, HasProfession: true,
	})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 100, Count: 1}}, grants[0].items)

	grants = nil
	s.CharacterCreate(context.Background(), Recipient{
		CharacterID: 8, Profession: 5, Advancement: 5, HasProfession: true,
	})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 105, Count: 6}}, grants[0].items)

	// 不知道（例如 level_up 那条路）：字段缺席。
	grants = nil
	s.CharacterCreate(context.Background(), Recipient{CharacterID: 9})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 1, Count: 1}}, grants[0].items)
}

func TestNewRejectsSyntaxError(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "broken.lua", `on("level_up", function(ctx)`+"\n")
	_, err := New(Options{Scripts: os.DirFS(dir)})
	require.Error(t, err)
}

func TestNewRejectsUnknownEvent(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "typo.lua", `
on("levelup", function(ctx) end)
`)
	_, err := New(Options{Scripts: os.DirFS(dir)})
	require.Error(t, err)
}

func TestNewRejectsNoScripts(t *testing.T) {
	_, err := New(Options{Scripts: os.DirFS(t.TempDir())})
	require.Error(t, err)
}

// TestBundledScriptsLoad pins that the compiled-in rule set is loadable with no
// external path, which is the shipped configuration.
func TestBundledScriptsLoad(t *testing.T) {
	s, err := New(Options{})
	require.NoError(t, err)
	require.NotNil(t, s)
	s.LevelUp(context.Background(), Recipient{Level: 1})
}

func TestRuntimeErrorIsSkippedAndOtherScriptsRun(t *testing.T) {
	dir := t.TempDir()
	// Sorted by filename: the bad script is dispatched first.
	writeScript(t, dir, "a_bad.lua", `
on("level_up", function(ctx)
  error("boom")
end)
`)
	writeScript(t, dir, "b_good.lua", `
on("level_up", function(ctx)
  if ctx.level == 10 then
    grant_item(10000100, 2)
  end
end)
`)

	var grants []grantCall
	var logged []string
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
		Log: func(format string, args ...any) {
			logged = append(logged, fmt.Sprintf(format, args...))
		},
	})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		s.LevelUp(context.Background(), Recipient{Level: 10})
	})
	require.Len(t, grants, 1)
	require.Equal(t, []ItemGrant{{ID: 10000100, Count: 2}}, grants[0].items)
	require.Len(t, logged, 1)
	require.Contains(t, logged[0], "a_bad.lua")
}

func TestNilCallbacksAreSkippedAndLogged(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "level.lua", `
on("level_up", function(ctx)
  grant_item(10000100, 1)
  send_mail("subject", "body")
end)
`)

	var logged []string
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Log: func(format string, args ...any) {
			logged = append(logged, fmt.Sprintf(format, args...))
		},
	})
	require.NoError(t, err)
	require.NotPanics(t, func() {
		s.LevelUp(context.Background(), Recipient{Level: 10})
	})
	require.Len(t, logged, 2)
	for _, line := range logged {
		require.Contains(t, line, "not configured")
	}
}

func TestSharedStateKeepsEachScriptHandler(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "a.lua", `
on("level_up", function(ctx)
  grant_item(10000100, 1)
end)
`)
	writeScript(t, dir, "b.lua", `
on("level_up", function(ctx)
  grant_item(10000100, 2)
end)
`)

	var grants []grantCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Grant: func(_ context.Context, _ Recipient, key string, items []ItemGrant) error {
			grants = append(grants, grantCall{key: key, items: items})
			return nil
		},
	})
	require.NoError(t, err)
	s.LevelUp(context.Background(), Recipient{Level: 10})
	require.Len(t, grants, 2)
	require.Equal(t, []ItemGrant{{ID: 10000100, Count: 1}}, grants[0].items)
	require.Equal(t, []ItemGrant{{ID: 10000100, Count: 2}}, grants[1].items)
}

func TestSendMailSupportsMultipleAttachments(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "mail.lua", `
on("level_up", function(ctx)
  if ctx.level == 20 then
    send_mail("20级奖励", "奖励见附件。", { {id = 111, count = 1}, {id = 222, count = 3} })
  end
end)
`)

	var mails []mailCall
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Mail: func(_ context.Context, _ Recipient, key string, mail MailReward) error {
			mails = append(mails, mailCall{key: key, mail: mail})
			return nil
		},
	})
	require.NoError(t, err)
	s.LevelUp(context.Background(), Recipient{Level: 20})
	require.Len(t, mails, 1)
	require.Equal(t, []ItemGrant{{ID: 111, Count: 1}, {ID: 222, Count: 3}}, mails[0].mail.Attachments)

	// A non-matching level produces no mail.
	mails = nil
	s.LevelUp(context.Background(), Recipient{Level: 21})
	require.Empty(t, mails)
}

func TestSendMailRejectsBadAttachmentEntry(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "bad.lua", `
on("level_up", function(ctx)
  send_mail("s", "b", { {id = 0, count = 1} })
end)
`)

	var mails []mailCall
	var logged []string
	s, err := New(Options{
		Scripts: os.DirFS(dir),
		Mail: func(_ context.Context, _ Recipient, key string, mail MailReward) error {
			mails = append(mails, mailCall{key: key, mail: mail})
			return nil
		},
		Log: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) },
	})
	require.NoError(t, err)
	require.NotPanics(t, func() { s.LevelUp(context.Background(), Recipient{Level: 1}) })
	require.Empty(t, mails)
	require.Len(t, logged, 1)
	require.Contains(t, logged[0], "bad.lua")
}
