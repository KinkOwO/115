package servermod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModDir 在 modsDir 下造一个带 mod.json 的 mod 目录，返回该目录。
func writeModDir(t *testing.T, modsDir, id, modJSON string) string {
	t.Helper()
	dir := filepath.Join(modsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mod.json"), []byte(modJSON), 0o644); err != nil {
		t.Fatalf("写 mod.json 失败：%v", err)
	}
	return dir
}

func modJSONWithHooks(id string, hooks ...string) string {
	var b strings.Builder
	b.WriteString(`{"schema":2,"id":"` + id + `","layers":{"server":{"package":".","hooks":[`)
	for i, h := range hooks {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"name":"` + h + `"}`)
	}
	b.WriteString(`]}}}`)
	return b.String()
}

func TestRegisteredHooksCoversEveryRegistrar(t *testing.T) {
	reset(t)
	RegisterBoot("probe.mod", func(*BootContext) error { return nil })
	RegisterConsole("probe.mod", func(ConsoleCommand) (bool, error) { return false, nil })
	RegisterResponse("probe.mod", func(string, uint16, []byte) {})
	RegisterRequest("probe.mod", func(*RequestContext) (bool, error) { return false, nil })
	if err := RegisterRewardScript("probe.mod", "probe-hooks.lua", []byte("on(\"level_up\", function(ctx) end)")); err != nil {
		t.Fatalf("登记奖励脚本失败：%v", err)
	}
	got := strings.Join(RegisteredHooks("probe.mod"), ",")
	want := "console.command,protocol.request,protocol.response,reward.script,server.boot"
	if got != want {
		t.Fatalf("RegisteredHooks = %q，期望 %q", got, want)
	}
	// 别的 mod 不该被算进来。
	if other := RegisteredHooks("other.mod"); len(other) != 0 {
		t.Fatalf("未注册的 mod 应为空，得到 %v", other)
	}
}

func TestCheckDeclarationsCleanWhenDeclaredMatchesRegistered(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "probe.mod", modJSONWithHooks("probe.mod", HookBoot, HookConsoleCommand))
	RegisterBoot("probe.mod", func(*BootContext) error { return nil })
	RegisterConsole("probe.mod", func(ConsoleCommand) (bool, error) { return false, nil })

	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("声明与注册一致时不该有问题：%v", problems)
	}
}

// TestCheckDeclarationsCatchesDeclaredButNotRegistered 钉住本机制存在的理由：
// mod.json 声明了 server.boot，Go 里没注册 —— 过去完全静默，现在必须被抓出来。
func TestCheckDeclarationsCatchesDeclaredButNotRegistered(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "silent.mod", modJSONWithHooks("silent.mod", HookBoot))

	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("应报出 1 个问题，得到 %v", problems)
	}
	p := problems[0]
	if p.ModID != "silent.mod" {
		t.Fatalf("ModID = %q", p.ModID)
	}
	if strings.Join(p.Missing, ",") != HookBoot {
		t.Fatalf("Missing = %v，期望 %s", p.Missing, HookBoot)
	}
	if len(p.Extra) != 0 {
		t.Fatalf("不该有 Extra：%v", p.Extra)
	}
	if !strings.Contains(p.String(), "声明了却没注册") {
		t.Fatalf("描述不可读：%q", p.String())
	}
}

func TestCheckDeclarationsCatchesRegisteredButNotDeclared(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "understated.mod", modJSONWithHooks("understated.mod", HookBoot))
	RegisterBoot("understated.mod", func(*BootContext) error { return nil })
	RegisterRequest("understated.mod", func(*RequestContext) (bool, error) { return false, nil })

	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("应报出 1 个问题，得到 %v", problems)
	}
	if strings.Join(problems[0].Extra, ",") != HookProtocolRequest {
		t.Fatalf("Extra = %v，期望 %s", problems[0].Extra, HookProtocolRequest)
	}
	if len(problems[0].Missing) != 0 {
		t.Fatalf("不该有 Missing：%v", problems[0].Missing)
	}
}

// TestCheckDeclarationsSkipsDisabledMod 钉住"停用不是故障"：
// 被 enabled.json 禁用的 mod 本来就不该注册任何钩子。
func TestCheckDeclarationsSkipsDisabledMod(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "off.mod", modJSONWithHooks("off.mod", HookBoot))
	if err := os.WriteFile(filepath.Join(mods, "enabled.json"),
		[]byte(`{"schema":1,"disabled":["off.mod"]}`), 0o644); err != nil {
		t.Fatalf("写 enabled.json 失败：%v", err)
	}
	LoadEnabledList(mods)
	if Enabled("off.mod") {
		t.Fatal("off.mod 应处于禁用状态")
	}

	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("禁用 mod 不该被报成问题：%v", problems)
	}
}

// TestCheckDeclarationsIgnoresNonModDirs 钉住识别判据与 modkit 一致：
// 只有带 mod.json 的目录才是服务端源码 mod（mods/scripts/ 这类目录必须被无视）。
func TestCheckDeclarationsIgnoresNonModDirs(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	for _, name := range []string{"scripts", "some-分享", "examples"} {
		if err := os.MkdirAll(filepath.Join(mods, name), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
	}
	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("没有 mod.json 的目录不该被当成 mod：%v", problems)
	}
}

// TestRewardScriptSatisfiesRewardScriptHook 钉住特殊映射：
// reward.script 没有独立注册函数，由 RegisterRewardScript 满足。
func TestRewardScriptSatisfiesRewardScriptHook(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "rules.mod", modJSONWithHooks("rules.mod", HookRewardScript))
	if err := RegisterRewardScript("rules.mod", "probe-rules.lua", []byte("on(\"level_up\", function(ctx) end)")); err != nil {
		t.Fatalf("登记奖励脚本失败：%v", err)
	}
	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("登记了规则脚本就不该报 reward.script 缺失：%v", problems)
	}
}

func TestCheckDeclarationsRejectsMalformedManifest(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "broken.mod", `{"schema":2,"id":`)
	if _, err := CheckDeclarations(mods); err == nil {
		t.Fatal("mod.json 解析不了时必须报错，而不是当成没问题")
	}
}

func TestCheckDeclarationsMissingDirIsNotAnError(t *testing.T) {
	reset(t)
	problems, err := CheckDeclarations(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("目录不存在不该报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("目录不存在不该有问题：%v", problems)
	}
}
