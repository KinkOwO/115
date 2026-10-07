package servermod

import (
	"errors"
	"strings"
	"testing"
)

// reset 清空包级注册状态，让每个用例从零开始（这些状态是进程级单例）。
func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	boots = nil
	consoles = nil
	responses = nil
	requests = nil
	registeredIDs = map[string]bool{}
	modValues = map[string]string{}
	configKeys = map[string]string{}
	contentClaims = nil
	mu.Unlock()
	// 脚本注册表与启用清单各有自己的锁与状态，一并清掉，保证用例互不串味。
	scriptMu.Lock()
	modScripts = nil
	scriptMu.Unlock()
	enabledMu.Lock()
	enabledPath = ""
	disabledSet = map[string]bool{}
	enabledLoadNote = ""
	enabledMu.Unlock()
}

func TestRegisterAndBootRunsInOrder(t *testing.T) {
	reset(t)
	var order []string
	RegisterBoot("b.mod", func(*BootContext) error { order = append(order, "b"); return nil })
	RegisterBoot("a.mod", func(*BootContext) error { order = append(order, "a"); return nil })
	if err := Boot(&BootContext{Version: "test"}); err != nil {
		t.Fatalf("Boot 失败：%v", err)
	}
	// 注册顺序决定执行顺序（不是 id 排序）。
	if strings.Join(order, ",") != "b,a" {
		t.Fatalf("执行顺序 = %v，期望注册顺序 b,a", order)
	}
	if got := Registered(); len(got) != 2 || got[0] != "a.mod" || got[1] != "b.mod" {
		t.Fatalf("Registered() = %v，期望排序后的两个 id", got)
	}
}

func TestBootFailsClosed(t *testing.T) {
	reset(t)
	RegisterBoot("bad.mod", func(*BootContext) error { return errors.New("自检不过") })
	err := Boot(&BootContext{})
	if err == nil {
		t.Fatal("mod 自检失败时必须让 Boot 返回错误（fail-closed）")
	}
	if !strings.Contains(err.Error(), "bad.mod") {
		t.Fatalf("错误里应指认是哪个 mod：%v", err)
	}
}

func TestConsoleDispatchAndUnclaimed(t *testing.T) {
	reset(t)
	var seen ConsoleCommand
	RegisterConsole("x.mod", func(cmd ConsoleCommand) (bool, error) {
		if cmd.Name == "hello" {
			seen = cmd
			return true, nil
		}
		return false, nil
	})
	handled, err := Console("x.mod", "hello", []string{"a", "b"})
	if err != nil || !handled {
		t.Fatalf("命令应被认领：handled=%v err=%v", handled, err)
	}
	if seen.ModID != "x.mod" || seen.Name != "hello" || len(seen.Args) != 2 {
		t.Fatalf("命令上下文不对：%+v", seen)
	}
	// 未被认领的命令
	if handled, err := Console("x.mod", "nope", nil); handled || err != nil {
		t.Fatalf("未认领命令应 handled=false：%v %v", handled, err)
	}
	// 指定别的 mod id 时不应触发
	if handled, _ := Console("other.mod", "hello", nil); handled {
		t.Fatal("指定其它 mod id 时不应命中")
	}
}

func TestConsoleErrorIsReported(t *testing.T) {
	reset(t)
	RegisterConsole("x.mod", func(ConsoleCommand) (bool, error) { return true, errors.New("炸了") })
	handled, err := Console("x.mod", "boom", nil)
	if !handled || err == nil {
		t.Fatalf("命令返回错误时应上报：handled=%v err=%v", handled, err)
	}
}

