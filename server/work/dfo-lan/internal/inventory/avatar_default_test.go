package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"testing"
)

func toks(ts ...pvf.Token) []pvf.Token { return ts }

func TestDefaultSocketsFromFields(t *testing.T) {
	// [avatar type select] 尾部 "2 [C socket] [C socket]"（普通上衣）。
	fields := map[string][]pvf.Token{
		"[avatar type select]": toks(
			pvf.Token{Type: 0, Value: 7}, pvf.Token{Type: 0}, pvf.Token{Type: 0},
			pvf.Token{Type: 0, Value: 2}, pvf.Token{Type: 3, Text: "[C socket]"}, pvf.Token{Type: 3, Text: "[C socket]"},
		),
	}
	ss := defaultSocketsFromFields(fields)
	if len(ss) != 2 || ss[0] != 4 || ss[1] != 4 { // C socket mask = 4
		t.Fatalf("coat default sockets = %v, want [4 4]", ss)
	}

	// [emblem socket default]（光环）：[M socket] [M socket] → 65519 ×2。
	fields2 := map[string][]pvf.Token{
		"[emblem socket default]": toks(
			pvf.Token{Type: 3, Text: "[M socket]"}, pvf.Token{Type: 3, Text: "[M socket]"},
		),
	}
	ss2 := defaultSocketsFromFields(fields2)
	if len(ss2) != 2 || ss2[0] != avatarMultiSocket || ss2[1] != avatarMultiSocket {
		t.Fatalf("aura default sockets = %v, want [65519 65519]", ss2)
	}

	// 115 级时装（115500002 实测）：`3 [S socket] [C socket] [C socket]`
	// → 白金孔 + 绿孔 ×2。
	fields3 := map[string][]pvf.Token{
		"[avatar type select]": toks(
			pvf.Token{Type: 0}, pvf.Token{Type: 0}, pvf.Token{Type: 0},
			pvf.Token{Type: 0, Value: 3},
			pvf.Token{Type: 3, Text: "[S socket]"},
			pvf.Token{Type: 3, Text: "[C socket]"},
			pvf.Token{Type: 3, Text: "[C socket]"},
		),
	}
	ss3 := defaultSocketsFromFields(fields3)
	if len(ss3) != 3 || ss3[0] != avatarPlatinumSocket || ss3[1] != 4 || ss3[2] != 4 {
		t.Fatalf("115 coat default sockets = %v, want [16 4 4]", ss3)
	}

	// 无默认孔定义 → 空。
	if ss4 := defaultSocketsFromFields(map[string][]pvf.Token{}); len(ss4) != 0 {
		t.Fatalf("no-socket fields = %v, want empty", ss4)
	}

	// 节日光环 [emblem socket default] 带白金孔：[M socket] [M socket] [S socket]
	// → 彩色 ×2 + 白金。
	fields5 := map[string][]pvf.Token{
		"[emblem socket default]": toks(
			pvf.Token{Type: 3, Text: "[M socket]"},
			pvf.Token{Type: 3, Text: "[M socket]"},
			pvf.Token{Type: 3, Text: "[S socket]"},
		),
	}
	ss5 := defaultSocketsFromFields(fields5)
	if len(ss5) != 3 || ss5[0] != avatarMultiSocket || ss5[1] != avatarMultiSocket || ss5[2] != avatarPlatinumSocket {
		t.Fatalf("festive aura default sockets = %v, want [65519 65519 16]", ss5)
	}
}

func TestDefaultAvatarSocketsWritesWireFormat(t *testing.T) {
	c := &EquipmentCatalog{}
	// 无定义 → 30 字节全 0（0 孔）。
	opts := c.DefaultAvatarSockets(0)
	if len(opts) != 30 {
		t.Fatalf("len = %d, want 30", len(opts))
	}
	if binary.LittleEndian.Uint16(opts[0:]) != 0 {
		t.Fatalf("slot0 = %d, want 0", binary.LittleEndian.Uint16(opts[0:]))
	}
}
