package launcher

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// buildGatewayTree 造出命令行构造函数会去"看存在性"的那些配置文件。只有这些文件存在时，
// 启动器这一族 tag 才会产出与 Python 版历史命令行完全相同的 token 序列。
func buildGatewayTree(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	for _, relative := range []string{
		"configs/character-rules.odyssey-release.json",
		"configs/character-probe.json",
		"configs/characters.skycastle-release.json",
		"configs/characters.generated.json",
		"configs/itemshop-candidate.json",
		"configs/randomoption.current37.json",
		"configs/equipment-wear.full-candidate.json",
		"configs/odyssey-currency.json",
		"configs/odyssey-weapon-box-release.json",
		"configs/odyssey-growth-release.json",
		"configs/odyssey-chapters-release.json",
		"configs/odyssey-chapter-drop-release.json",
	} {
		writeLaunchFile(t, project, relative, "{}")
	}
	return project
}

// currentSourceFlags 是当前源码程序（bin/wireprobe-pvf.exe -h）自报参数名里本阶段会下发
// 的那些。真实程序的列表比这更长，这里只需要"够它一个都不丢"，让本测试专注在 token 顺序。
const currentSourceFlags = `fixture output character-storage character-catalog character-rules
	select-probe-config entry-basic-probe town-catalog town-entry-probe responses world-rules
	vault-rules entry-addition-probe fatigue-rules progression-rules loot-catalog loot-rules
	bag-rules card-rules channel-refresh-config game-listen equipment-wear-rules account-options
	tutorial-routes tutorial-dungeons solo-party-bootstrap item-shop apocalypse-catalog
	random-option-catalog venus-flip-gear pvf-catalogs channel-identity`

// probeSupporting 是"当前源码程序"的能力表：它自报认识上面每一个参数。
func probeSupporting(string) map[string]bool {
	supported := map[string]bool{}
	for _, flag := range strings.Fields(currentSourceFlags) {
		supported[flag] = true
	}
	return supported
}

// launcherGatewayInput 是启动器真实下发时的那一组输入。
func launcherGatewayInput(t *testing.T, project string) gatewayCommandInput {
	t.Helper()
	env := newChildEnv(nil)
	env.Set("DFO_PVF_CATALOGS", "world,quests,characters,dungeons")
	env.Set("DFO_SERVER_BINARY", filepath.Join(project, "bin", "wireprobe-pvf.exe"))
	env.Set("DFO_CHANNEL_IDENTITY", "0")
	env.Set("DFO_ODYSSEY_REWARDS_RELEASE", "1")
	return gatewayCommandInput{
		Project: project,
		Session: newSessionTag(project, sessionTagForTest),
		Env:     env,
		Probe:   probeSupporting,
	}
}