func TestResponseObserversOnlyWhenRegistered(t *testing.T) {
	reset(t)
	if HasObservers() {
		t.Fatal("未注册时 HasObservers 应为 false（保证零开销路径）")
	}
	var gotOpcode uint16
	var gotBody []byte
	RegisterResponse("r.mod", func(conn string, opcode uint16, body []byte) {
		gotOpcode, gotBody = opcode, append([]byte(nil), body...)
	})
	if !HasObservers() {
		t.Fatal("注册后 HasObservers 应为 true")
	}
	ObserveResponse("1.2.3.4:5", 108, []byte{1, 2, 3})
	if gotOpcode != 108 || len(gotBody) != 3 {
		t.Fatalf("观察者收到的报文不对：%d %v", gotOpcode, gotBody)
	}
}

func TestConfigSnapshotAndWriteWhitelist(t *testing.T) {
	reset(t)
	SetEnvSnapshot([]string{
		"DFO_SHOP_OPEN_ALL=1",
		"DFO_BAKAL_MODE=unlimited",
		"PATH=/usr/bin", // 非 DFO_ 前缀，不应进快照
	})
	if v, ok := ConfigRead("DFO_SHOP_OPEN_ALL"); !ok || v != "1" {
		t.Fatalf("应能读到 DFO_ 键：%q %v", v, ok)
	}
	if _, ok := ConfigRead("PATH"); ok {
		t.Fatal("非 DFO_ 前缀的键不应进快照")
	}
	// 已登记的键可写
	if err := ConfigWrite("DFO_BAKAL_MODE", "limited"); err != nil {
		t.Fatalf("已登记键应可写：%v", err)
	}
	if v, _ := ModValue("DFO_BAKAL_MODE"); v != "limited" {
		t.Fatalf("写回值不对：%q", v)
	}
	// 未登记的键拒绝
	if err := ConfigWrite("DFO_NOT_REGISTERED", "x"); err == nil {
		t.Fatal("未登记键必须被拒绝（防止 mod 凭空造配置项）")
	}
	if err := ConfigWrite("", "x"); err == nil {
		t.Fatal("空键必须被拒绝")
	}
}

func TestContentRegisterRequiresName(t *testing.T) {
	reset(t)
	if err := ContentRegistrar("c.mod", "", "note"); err == nil {
		t.Fatal("空 name 必须被拒绝")
	}
	if err := ContentRegistrar("c.mod", "extra-drop-table", "示例"); err != nil {
		t.Fatalf("正常声明不应失败：%v", err)
	}
	if claims := ContentClaims(); len(claims) != 1 || !strings.Contains(claims[0], "extra-drop-table") {
		t.Fatalf("声明未记录：%v", claims)
	}
}

func TestRegisterRejectsNilAndBadID(t *testing.T) {
	reset(t)
	defer func() {
		if recover() == nil {
			t.Fatal("nil 回调应当 panic（编程错误要立刻暴露）")
		}
	}()
	RegisterBoot("ok.mod", nil)
}

func TestDescriptionEmptyAndNonEmpty(t *testing.T) {
	reset(t)
	if d := Description(); !strings.Contains(d, "未装载") {
		t.Fatalf("空状态描述不对：%q", d)
	}
	RegisterBoot("m1", func(*BootContext) error { return nil })
	RegisterResponse("m1", func(string, uint16, []byte) {})
	d := Description()
	if !strings.Contains(d, "m1") || !strings.Contains(d, "boot 钩子 1 个") {
		t.Fatalf("描述不对：%q", d)
	}
}

func TestValidModID(t *testing.T) {
	for _, ok := range []string{"a", "a.b", "a-b", "115us-devpack-pvf"} {
		if !validModID(ok) {
			t.Errorf("%q 应合法", ok)
		}
	}
	for _, bad := range []string{"", "A", "a_b", "a b", strings.Repeat("x", 65), "a/b"} {
		if validModID(bad) {
			t.Errorf("%q 应非法", bad)
		}
	}
}

