package servermod

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本文件钉住「热开关」的三条责任：
//
//  1. **登记 ≠ 在跑**：禁用名单里的 mod 照样登记钩子（它已经编进二进制了），
//     但每一个派发点都按 live 过滤；
//  2. **清单变化能被发现**：RefreshEnabledList 只在文件指纹变了时才重读；
//  3. **迁移只做能安全做的**：没有 cleanup 的 mod 关掉后不许再打开，
//     免得 boot 副作用被改第二遍。

// writeDisabledList 写一份禁用清单并返回 mods 目录。
func writeDisabledList(t *testing.T, ids ...string) string {
	t.Helper()
	mods := t.TempDir()
	list := `{"schema":1,"disabled":[`
	for i, id := range ids {
		if i > 0 {
			list += ","
		}
		list += `"` + id + `"`
	}
	list += `]}`
	if err := os.WriteFile(filepath.Join(mods, enabledFileName), []byte(list), 0o644); err != nil {
		t.Fatalf("写 %s 失败：%v", enabledFileName, err)
	}
	return mods
}

// TestDisabledModRegistersButNeverDispatches 是热开关的核心口径：
// 钩子登记着（证明二进制里有它），但一个都不会被调用。
func TestDisabledModRegistersButNeverDispatches(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t, "off.mod")
	LoadEnabledList(mods)

	var resp, req int
	RegisterResponse("off.mod", func(string, uint16, []byte) { resp++ })
	RegisterRequest("off.mod", func(*RequestContext) (bool, error) { req++; return false, nil })
	RegisterBoot("off.mod", func(*BootContext) error { return errors.New("停用的 mod 不该跑 boot") })

	if !IsRegistered("off.mod") {
		t.Fatal("钩子照样登记（二进制里确实有它），否则实况文件会误判成「没重新编译」")
	}
	if Live("off.mod") {
		t.Fatal("禁用名单里的 mod 不该 live")
	}
	if err := Boot(&BootContext{Version: "test"}); err != nil {
		t.Fatalf("Boot 不该跑停用的 mod，却报错了：%v", err)
	}
	ObserveResponse("c", 1, []byte{1})
	if resp != 0 {
		t.Fatalf("停用的 mod 的响应观察者被调用 %d 次", resp)
	}
	if ObserveRequest("c", 1, 1, nil, nil, true, func(byte, uint16, []byte) error { return nil }) {
		t.Fatal("没有 live 钩子时不应报 handled")
	}
	if req != 0 {
		t.Fatalf("停用的 mod 的请求钩子被调用 %d 次", req)
	}
	// HasObservers / HasRequestHooks 问的是"有没有人在跑"，不是"有没有登记"。
	if HasObservers() || HasRequestHooks() {
		t.Fatal("全部 mod 都停用时两个快路径判据必须为假")
	}
}

// TestRegisteredModStaysLiveWhenListMissing 钉住默认方向：
// 没有清单 = 全部启用，新装的 mod 不需要谁先勾选才生效。
func TestRegisteredModStaysLiveWhenListMissing(t *testing.T) {
	reset(t)
	LoadEnabledList(t.TempDir())
	var ran bool
	RegisterBoot("new.mod", func(*BootContext) error { ran = true; return nil })
	if !Live("new.mod") {
		t.Fatal("没有禁用清单时新 mod 必须直接 live")
	}
	if err := Boot(&BootContext{}); err != nil {
		t.Fatalf("Boot 失败：%v", err)
	}
	if !ran {
		t.Fatal("boot 钩子没跑")
	}
}

