package accountname

import (
	"strings"
	"testing"
)

func TestValidAcceptsSafeNames(t *testing.T) {
	for _, name := range []string{Default, "a", "Tomeu-2", "tomeu_99", strings.Repeat("x", MaxLength)} {
		if !Valid(name) {
			t.Errorf("Valid(%q) = false, want true", name)
		}
	}
}

// 列表里每一项都会改变 payload 的 ? 分段、变成参数解析不了的写法，或者撞上 SQL 侧的唯一键
// 报错，所以必须在任何进程起来之前就拒掉。
func TestValidRejectsNamesThatWouldBreakThePayload(t *testing.T) {
	for _, name := range []string{"", " ", "probe?2", "pro be", "pro\tbe", "玩家", "x=y", "a/b", "a.b", strings.Repeat("x", MaxLength+1)} {
		if Valid(name) {
			t.Errorf("Valid(%q) = true, want false", name)
		}
	}
}
