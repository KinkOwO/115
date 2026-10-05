package servermod

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestRewardScriptFSSeesLateRegistration 是 2026-10-06 02:22 现场的回归测试。
//
// 现场：mod 的奖励规则脚本"已装载/已登记"，但新角色创建后收不到邮件。
// 根因是**构造顺序**：奖励管线在 prepareRuntime 里构造，而 mods.RegisterMods()
// 在那之后才跑；如果 modScriptFS 在构造时把注册表**快照**下来，
// 快照就是空的，之后登记也进不去 —— 规则静默失效。
//
// 本测试固定住正确语义：**视图必须能看到构造之后才登记的脚本**。
func TestRewardScriptFSSeesLateRegistration(t *testing.T) {
	// 1) 先构造视图（模拟"奖励管线先构造"）
	f := NewRewardScriptFS(t.TempDir())

	// 2) 构造之后才登记（模拟"mod 稍后 Register()"）
	const name = "zz-late-registration-test.lua"
	const modID = "test.late.registration"
	body := []byte(`on("character_create", function(ctx) send_mail("t","b") end)` + "\n")
	if err := RegisterRewardScript(modID, name, body); err != nil {
		t.Fatalf("登记失败：%v", err)
	}

	// 3) 视图必须能看到它 —— fs.Glob 走的是 ReadDir
	names, err := fs.Glob(f, "*.lua")
	if err != nil {
		t.Fatalf("glob 失败：%v", err)
	}
	found := false
	for _, n := range names {
		if n == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("构造后才登记的脚本没有出现在视图里（%v）："+
			"modScriptFS 又在做快照了 —— 这会让 mod 规则静默失效", names)
	}

	// 4) 内容也要能读出来
	got, err := fs.ReadFile(f, name)
	if err != nil {
		t.Fatalf("读脚本失败：%v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("脚本内容不一致：got %q want %q", got, body)
	}

	// 5) 清理：把测试脚本摘掉，避免影响同包其它测试
	scriptMu.Lock()
	for i, s := range modScripts {
		if s.Name == name {
			modScripts = append(modScripts[:i], modScripts[i+1:]...)
			break
		}
	}
	delete(registeredIDs, modID)
	scriptMu.Unlock()
}

// TestRewardScriptFSDiskOverridesMemory 固定"磁盘优先"的语义（便于就地调试规则）。
func TestRewardScriptFSDiskOverridesMemory(t *testing.T) {
	dir := t.TempDir()
	const name = "zz-disk-override-test.lua"
	if err := RegisterRewardScript("test.disk.override", name, []byte("-- memory\n")); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	t.Cleanup(func() {
		scriptMu.Lock()
		for i, s := range modScripts {
			if s.Name == name {
				modScripts = append(modScripts[:i], modScripts[i+1:]...)
				break
			}
		}
		delete(registeredIDs, "test.disk.override")
		scriptMu.Unlock()
	})

	f := NewRewardScriptFS(dir)
	if b, err := fs.ReadFile(f, name); err != nil || string(b) != "-- memory\n" {
		t.Fatalf("只有内存脚本时应读到内存内容：%q err=%v", b, err)
	}

	// 放一份同名磁盘脚本 → 磁盘优先
	if err := writeTestFile(dir, name, "-- disk\n"); err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(f, name)
	if err != nil {
		t.Fatalf("读磁盘脚本失败：%v", err)
	}
	if string(b) != "-- disk\n" {
		t.Fatalf("磁盘脚本应优先于内存脚本：got %q", b)
	}
}

// writeTestFile 写一份测试用的磁盘脚本。
func writeTestFile(dir, name, body string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
}
