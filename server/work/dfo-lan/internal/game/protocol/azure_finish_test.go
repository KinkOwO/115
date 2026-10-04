package protocol

import (
	"bytes"
	"testing"
)

// [AZURE-FINISH-249] NOTI249 FINISH_VILLAGE_MONSTER_FIGHTING 是蔚蓝号的收尾帧。
//
// 官服尾段在 CMD72 的应答之后补它（F16-s2c.txt #698），形态是 16B、中间一段未解语义的
// 5B 值、其余补零。本仓把那段 5B 留 0（与 N29 尾部 / N38 尾部已经验证「可省略」的
// 同一类字段一致），所以这里就是 16 个零。
func TestAzureMainFinishFightingShape(t *testing.T) {
	p := AzureMainFinishFighting()
	if len(p) != 16 {
		t.Fatalf("NOTI249 body = %d bytes, want 16", len(p))
	}
	for i, b := range p {
		if b != 0 {
			t.Fatalf("byte %d = 0x%02x, want 0（未解的 5B 留零）", i, b)
		}
	}
}
// [AZURE-SETTLEMENT-OPTION] NOTI72 把「重开 / 返回城镇」摆在结算面板上。
//
// 官服 #695/#696 是 16B `State, Option, 2, <5B 常量 c5 20 24 76 3f>`，
// 与客户端随后的 CMD72 体同形（cards.go 已记录那个常量）。
func TestAzureSettlementOptionOfferShape(t *testing.T) {
	for _, tc := range []struct{ state, option byte }{{1, 1}, {1, 2}} {
		p := AzureSettlementOptionOffer(tc.state, tc.option)
		if len(p) != 16 {
			t.Fatalf("NOTI72 body = %d bytes, want 16", len(p))
		}
		if p[0] != tc.state || p[1] != tc.option || p[2] != 2 {
			t.Fatalf("header = %v, want state=%d option=%d source=2", p[:3], tc.state, tc.option)
		}
		if !bytes.Equal(p[3:8], settlementExitEchoToken) {
			t.Fatalf("token = %x, want %x", p[3:8], settlementExitEchoToken)
		}
		for i, b := range p[8:] {
			if b != 0 {
				t.Fatalf("padding byte %d = 0x%02x, want 0", i+8, b)
			}
		}
	}
}
// [AZURE-SETTLEMENT-OPTION] NOTI70 / NOTI71 的形状（官服 #687 / #692 逐字节）。
func TestAzureSettlementOptionEnableShape(t *testing.T) {
	en := AzureSettlementOptionEnable()
	if len(en) != 32 {
		t.Fatalf("NOTI70 body = %d bytes, want 32", len(en))
	}
	if en[0] != 1 || en[1] != 1 || en[2] != 0 {
		t.Fatalf("NOTI70 header = %v, want 01 01 00", en[:3])
	}
	for i := 3; i < 17; i++ {
		if en[i] != 0xff {
			t.Fatalf("NOTI70 使能字节 %d = 0x%02x, want 0xff", i, en[i])
		}
	}
	rows := AzureSettlementOptionRows()
	if len(rows) != 40 {
		t.Fatalf("NOTI71 body = %d bytes, want 40", len(rows))
	}
	if rows[0] != 1 || rows[1] != 0 || rows[2] != 0xff || rows[3] != 0 || rows[4] != 0 {
		t.Fatalf("NOTI71 header = %v, want 01 00 ff 00 00", rows[:5])
	}
	for i := 5; i < 33; i += 4 {
		if rows[i] != 0xff || rows[i+1] != 0xff || rows[i+2] != 0 || rows[i+3] != 0 {
			t.Fatalf("NOTI71 行 %d = %v, want ff ff 00 00", i, rows[i:i+4])
		}
	}
}
