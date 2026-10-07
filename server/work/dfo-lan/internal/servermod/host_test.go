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
	registeredIDs = map[string]bool{}
	modValues = map[string]string{}
	configKeys = map[string]string{}
	contentClaims = nil
	mu.Unlock()
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