// TestRefreshEnabledListOnlyReparsesOnFingerprintChange 钉住轮询成本：
// 文件没变 ⇒ 不重读（changed=false），内容变了一行 ⇒ 立刻发现。
func TestRefreshEnabledListOnlyReparsesOnFingerprintChange(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t, "a.mod")
	LoadEnabledList(mods)

	if changed, err := RefreshEnabledList(); err != nil || changed {
		t.Fatalf("同一份文件不该算变化：changed=%v err=%v", changed, err)
	}
	// 只改顺序不改集合：集合等价 ⇒ 不触发迁移（否则每次写盘都会白搬一遍）。
	if err := os.WriteFile(filepath.Join(mods, enabledFileName),
		[]byte(`{"schema":1,"disabled":["a.mod"]}`), 0o644); err != nil {
		t.Fatalf("写清单失败：%v", err)
	}
	if changed, err := RefreshEnabledList(); err != nil || changed {
		t.Fatalf("集合等价时不该报变化：changed=%v err=%v", changed, err)
	}
	// 新增一个禁用项：大小一定变，指纹必然不同。
	if err := os.WriteFile(filepath.Join(mods, enabledFileName),
		[]byte(`{"schema":1,"disabled":["a.mod","b.mod"]}`), 0o644); err != nil {
		t.Fatalf("写清单失败：%v", err)
	}
	changed, err := RefreshEnabledList()
	if err != nil || !changed {
		t.Fatalf("禁用名单变了必须报变化：changed=%v err=%v", changed, err)
	}
	if Enabled("b.mod") {
		t.Fatal("b.mod 现在应已禁用")
	}
}

// TestRefreshEnabledListCorruptKeepsEverythingOn 钉住"清单坏了 ≠ 全关"。
func TestRefreshEnabledListCorruptKeepsEverythingOn(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t, "a.mod")
	LoadEnabledList(mods)
	if err := os.WriteFile(filepath.Join(mods, enabledFileName), []byte(`{不是 json`), 0o644); err != nil {
		t.Fatalf("写坏清单失败：%v", err)
	}
	changed, err := RefreshEnabledList()
	if err != nil {
		t.Fatalf("坏清单不是错误（只记说明）：%v", err)
	}
	if !changed {
		t.Fatal("从「禁用 a」变成「全部启用」是真实变化，必须报出来")
	}
	if !Enabled("a.mod") {
		t.Fatal("清单解析失败时必须退回全部启用")
	}
	if _, _, note := EnabledInfo(); note == "" {
		t.Fatal("退回全部启用的原因必须留一条可见的说明")
	}
}

// TestApplyDesiredStateDisableWithoutCleanupRefusesReEnable 钉住热开关唯一的拒绝路径：
// boot 改过的状态没人撤销 ⇒ 不允许再打开（那会把同一份状态改两遍）。
func TestApplyDesiredStateDisableWithoutCleanupRefusesReEnable(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t)
	LoadEnabledList(mods)

	var booted int
	RegisterBoot("dirty.mod", func(*BootContext) error { booted++; return nil })
	if !Live("dirty.mod") || booted != 0 {
		t.Fatal("起点应是 live（但还没跑过 boot）")
	}
	// 启动期照常跑一次 boot：下面判"会不会被改第二遍"要有这个第一次作参照。
	if err := Boot(&BootContext{Version: "test"}); err != nil {
		t.Fatalf("Boot 失败：%v", err)
	}
	if booted != 1 {
		t.Fatalf("启动期 boot 应恰好跑一次（现在 %d 次）", booted)
	}

	// 关掉：它没有 server.shutdown，所以 boot 副作用留在进程里。
	mods = writeDisabledListIn(t, mods, "dirty.mod")
	LoadEnabledList(mods)
	out, err := ApplyDesiredState("test")
	if err != nil {
		t.Fatalf("ApplyDesiredState 失败：%v", err)
	}
	if len(out) != 1 || out[0].Action != ActionDisabled {
		t.Fatalf("应有一条停用迁移：%v", out)
	}
	if !out[0].RequiresRestart || out[0].Note == "" {
		t.Fatalf("没有 cleanup 的停用必须标 RequiresRestart 并说明原因：%+v", out[0])
	}
	if Live("dirty.mod") {
		t.Fatal("停用后必须停止派发")
	}

	// 再打开：必须被拒绝，且 boot 不能被重跑。
	LoadEnabledList(writeDisabledList(t))
	if _, err := RefreshEnabledList(); err != nil {
		t.Fatalf("RefreshEnabledList 失败：%v", err)
	}
	out, err = ApplyDesiredState("test")
	if err != nil {
		t.Fatalf("ApplyDesiredState 失败：%v", err)
	}
	if len(out) != 1 || out[0].Action != ActionRefused {
		t.Fatalf("脏状态的 mod 重新打开必须被拒绝：%v", out)
	}
	if Live("dirty.mod") {
		t.Fatal("被拒绝后不该 live")
	}
	if booted != 1 {
		t.Fatalf("被拒绝时不该重跑 boot（跑了 %d 次）", booted)
	}
	if !IsDirty("dirty.mod") {
		t.Fatal("拒绝后仍应标记为脏，直到重启")
	}
}