// TestGatewayCommandMatchesThePythonTokenOrder 是这个阶段最重要的断言：整个 argv 的
// 每一个 token 与顺序，都与 Python 版真实写进 gateway.out 的那一行一致（历史运行
// 20261005_003333_830830_next37）。
func TestGatewayCommandMatchesThePythonTokenOrder(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	result, err := buildGatewayCommand(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out := in.Session.Out
	at := func(parts ...string) string { return filepath.Join(append([]string{project}, parts...)...) }
	want := []string{
		filepath.Join(project, "bin", "wireprobe-pvf.exe"),
		"-fixture", filepath.Join(out, "channelinfo.bin"),
		"-output", out,
		"-character-storage", at("runtime", "storage", "local.json"),
		"-character-catalog", at("configs", "characters.skycastle-release.json"),
		"-character-rules", at("configs", "character-rules.odyssey-release.json"),
		"-select-probe-config", at("cmd", "wireprobe", "testdata", "select-world-probe.json"),
		"-entry-basic-probe",
		"-town-catalog", at("configs", "town.generated.json"),
		"-town-entry-probe", at("cmd", "wireprobe", "testdata", "town-entry-probe.json"),
		"-responses", filepath.Join(out, "responses.json"),
		"-world-rules", at("configs", "world-probe.json"),
		"-vault-rules", at("configs", "vault.generated.json"),
		"-entry-addition-probe",
		"-fatigue-rules", at("configs", "fatigue-probe.json"),
		"-progression-rules", at("configs", "experience.compat90.json"),
		"-loot-catalog", "pvf",
		"-loot-rules", at("configs", "drop.current36.json"),
		"-bag-rules", at("configs", "inventory.current37.json"),
		"-card-rules", at("configs", "cards.compat90.json"),
		"-channel-refresh-config", at("configs", "channel.local35.json"),
		"-game-listen", "127.0.0.2:0",
		"-equipment-wear-rules", at("configs", "equipment-wear.current35.json"),
		"-account-options", at("configs", "account-options.current35.json"),
		"-tutorial-routes", at("configs", "tutorial-routes.current35.json"),
		"-tutorial-dungeons", at("configs", "tutorial-dungeons.current36.json"),
		"-solo-party-bootstrap",
		"-item-shop", at("configs", "itemshop-candidate.json"),
		"-apocalypse-catalog", at("configs", "apocalypse.generated.json"),
		"-random-option-catalog", at("configs", "randomoption.current37.json"),
		"-venus-flip-gear", at("configs", "venus-flip-gear.generated.json"),
		"-quest-equipment-catalog", "pvf",
	}
	if !reflect.DeepEqual(result.Args, want) {
		t.Errorf("argv 与 Python 版不一致\n got %s\nwant %s",
			strings.Join(result.Args, " "), strings.Join(want, " "))
	}
	if len(result.Notices) != 0 || len(result.Warnings) != 0 {
		t.Errorf("不应有告警：notices=%v warnings=%v", result.Notices, result.Warnings)
	}
	// 环境侧：PVF 直读时装备目录用 pvf，三张表给绝对路径，奥德赛组件常驻挂载。
	for key, wantValue := range map[string]string{
		"DFO_EQUIPMENT_CATALOG":       "pvf",
		"DFO_ATTUNEMENT_REWARDS":      at("configs", "attunement-rewards.generated.json"),
		"DFO_EQUIPMENT_JOURNAL_RULES": at("configs", "equipment-journal.generated.json"),
		"DFO_EQUIPMENT_CREATE_COST":   at("configs", "equipment-create-cost.generated.json"),
		"DFO_EQUIPMENT_WEAR_RULES":    at("configs", "equipment-wear.full-candidate.json"),
		"DFO_ODYSSEY_COIN_RULES":      at("configs", "odyssey-currency.json"),
		"DFO_ODYSSEY_WEAPON_BOX":      at("configs", "odyssey-weapon-box-release.json"),
		"DFO_ODYSSEY_REWARDS_RELEASE": "1",
		"DFO_ODYSSEY_GROWTH":          at("configs", "odyssey-growth-release.json"),
		"DFO_ODYSSEY_CHAPTERS":        at("configs", "odyssey-chapters-release.json"),
		"DFO_ODYSSEY_CHAPTER_DROP":    at("configs", "odyssey-chapter-drop-release.json"),
	} {
		if got := in.Env.Get(key); got != wantValue {
			t.Errorf("%s = %q, want %q", key, got, wantValue)
		}
	}
}

// channel_identity=1 时追加 -channel-identity，位置在 prune 之前、装备目录追加之前。
func TestGatewayCommandAddsChannelIdentity(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	in.Env.Set("DFO_CHANNEL_IDENTITY", "1")
	result, err := buildGatewayCommand(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{"-channel-identity", "-quest-equipment-catalog", "pvf"}
	got := result.Args[len(result.Args)-len(want):]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("命令行结尾 = %v, want %v", got, want)
	}
}

// 历史 tag（不含 roles_persist）走的是短命令行：只有 fixture/output 与 responses。
func TestGatewayCommandForAPlainChannelTag(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	in.Session = newSessionTag(project, "channel_04")
	result, err := buildGatewayCommand(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{
		filepath.Join(project, "bin", "wireprobe-pvf.exe"),
		"-fixture", filepath.Join(in.Session.Out, "channelinfo.bin"),
		"-output", in.Session.Out,
		"-responses", filepath.Join(in.Session.Out, "responses.json"),
		"-venus-flip-gear", filepath.Join(project, "configs", "venus-flip-gear.generated.json"),
		"-quest-equipment-catalog", "pvf",
	}
	if !reflect.DeepEqual(result.Args, want) {
		t.Errorf("argv = %v\nwant %v", result.Args, want)
	}
}

// prune_unsupported：旧程序不认识的参数连值一起丢，并留下与 Python 同义的告警。
func TestGatewayCommandPrunesFlagsTheBinaryDoesNotDefine(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	in.Env = newChildEnv(nil)
	in.Env.Set("DFO_SERVER_BINARY", filepath.Join(project, "bin", "old.exe"))
	in.Session = newSessionTag(project, "channel_04")
	in.Probe = func(exe string) map[string]bool {
		return map[string]bool{"fixture": true, "output": true}
	}
	result, err := buildGatewayCommand(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{filepath.Join(project, "bin", "old.exe"), "-fixture",
		filepath.Join(in.Session.Out, "channelinfo.bin"), "-output", in.Session.Out}
	if !reflect.DeepEqual(result.Args, want) {
		t.Errorf("argv = %v\nwant %v", result.Args, want)
	}
	if len(result.Notices) != 1 || !strings.Contains(result.Notices[0], "does not define -responses") {
		t.Errorf("告警 = %v", result.Notices)
	}
	// 没有 pvf-catalogs 支持、又没有历史目录要求时，不追加装备目录，也不报错。
	if commandFlagValue(result.Args, "-quest-equipment-catalog") != "" {
		t.Errorf("不该追加装备目录：%v", result.Args)
	}
}

// PVF 直读遇到不认识 pvf-catalogs 的程序必须拒绝启动，而不是起一个读别处的服务端。
func TestGatewayCommandRefusesABinaryWithoutPVFSupport(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	in.Probe = func(exe string) map[string]bool { return nil }
	_, err := buildGatewayCommand(in)
	if !errors.Is(err, errNoPVFSupport) {
		t.Fatalf("err = %v, want errNoPVFSupport", err)
	}
}

// 历史二进制必须显式给出配套导出的装备目录，否则拒绝（避免静默半套）。
func TestGatewayCommandRefusesHistoricalBinaryWithoutCatalog(t *testing.T) {
	project := buildGatewayTree(t)
	in := launcherGatewayInput(t, project)
	in.Session = newSessionTag(project, "roles_persist_select_actor_town_world_live_detail_dungeon_manual_x_next37")
	in.Env = newChildEnv(nil)
	in.Env.Set("DFO_SERVER_BINARY", filepath.Join(project, "bin", "old.exe"))
	in.Probe = func(exe string) map[string]bool { return map[string]bool{"fixture": true} }
	if _, err := buildGatewayCommand(in); err == nil {
		t.Fatal("want an error for a historical binary without a catalog")
	}
	// 给了历史目录就用它，并同步到环境变量。
	in.Env.Set("DFO_HISTORICAL_EQUIPMENT_CATALOG", "legacy/equipment.json")
	result, err := buildGatewayCommand(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := commandFlagValue(result.Args, "-quest-equipment-catalog"); got != "legacy/equipment.json" {
		t.Errorf("-quest-equipment-catalog = %q", got)
	}
	if got := in.Env.Get("DFO_EQUIPMENT_CATALOG"); got != "legacy/equipment.json" {
		t.Errorf("DFO_EQUIPMENT_CATALOG = %q", got)
	}
}

// 三张绝对路径表与奥德赛默认表都要在"环境里没给"时由启动器挂上。
func TestGatewayMountsDefaultTablesOnlyWhenPresent(t *testing.T) {
	project := t.TempDir()
	env := newChildEnv(nil)
	fallback := filepath.Join(project, "configs", "odyssey-currency.json")
	mountEnvTable(env, project, "DFO_ODYSSEY_COIN_RULES", fallback, nil)
	if env.Has("DFO_ODYSSEY_COIN_RULES") {
		t.Error("默认表不存在时不该挂载")
	}
	writeLaunchFile(t, project, "configs/odyssey-currency.json", "{}")
	mountEnvTable(env, project, "DFO_ODYSSEY_COIN_RULES", fallback, nil)
	if got := env.Get("DFO_ODYSSEY_COIN_RULES"); got != fallback {
		t.Errorf("挂载值 = %q, want %q", got, fallback)
	}
	// 环境里已经给了存在的相对路径：按包根解析并覆盖。
	env.Set("DFO_ODYSSEY_COIN_RULES", "configs/custom.json")
	writeLaunchFile(t, project, "configs/custom.json", "{}")
	mountEnvTable(env, project, "DFO_ODYSSEY_COIN_RULES", fallback, nil)
	if got, want := env.Get("DFO_ODYSSEY_COIN_RULES"), filepath.Join(project, "configs", "custom.json"); got != want {
		t.Errorf("覆盖值 = %q, want %q", got, want)
	}
	// 环境里指的是不存在的文件：保持原样（Python 同样只在 exists() 时覆盖）。
	env.Set("DFO_ODYSSEY_COIN_RULES", "configs/missing.json")
	mountEnvTable(env, project, "DFO_ODYSSEY_COIN_RULES", fallback, nil)
	if got := env.Get("DFO_ODYSSEY_COIN_RULES"); got != "configs/missing.json" {
		t.Errorf("不存在时 = %q，应保持原样", got)
	}
	// 武器盒挂上时要连带打开奖励开关。
	boxFallback := filepath.Join(project, "configs", "odyssey-weapon-box-release.json")
	writeLaunchFile(t, project, "configs/odyssey-weapon-box-release.json", "{}")
	mountEnvTable(env, project, "DFO_ODYSSEY_WEAPON_BOX", boxFallback, map[string]string{"DFO_ODYSSEY_REWARDS_RELEASE": "1"})
	if env.Get("DFO_ODYSSEY_WEAPON_BOX") != boxFallback || env.Get("DFO_ODYSSEY_REWARDS_RELEASE") != "1" {
		t.Errorf("武器盒挂载 = %q / %q", env.Get("DFO_ODYSSEY_WEAPON_BOX"), env.Get("DFO_ODYSSEY_REWARDS_RELEASE"))
	}
}

// 角色目录只在"characters"域没走原生 PVF 时才检查，告警口径与 catalog_startup.py 一致。
func TestValidateJSONCatalogs(t *testing.T) {
	project := t.TempDir()
	env := newChildEnv(nil)

	// 原生域：整段跳过，哪怕文件根本不存在。
	env.Set("DFO_PVF_CATALOGS", "characters,dungeons")
	warnings, err := validateJSONCatalogs([]string{"-character-catalog", filepath.Join(project, "missing.json")}, env)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("原生域不该有告警：%v %v", warnings, err)
	}

	// JSON 档：文件不存在 -> 告警；存在但没有 growtype -> 另一条告警。
	env = newChildEnv(nil)
	missing := filepath.Join(project, "configs", "missing.json")
	warnings, err = validateJSONCatalogs([]string{"-character-catalog", missing}, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "角色目录不存在") {
		t.Errorf("告警 = %v", warnings)
	}
	noGrowth := filepath.Join(project, "configs", "nogrowth.json")
	writeLaunchFile(t, project, "configs/nogrowth.json", `{"professions": {"a": {"advancement_growth": {}}}}`)
	warnings, _ = validateJSONCatalogs([]string{"-character-catalog", noGrowth}, env)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "growtype") {
		t.Errorf("告警 = %v", warnings)
	}
	withGrowth := filepath.Join(project, "configs", "growth.json")
	writeLaunchFile(t, project, "configs/growth.json", `{"professions": {"a": {"advancement_growth": {"x": 1}}}}`)
	if warnings, _ = validateJSONCatalogs([]string{"-character-catalog", withGrowth}, env); len(warnings) != 0 {
		t.Errorf("有 growtype 时不该告警：%v", warnings)
	}
	bad := filepath.Join(project, "configs", "bad.json")
	writeLaunchFile(t, project, "configs/bad.json", "not json")
	warnings, _ = validateJSONCatalogs([]string{"-character-catalog", bad}, env)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "无法解析") {
		t.Errorf("告警 = %v", warnings)
	}

	// 副本目录已经退休 JSON 入口：直接拒绝。
	if _, err := validateJSONCatalogs([]string{"-dungeon-catalog", "x.json"}, env); err == nil {
		t.Error("want an error for -dungeon-catalog without the PVF domain")
	}
}

// 会话 tag：目录留原始 tag，行为看逐级降级后的 tag。
func TestSessionTagDowngrade(t *testing.T) {
	cases := []struct {
		tag           string
		effective     string
		c35, c36, c37 bool
	}{
		{"channel_01", "channel_01", false, false, false},
		{"roles_persist_x_next34", "roles_persist_x_next34", false, false, false},
		{"roles_persist_x_next35", "roles_persist_x_next34", true, false, false},
		{"roles_persist_x_next36", "roles_persist_x_next34", true, true, false},
		{"roles_persist_x_next37", "roles_persist_x_next34", true, true, true},
	}
	for _, testCase := range cases {
		got := newSessionTag(`C:\mod`, testCase.tag)
		if got.Effective != testCase.effective ||
			got.Candidate35 != testCase.c35 || got.Candidate36 != testCase.c36 || got.Candidate37 != testCase.c37 {
			t.Errorf("tag %q -> %+v", testCase.tag, got)
		}
		if got.Original != testCase.tag {
			t.Errorf("原始 tag 被改动了：%q", got.Original)
		}
		if want := filepath.Join(`C:\mod`, "runtime", testCase.tag); got.Out != want {
			t.Errorf("out = %q, want %q", got.Out, want)
		}
	}
}

// childEnv 保持 base 的顺序，重复的键原位覆盖，新键追加。
func TestChildEnvOrderAndOverride(t *testing.T) {
	env := newChildEnv([]string{"A=1", "B=2", "A=3", "WEIRD"})
	env.Set("C", "4")
	env.Set("B", "5")
	want := []string{"A=3", "B=5", "C=4", "WEIRD"}
	if got := env.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
	if !env.Has("C") || env.Has("D") {
		t.Error("Has() 判断错误")
	}
	if env.Get("D") != "" {
		t.Error("未设置的键应为空串")
	}
}

// 会话 tag 的形态：前缀 + YYYYmmdd_HHMMSS_ffffff + _next37。
func TestSessionTagNameShape(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 123456000, time.Local)
	tag := sessionTagName(now)
	if want := launcherTagPrefix + "20261005_120000_123456_next37"; tag != want {
		t.Fatalf("tag = %q, want %q", tag, want)
	}
}
