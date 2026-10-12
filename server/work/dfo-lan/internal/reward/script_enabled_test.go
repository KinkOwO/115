package reward

import (
	"context"
	"testing"
	"testing/fstest"
)

// TestScriptEnabledSkipsDisabledModScript 钉住 mod 停用方向在奖励管线里的落点：
// 脚本在启动时已经编进唯一的 LState，派发时必须按当前名单再问一次，
// 否则"关掉一个 mod"要等到下次重启才停止发奖。
func TestScriptEnabledSkipsDisabledModScript(t *testing.T) {
	scripts := fstest.MapFS{
		"builtin.lua":   &fstest.MapFile{Data: []byte(`on("level_up", function(ctx) grant_item(1001, 1) end)`)},
		"mod.kit.lua":   &fstest.MapFile{Data: []byte(`on("level_up", function(ctx) grant_item(2002, 1) end)`)},
		"mod.other.lua": &fstest.MapFile{Data: []byte(`on("level_up", function(ctx) grant_item(3003, 1) end)`)},
	}
	var granted []uint32
	grant := func(ctx context.Context, r Recipient, key string, items []ItemGrant) error {
		for _, it := range items {
			granted = append(granted, it.ID)
		}
		return nil
	}
	newSvc := func(pred func(string) bool) *Service {
		t.Helper()
		svc, err := New(Options{Scripts: scripts, Grant: grant, ScriptEnabled: pred})
		if err != nil {
			t.Fatalf("New 失败：%v", err)
		}
		return svc
	}
	who := Recipient{AccountID: 1, CharacterID: 1}

	// 没有谓词 = 全部照跑（内置规则集与现存档位的行为不能被这次改动改变）。
	granted = nil
	newSvc(nil).LevelUp(context.Background(), who)
	if len(granted) != 3 {
		t.Fatalf("不设门禁时三份脚本都该发奖，得到 %v", granted)
	}

	// 关掉 mod.kit.lua：它的规则一条都不该执行，别人的不受影响。
	granted = nil
	newSvc(func(name string) bool { return name != "mod.kit.lua" }).LevelUp(context.Background(), who)
	if hasItem(granted, 2002) {
		t.Fatalf("停用的脚本仍在发奖：%v", granted)
	}
	if !hasItem(granted, 1001) || !hasItem(granted, 3003) {
		t.Fatalf("停用一份脚本不该牵连其它脚本：%v", granted)
	}

	// 谓词恒假 = 一条都不发（关掉整个 mod 库时的形状）。
	granted = nil
	newSvc(func(string) bool { return false }).LevelUp(context.Background(), who)
	if len(granted) != 0 {
		t.Fatalf("谓词恒假时不该有任何发放：%v", granted)
	}
}

func hasItem(ids []uint32, want uint32) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