// TestApplyDesiredStateWithCleanupRoundTrips 钉住正例：
// 写了 server.shutdown 的 mod 可以关→开往返，且 boot 恰好各跑一次。
func TestApplyDesiredStateWithCleanupRoundTrips(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t)
	LoadEnabledList(mods)

	var booted, cleaned int
	RegisterBoot("tidy.mod", func(*BootContext) error { booted++; return nil })
	RegisterShutdown("tidy.mod", func(ctx *ShutdownContext) error {
		if ctx.Reason != ReasonDisabled {
			t.Errorf("运行中停用时的收尾原因 = %q，期望 %q", ctx.Reason, ReasonDisabled)
		}
		cleaned++
		return nil
	})

	LoadEnabledList(writeDisabledListIn(t, mods, "tidy.mod"))
	if _, err := RefreshEnabledList(); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	out, err := ApplyDesiredState("test")
	if err != nil {
		t.Fatalf("停用失败：%v", err)
	}
	if len(out) != 1 || out[0].RequiresRestart {
		t.Fatalf("有 cleanup 的停用不该要求重启：%+v", out)
	}
	if cleaned != 1 || booted != 0 {
		t.Fatalf("收尾应跑一次、boot 不该跑（cleaned=%d booted=%d）", cleaned, booted)
	}

	// 再打开：boot 重跑一次，脏标记清掉。
	LoadEnabledList(writeDisabledList(t))
	if _, err := RefreshEnabledList(); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	if _, err := ApplyDesiredState("test"); err != nil {
		t.Fatalf("打开失败：%v", err)
	}
	if booted != 1 {
		t.Fatalf("打开时 boot 应重跑一次（现在 %d 次）", booted)
	}
	if !Live("tidy.mod") || IsDirty("tidy.mod") {
		t.Fatal("往返后应是干净的 live 状态")
	}
}

// TestEnableRefusedWhenBootFails 钉住运行期打开失败的处置：只关这个 mod，不牵连服务端。
func TestEnableRefusedWhenBootFails(t *testing.T) {
	reset(t)
	LoadEnabledList(writeDisabledList(t, "sick.mod"))
	RegisterBoot("sick.mod", func(*BootContext) error { return errors.New("缺存储") })
	if Live("sick.mod") {
		t.Fatal("起点应是停用")
	}
	LoadEnabledList(writeDisabledList(t))
	if _, err := RefreshEnabledList(); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	out, err := ApplyDesiredState("test")
	if err != nil {
		t.Fatalf("热开失败不该让整个调用报错（应作为迁移结果返回）：%v", err)
	}
	if len(out) != 1 || out[0].Action != ActionRefused {
		t.Fatalf("boot 自检失败必须报成拒绝：%v", out)
	}
	if !strings.Contains(out[0].Note, "缺存储") {
		t.Fatalf("拒绝说明里要带上 mod 自己的错误：%q", out[0].Note)
	}
	if Live("sick.mod") {
		t.Fatal("boot 失败的 mod 必须保持关闭")
	}
}

// TestConsoleNamesStoppedMod 钉住"命令打到停用的 mod"不能假装没这条命令。
func TestConsoleNamesStoppedMod(t *testing.T) {
	reset(t)
	LoadEnabledList(writeDisabledList(t, "off.mod"))
	RegisterConsole("off.mod", func(ConsoleCommand) (bool, error) {
		t.Fatal("停用的 mod 的 console 钩子不该被调用")
		return true, nil
	})
	handled, err := Console("off.mod", "run", nil)
	if !handled {
		t.Fatal("点名打到停用的 mod 时必须算已认领（否则操作者看到帮助而不是拒绝）")
	}
	if err == nil || !strings.Contains(err.Error(), "已停用") {
		t.Fatalf("应明确报「已停用」，得到：%v", err)
	}
}

