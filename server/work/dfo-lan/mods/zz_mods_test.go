//go:build modtest

package mods

// 本文件带 modtest 构建标签：它断言"**已安装的**服务端 mod 真的被接上了"，
// 所以只在**装了 mod 之后**才有意义。干净的树上 RegisterMods() 什么都不注册，
// 这些断言必然失败——那是正确行为，不是缺陷。
//
// 用法（装好 mod 后自证整条链真的通了）：
//
//	go test -tags modtest ./mods/ -count=1 -v
//
// 它验证的就是服务端启动时走的顺序：
//
//	servermod.LoadEnabledList(mods/)   ← 读启用/禁用清单
//	mods.RegisterMods()                ← 各 mod 的 Register()
//	servermod.Boot()                   ← 自检
//
// 最有价值的一条是 TestGiveawayRuleCompilesAndFires：它把 mod 的 Lua 规则真的
// 交给奖励管线跑一遍并触发 character_create，断言产出了带装备附件的邮件。

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"dfolan/internal/reward"
	"dfolan/internal/servermod"
)

// TestRegisterModsWiresInstalledMods 验证编译期注册这条链真的通了。
func TestRegisterModsWiresInstalledMods(t *testing.T) {
	servermod.LoadEnabledList(".")
	RegisterMods()

	if ids := servermod.Registered(); len(ids) == 0 {
		t.Fatal("RegisterMods() 之后没有任何 mod 被登记：生成清单与 mod 的 Register() 没接上")
	}
	desc := servermod.Description()
	t.Logf("宿主描述：%s", desc)
	if !strings.Contains(desc, "giveaway.random-equipment") {
		t.Fatalf("随机装备 mod 未被登记：%s", desc)
	}
}

// TestGiveawayRegistersRewardScript 验证 mod 把自己的 Lua 规则交进了奖励管线。
func TestGiveawayRegistersRewardScript(t *testing.T) {
	servermod.LoadEnabledList(".")
	RegisterMods()

	scripts := servermod.RewardScripts()
	if len(scripts) == 0 {
		t.Fatal("没有任何 mod 规则脚本被登记")
	}
	var found bool
	for _, s := range scripts {
		t.Logf("规则脚本：%s ← mod %s（%d 字节）", s.Name, s.ModID, len(s.Body))
		if s.ModID == "giveaway.random-equipment" {
			found = true
			if !strings.Contains(string(s.Body), "character_create") {
				t.Fatalf("规则脚本没有注册 character_create 事件：\n%s", s.Body)
			}
			if !strings.Contains(string(s.Body), "send_mail") {
				t.Fatalf("规则脚本没有用 send_mail 投递装备：\n%s", s.Body)
			}
		}
	}
	if !found {
		t.Fatal("随机装备 mod 的规则脚本没有被登记")
	}
}

// TestGiveawayRuleCompilesAndFires 是本组最有价值的断言：
// 把 mod 的 Lua 规则交给奖励管线真跑一遍（与内嵌规则集合同一个 fs.FS），
// 然后用假收件人触发 character_create，断言产出了一封带装备附件的邮件。
func TestGiveawayRuleCompilesAndFires(t *testing.T) {
	servermod.LoadEnabledList(".")
	RegisterMods()

	composed := composeFS(reward.BundledScripts(), servermod.NewRewardScriptFS(t.TempDir()))

	type mailCall struct {
		subject string
		assets  []reward.ItemGrant
	}
	var got []mailCall
	var granted []reward.ItemGrant

	svc, err := reward.New(reward.Options{
		Scripts: composed,
		Grant: func(_ context.Context, _ reward.Recipient, _ string, items []reward.ItemGrant) error {
			granted = append(granted, items...)
			return nil
		},
		Mail: func(_ context.Context, _ reward.Recipient, _ string, m reward.MailReward) error {
			got = append(got, mailCall{subject: m.Subject, assets: m.Attachments})
			return nil
		},
		Cera: func(context.Context, reward.Recipient, string, uint64) error { return nil },
	})
	if err != nil {
		t.Fatalf("奖励管线加载 mod 规则失败（脚本语法或 on() 用法有错）：%v", err)
	}

	svc.CharacterCreate(context.Background(), reward.Recipient{
		AccountID: 1, CharacterID: 1, Name: "newbie",
	})

	var giveaway *mailCall
	for i := range got {
		if strings.Contains(got[i].subject, "mod 生效验证") {
			giveaway = &got[i]
			break
		}
	}
	if giveaway == nil {
		t.Fatalf("character_create 没有触发 mod 的邮件；本次收到的邮件：%+v", got)
	}
	// 金币：id 0 是角色金币堆，必定成功、不依赖内容模板
	var gold uint32
	for _, g := range granted {
		if g.ID == 0 {
			gold += g.Count
		}
	}
	if gold == 0 {
		t.Fatalf("没有发到金币（grant_item(0, N)），本次发放：%+v", granted)
	}
	t.Logf("mod 生效：邮件 subject=%q；金币=%d", giveaway.subject, gold)
}

// TestDisabledModDoesNotRegister 验证启用/禁用门禁真的起作用。
func TestDisabledModDoesNotRegister(t *testing.T) {
	dir := t.TempDir()
	if err := servermod.SetModEnabled(dir, "giveaway.random-equipment", false, "test"); err != nil {
		t.Fatal(err)
	}
	servermod.LoadEnabledList(dir)
	if servermod.Enabled("giveaway.random-equipment") {
		t.Fatal("禁用名单写好后 Enabled() 仍为 true")
	}
}

// composeFS 依次在 first、second 里找文件（与主程序的合成 FS 同语义）。
func composeFS(first, second fs.FS) fs.FS { return &composed{first: first, second: second} }

type composed struct{ first, second fs.FS }

func (c *composed) Open(name string) (fs.File, error) {
	if f, err := c.first.Open(name); err == nil {
		return f, nil
	}
	return c.second.Open(name)
}

// ReadDir 与主程序的 compositeScriptFS 同语义：fs.Glob 靠它列脚本，
// 少了它 mod 规则会被静默忽略（实测踩过）。
func (c *composed) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." && name != "" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	seen := map[string]bool{}
	var out []fs.DirEntry
	collect := func(src fs.FS) {
		rd, ok := src.(fs.ReadDirFS)
		if !ok {
			return
		}
		entries, err := rd.ReadDir(".")
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			out = append(out, e)
		}
	}
	collect(c.first)
	collect(c.second)
	return out, nil
}

// TestGiveawayBootHookConfirmsPipeline 验证 mod 的启动自检能看穿
// "规则脚本有没有进奖励管线" —— 顺序对了就通过；顺序反了 boot 会返回 error。
func TestGiveawayBootHookConfirmsPipeline(t *testing.T) {
	servermod.LoadEnabledList(".")
	RegisterMods()
	if err := servermod.Boot(&servermod.BootContext{Version: "modtest", ChannelCount: 0}); err != nil {
		t.Fatalf("mod 的启动自检失败（多半是规则脚本没进奖励管线）：%v", err)
	}
}
