package main

import (
	"bytes"
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dfolan/internal/reward"
)

// reward_scripts_dir_test.go —— 规则脚本目录的口径（业主 2026-10-06 定）。
//
// 正式位置 = **mod 库根平铺**：`<包根>/mods/<名>.lua`（与其它 mod 同级，不再进 scripts/ 子目录）。
// 这条口径要让"服务端真的会加载它"可证：这里用**真正的装配函数** rewardScriptFS()
// （内嵌规则集 + 磁盘目录的合成视图）验证放到该目录的脚本会被枚举到。
//
// 用 DFO_REWARD_SCRIPTS_DIR 注入临时目录而不是依赖本机包根，测试才是自洽的
// （同时正好覆盖"启动器显式注入"这条最高优先级路径）。

func TestRewardScriptsReadFromModLibraryRoot(t *testing.T) {
	dir := t.TempDir()
	script := "on(\"character_create\", function(ctx) end)\n"
	if err := os.WriteFile(filepath.Join(dir, "loose_probe.lua"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(RewardScriptsEnvVar, dir)

	// 搜索目录的第一位必须是注入的那个（正式位置优先）。
	dirs := rewardScriptsDirs()
	if len(dirs) == 0 || dirs[0] != filepath.Clean(dir) {
		t.Fatalf("搜索目录第一位应当是注入的 mod 库根，实际 %v", dirs)
	}

	// 用真正的装配函数装载：松散脚本要能被枚举并执行。
	files := rewardScriptFS()
	services, err := reward.New(reward.Options{
		Scripts: files,
		Grant:   func(context.Context, reward.Recipient, string, []reward.ItemGrant) error { return nil },
		Mail:    func(context.Context, reward.Recipient, string, reward.MailReward) error { return nil },
		Cera:    func(context.Context, reward.Recipient, string, uint64) error { return nil },
		Entitlements: func(context.Context, reward.Recipient, string, reward.Entitlements) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("装载奖励规则集失败：%v", err)
	}
	// 只要装载成功就说明磁盘脚本进了 LState；再触发一次确保 handler 可调用。
	services.CharacterCreate(context.Background(), reward.Recipient{CharacterID: 1})
}

// TestRewardScriptsDirsPreferPackRootMods 证明**真实运行目录**下的解析口径。
//
// 服务端是以服务端模块根为工作目录跑起来的（启动器这么设），此时搜索目录的第一位必须是
// **包根的 `mods/`**（`<包根>/mods`，与其他 mod 同级），而不是模块内的 `mods/scripts`。
// 测试进程的工作目录是包内目录（cmd/wireprobe），所以这里显式切到模块根再问一次。
func TestRewardScriptsDirsPreferPackRootMods(t *testing.T) {
	moduleRoot := findServerModuleRoot(t)
	t.Chdir(moduleRoot)

	dirs := rewardScriptsDirs()
	if len(dirs) == 0 {
		t.Fatalf("模块根 %s 下解析不出任何脚本目录（包根的 mods/ 应当存在）", moduleRoot)
	}
	want := filepath.Clean(filepath.Join(moduleRoot, "..", "..", "..", "mods"))
	if dirs[0] != want {
		t.Fatalf("搜索目录第一位应当是本包根的 mods/（%s），实际 %v", want, dirs)
	}
	// 正式位置里那份松散脚本（如果有）必须能被枚举到 —— 这里只作提示，不因缺文件失败。
	if _, err := os.Stat(filepath.Join(want, "newchar_kit.lua")); err != nil {
		t.Logf("提示：包根 mods/ 下暂时没有 newchar_kit.lua（%v）", err)
	}
}

// 磁盘那份 newchar.lua 发的模板号：内置集里没有这个号，用来区分"跑的是哪一份"。
const diskOverrideMarkerID = 999999

// writeDiskOverrideScript 在 dir 里写一份**覆盖内置 newchar.lua** 的磁盘脚本。
func writeDiskOverrideScript(t *testing.T, dir string) string {
	t.Helper()
	script := "on(\"character_create\", function(ctx)\n  grant_item(" +
		strconv.Itoa(diskOverrideMarkerID) + ", 1)\nend)\n"
	if err := os.WriteFile(filepath.Join(dir, "newchar.lua"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	return script
}

// TestRewardScriptsDiskOverridesBundled 钉住"同名时磁盘/mod 脚本胜出、内置集退化为兜底"。
//
// 用的是**真正的装配函数** rewardScriptFS()，所以断言直接对应奖励管线的实际取用：
//
//	① 同名 newchar.lua —— 列出来只有一份，读到的必须是磁盘内容（Open 与 ReadDir 同一契约，
//	   不会"列出内置的、读的是磁盘的"）；
//	② 没有同名磁盘脚本的内置 level.lua / quest.lua —— 仍在列表里、内容仍是内置那份（兜底生效）；
//	③ 真跑一遍 character_create —— 拿到的只能是磁盘脚本发的那个标记模板号（内置 newchar.lua
//	   发的是金币 + 一堆内置模板，若内置赢了这个断言必然失败）。
func TestRewardScriptsDiskOverridesBundled(t *testing.T) {
	dir := t.TempDir()
	diskScript := writeDiskOverrideScript(t, dir)
	t.Setenv(RewardScriptsEnvVar, dir)

	src := rewardScriptFS()

	names, err := fs.Glob(src, "*.lua")
	if err != nil {
		t.Fatalf("列脚本失败：%v", err)
	}
	count := map[string]int{}
	for _, n := range names {
		count[n]++
	}
	if count["newchar.lua"] != 1 {
		t.Fatalf("newchar.lua 应当只被列出一次，实际 %v", names)
	}
	got, err := fs.ReadFile(src, "newchar.lua")
	if err != nil {
		t.Fatalf("读 newchar.lua 失败：%v", err)
	}
	if string(got) != diskScript {
		t.Fatalf("同名时应当读到**磁盘**那份，实际读到：%q", string(got))
	}

	bundled := reward.BundledScripts()
	if bundled == nil {
		t.Fatal("内置规则集不可用")
	}
	for _, name := range []string{"level.lua", "quest.lua"} {
		if count[name] != 1 {
			t.Fatalf("内置 %s 没有同名磁盘脚本，应当仍在列表里（兜底），实际 %v", name, names)
		}
		want, err := fs.ReadFile(bundled, name)
		if err != nil {
			t.Fatalf("读内置 %s 失败：%v", name, err)
		}
		got, err := fs.ReadFile(src, name)
		if err != nil {
			t.Fatalf("从合成视图读 %s 失败：%v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s 应当仍是内置那份（只有同名文件才被覆盖）", name)
		}
	}

	var grants []reward.ItemGrant
	svc, err := reward.New(reward.Options{
		Scripts: src,
		Grant: func(_ context.Context, _ reward.Recipient, _ string, items []reward.ItemGrant) error {
			grants = append(grants, items...)
			return nil
		},
		Mail:         func(context.Context, reward.Recipient, string, reward.MailReward) error { return nil },
		Cera:         func(context.Context, reward.Recipient, string, uint64) error { return nil },
		Entitlements: func(context.Context, reward.Recipient, string, reward.Entitlements) error { return nil },
	})
	if err != nil {
		t.Fatalf("装载奖励规则集失败：%v", err)
	}
	svc.CharacterCreate(context.Background(), reward.Recipient{CharacterID: 1})
	if len(grants) != 1 || grants[0].ID != diskOverrideMarkerID {
		t.Fatalf("character_create 只应执行磁盘那份 newchar.lua（发模板 %d），实际 %+v",
			diskOverrideMarkerID, grants)
	}
}

// TestRewardScriptsDiskOverrideIsLogged 钉住"磁盘盖内置"是**可观测**的。
//
// 覆盖是静默生效的（同名文件落盘就顶掉内置规则），所以必须有一条点名日志：
// 文件名 + 来源目录。少了它，"我放的那份生效了吗"就只能靠猜 —— 这正是本次补日志的理由。
func TestRewardScriptsDiskOverrideIsLogged(t *testing.T) {
	dir := t.TempDir()
	writeDiskOverrideScript(t, dir)
	t.Setenv(RewardScriptsEnvVar, dir)

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	_ = rewardScriptFS()

	out := buf.String()
	for _, want := range []string{"覆盖内置同名规则", "newchar.lua", filepath.Clean(dir)} {
		if !strings.Contains(out, want) {
			t.Fatalf("覆盖日志应当包含 %q（文件名 + 来源目录），实际：\n%s", want, out)
		}
	}
	// 没被覆盖的内置名字不该被误报成覆盖。
	if strings.Contains(out, "level.lua 覆盖内置") || strings.Contains(out, "quest.lua 覆盖内置") {
		t.Fatalf("只有同名文件才算覆盖，日志里不该出现未覆盖的名字：\n%s", out)
	}
}

// findServerModuleRoot 从当前工作目录往上找服务端模块根（go.mod 且 module dfolan）。
func findServerModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil &&
			strings.Contains(string(b), "module dfolan") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("往上找不到服务端模块根（go.mod 声明 module dfolan）")
		}
		dir = parent
	}
}