// TestShutdownLiveCleansEveryRunningMod 钉住退出路径：每个 live 的 mod 各收尾一次。
func TestShutdownLiveCleansEveryRunningMod(t *testing.T) {
	reset(t)
	LoadEnabledList(writeDisabledList(t, "off.mod"))
	var off, on int
	RegisterShutdown("off.mod", func(*ShutdownContext) error { off++; return nil })
	RegisterShutdown("on.mod", func(ctx *ShutdownContext) error {
		if ctx.Reason != ReasonStopping {
			t.Errorf("进程退出时的收尾原因 = %q，期望 %q", ctx.Reason, ReasonStopping)
		}
		on++
		return nil
	})
	ShutdownLive("test")
	if on != 1 || off != 0 {
		t.Fatalf("只该收尾在跑的 mod（on=%d off=%d）", on, off)
	}
	if Live("on.mod") {
		t.Fatal("收尾之后不该再有 mod 处于 live")
	}
}

// TestBuildStatusFlagsUncompiledMod 钉住"装了但没重新编译"这条硬判据。
func TestBuildStatusFlagsUncompiledMod(t *testing.T) {
	reset(t)
	mods := t.TempDir()
	writeModDir(t, mods, "stale.mod", modJSONWithHooks("stale.mod", HookBoot))
	LoadEnabledList(mods)

	st := BuildStatus(SelfInfo{ServerVersion: "test", PID: 42})
	if len(st.Mods) != 1 || st.Mods[0].ID != "stale.mod" {
		t.Fatalf("实况里该有磁盘上那一个 mod：%+v", st.Mods)
	}
	got := st.Mods[0]
	if !got.RequiresRebuild || !got.RequiresRestart {
		t.Fatalf("磁盘有声明、进程无钩子 ⇒ 必须标 requiresRebuild：%+v", got)
	}
	if got.Live {
		t.Fatal("没登记的 mod 不该 live")
	}
	if len(got.Declared) != 1 || got.Declared[0] != HookBoot {
		t.Fatalf("声明列表应原样带出：%v", got.Declared)
	}
}

// TestWriteRuntimeStatusIsReadableBack 钉住文件契约本身：写出去的东西要能按结构读回来。
func TestWriteRuntimeStatusIsReadableBack(t *testing.T) {
	reset(t)
	LoadEnabledList(writeDisabledList(t, "off.mod"))
	RegisterBoot("on.mod", func(*BootContext) error { return nil })
	RegisterRewardScript("on.mod", "on-live.lua", []byte(`on("level_up", function(ctx) end)`))

	path := filepath.Join(t.TempDir(), "nested", StatusFileName)
	self := SelfInfo{ServerVersion: "test", PID: 7, ExeSHA256: "abc"}
	if err := WriteRuntimeStatus(path, self); err != nil {
		t.Fatalf("写实况文件失败：%v", err)
	}
	// 原子写的临时文件不该留下尸体。
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("临时文件没被 rename 掉：%v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回失败：%v", err)
	}
	var st RuntimeStatus
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("实况文件不是合法 JSON：%v", err)
	}
	if st.Schema != StatusFileSchema || st.Self.PID != 7 || st.Self.ExeSHA256 != "abc" {
		t.Fatalf("自述信息没落进文件：%+v", st.Self)
	}
	byID := map[string]ModStatus{}
	for _, m := range st.Mods {
		byID[m.ID] = m
	}
	on, offMod := byID["on.mod"], byID["off.mod"]
	if !on.Live || !on.Enabled || len(on.Scripts) != 1 {
		t.Fatalf("在跑的 mod 应带 live 与它的脚本：%+v", on)
	}
	if offMod.Enabled || offMod.Live {
		t.Fatalf("禁用名单里的 mod 不该显示成启用/在跑：%+v", offMod)
	}
	if !strings.Contains(st.Summary, "servermod:") {
		t.Fatalf("摘要应带上装载自述：%q", st.Summary)
	}
}