// TestRequestHookSeesFrameAndShortCircuits 钉住请求钩子的核心语义：
// 上下文带全帧信息、首个 handled=true 的钩子接手、后续钩子不再被问。
func TestRequestHookSeesFrameAndShortCircuits(t *testing.T) {
	reset(t)
	var order []string
	var seen *RequestContext
	RegisterRequest("first.mod", func(ctx *RequestContext) (bool, error) {
		order = append(order, "first")
		return false, nil
	})
	RegisterRequest("second.mod", func(ctx *RequestContext) (bool, error) {
		order = append(order, "second")
		seen = ctx
		return true, nil
	})
	RegisterRequest("third.mod", func(ctx *RequestContext) (bool, error) {
		order = append(order, "third")
		return false, nil
	})

	var replied []byte
	handled := ObserveRequest("peer-1", 1, 2043, []byte{0xAA}, []byte{0xBB, 0xCC}, true,
		func(kind byte, id uint16, payload []byte) error {
			replied = append(replied, payload...)
			return nil
		})

	if !handled {
		t.Fatal("第二个钩子返回 true 时 ObserveRequest 必须报 handled")
	}
	if strings.Join(order, ",") != "first,second" {
		t.Fatalf("执行顺序 = %v，期望 first,second（短路后不再问第三个）", order)
	}
	if seen == nil || seen.Conn != "peer-1" || seen.Type != 1 || seen.ID != 2043 || !seen.Verified {
		t.Fatalf("上下文不对：%+v", seen)
	}
	if string(seen.Plaintext) != string([]byte{0xBB, 0xCC}) {
		t.Fatalf("正文不对：%x", seen.Plaintext)
	}
	if err := seen.Reply(0, 99, []byte{1, 2}); err != nil {
		t.Fatalf("Reply 失败：%v", err)
	}
	if string(replied) != string([]byte{1, 2}) {
		t.Fatalf("Reply 没送到调用方给的发送口：%x", replied)
	}
}

// TestRequestHookErrorDoesNotStopOthers 钉住"一个 mod 出错不拖垮其余 mod 与内置分发"。
func TestRequestHookErrorDoesNotStopOthers(t *testing.T) {
	reset(t)
	RegisterRequest("bad.mod", func(*RequestContext) (bool, error) {
		return false, errors.New("这个 mod 坏了")
	})
	RegisterRequest("good.mod", func(*RequestContext) (bool, error) { return true, nil })
	if !ObserveRequest("p", 1, 1, nil, nil, false, nil) {
		t.Fatal("坏 mod 之后的好 mod 仍应有机会接手")
	}
}

// TestRequestHookZeroCostWhenNone 钉住快路径：没登记钩子时不进循环、不碰 reply。
func TestRequestHookZeroCostWhenNone(t *testing.T) {
	reset(t)
	if HasRequestHooks() {
		t.Fatal("没登记钩子时 HasRequestHooks 必须为假")
	}
	called := false
	if ObserveRequest("p", 1, 1, nil, nil, true, func(byte, uint16, []byte) error {
		called = true
		return nil
	}) {
		t.Fatal("没有钩子时不应报 handled")
	}
	if called {
		t.Fatal("没有钩子时不应调用发送口")
	}
	RegisterRequest("only.mod", func(*RequestContext) (bool, error) { return false, nil })
	if !HasRequestHooks() {
		t.Fatal("登记后 HasRequestHooks 必须为真")
	}
	if !strings.Contains(Description(), "request 钩子 1 个") {
		t.Fatalf("Description 应报出 request 钩子数：%q", Description())
	}
}

// TestRegisterRequestRejectsBadInput 钉住与其它注册函数一致的入参纪律。
func TestRegisterRequestRejectsBadInput(t *testing.T) {
	reset(t)
	mustPanic(t, "nil 回调", func() { RegisterRequest("a.mod", nil) })
	mustPanic(t, "非法 id", func() {
		RegisterRequest("Bad_ID", func(*RequestContext) (bool, error) { return false, nil })
	})
}

func mustPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s 应当 panic", what)
		}
	}()
	fn()
}