// TestRegisterRewardScriptSkipsDisabledMod 钉住"停用 = 不装载"：
// 坏脚本不该把一个关着的 mod 变成全服奖励管线的炸弹。
func TestRegisterRewardScriptSkipsDisabledMod(t *testing.T) {
	reset(t)
	LoadEnabledList(writeDisabledList(t, "off.mod"))
	if err := RegisterRewardScript("off.mod", "off-rule.lua", []byte(`这不是 lua`)); err != nil {
		t.Fatalf("停用的 mod 登记脚本应直接成功跳过（不是错误）：%v", err)
	}
	if len(RewardScripts()) != 0 {
		t.Fatalf("停用的 mod 的脚本不该进装载集合：%v", RewardScriptNames())
	}
	// 归属反查是**开放默认**的：查不到归属 = 内置规则集或磁盘直读的脚本，必须放行，
	// 否则一条反查失败就把整条奖励管线判成停用（空转，最难查的那种）。
	if !RewardScriptEnabled("off-rule.lua") {
		t.Fatal("没有 mod 归属的脚本名必须放行（内置/磁盘直读规则集）")
	}
	if !RewardScriptEnabled("newchar.lua") {
		t.Fatal("内置脚本没有 mod 归属，必须始终可用")
	}

	RegisterRewardScript("on.mod", "on-rule.lua", []byte(`on("level_up", function(ctx) end)`))
	if !RewardScriptEnabled("on-rule.lua") {
		t.Fatal("启用的 mod 的脚本应可用")
	}
	if got := RewardScriptsByMod("on.mod"); len(got) != 1 || got[0] != "on-rule.lua" {
		t.Fatalf("按 mod 反查脚本失败：%v", got)
	}
}

// TestBuildStatusDoesNotCallScriptOnlyModUncompiled 钉住另一侧的假告警：
// 只提供规则脚本/文件的 mod 不落 mod.json、本来也没有 Go 钩子，
// 把它标成"要重编译服务端"会让人去重建一个根本不需要重建的东西。
func TestBuildStatusDoesNotCallScriptOnlyModUncompiled(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t, "kit.scriptmod")
	LoadEnabledList(mods)

	st := BuildStatus(SelfInfo{ServerVersion: "test"})
	if len(st.Mods) != 1 || st.Mods[0].ID != "kit.scriptmod" {
		t.Fatalf("清单点名的 mod 必须出现在实况里：%+v", st.Mods)
	}
	got := st.Mods[0]
	if got.RequiresRebuild {
		t.Fatal("没有 mod.json 的 mod 不该被判成「装了但没重编译」")
	}
	if got.RequiresRestart {
		t.Fatal("既没有钩子也没有待撤销状态，谈不上要重启")
	}
	if got.Enabled {
		t.Fatal("它在禁用名单里，enabled 必须为假")
	}
	if !strings.Contains(got.Note, "只提供规则脚本/文件") {
		t.Fatalf("要说清「热开关管不到它」的原因：%q", got.Note)
	}
}

// TestShutdownHookIsNotADeclarationProblem 钉住一条会"悄悄弄坏每一次启动"的边界：
// server.shutdown 不在启动器 modkit 的钩子白名单里，mod 无法合法声明它；
// 一旦它被算进"实际注册的钩子"，声明与注册的双向差集就会永远报 extra，
// 于是每个写了收尾钩子的 mod 都让服务端**拒绝启动**（fail-closed 反过来咬人）。
func TestShutdownHookIsNotADeclarationProblem(t *testing.T) {
	reset(t)
	mods := writeDisabledList(t)
	writeModDir(t, mods, "tidy.mod", modJSONWithHooks("tidy.mod", HookBoot))
	LoadEnabledList(mods)

	RegisterBoot("tidy.mod", func(*BootContext) error { return nil })
	RegisterShutdown("tidy.mod", func(*ShutdownContext) error { return nil })

	problems, err := CheckDeclarations(mods)
	if err != nil {
		t.Fatalf("CheckDeclarations 报错：%v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("登记收尾钩子不该被判成声明不一致：%v", problems)
	}
	if contains(RegisteredHooks("tidy.mod"), HookShutdown) {
		t.Fatal("RegisteredHooks 不该带出 server.shutdown（它进不了 mod.json 的白名单）")
	}
	if !HasShutdown("tidy.mod") {
		t.Fatal("收尾钩子本身仍要能从 HasShutdown 查到，否则实况文件没法证明 mod 写了它")
	}
}

// writeDisabledListIn 往已有 mods 目录改写禁用清单（复用同一目录，让指纹变化可判）。
func writeDisabledListIn(t *testing.T, modsDir string, ids ...string) string {
	t.Helper()
	list := `{"schema":1,"disabled":[`
	for i, id := range ids {
		if i > 0 {
			list += ","
		}
		list += `"` + id + `"`
	}
	list += `]}`
	if err := os.WriteFile(filepath.Join(modsDir, enabledFileName), []byte(list), 0o644); err != nil {
		t.Fatalf("改写清单失败：%v", err)
	}
	return modsDir
}
