package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// 实机抓包回放（2026-09-28 20:38:24 会话，next37）。这一场是完整的 2×2 交叉：
// 两件有增幅的装备（槽 9 模板 101040829、槽 10 模板 101040830）分别与两件
// 没有增幅的装备（槽 11 模板 101040833、槽 12 模板 101021079）配对。
//
// 480 字节 = 466 字节请求体 + 14 字节分组补位。
const inheritLiveSampleNew = "00000000000000000000000000000900bdc205060000000000000000000000000b00c1c2050601010000000000000000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000"

// 实机抓包回放（2026-09-28 15:18:51 会话）。这一场里 A 侧装备**正穿在身上**：
// 请求体偏移 44（记录内 +30）是 3 = 已穿戴，偏移 45（记录内 +31）是 0 = 背包。
const inheritLiveSampleOld = "00000000000000000000000000000c00bec2050600000000000000000000000015009775050601010000000003000000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000" +
	"0000000000000000000000002e2e0000000000000000000000000000000000000000000000000000000000002e2e0000" +
	"000000000000000000000000000000000000000000000000000000002e2e000000000000000000000000000000000000"

func decodeInheritSample(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 480 {
		t.Fatalf("样本长度 = %d，期望 480（466 + 14 补位）", len(b))
	}
	return b
}

// 布局钉死：14 字节前缀 + 14×32 记录 + 尾部 u32 = 466。
// 这条错了整条链路都错，所以先用常量自身做一次算术校验。
func TestInheritLayoutConstants(t *testing.T) {
	if got := InheritPrefixSize + InheritRecordSize*InheritRecordCount + 4; got != InheritBodySize {
		t.Fatalf("14 + 14*32 + 4 = %d，与 InheritBodySize=%d 不符", got, InheritBodySize)
	}
	if InheritTrailerOffset != 462 {
		t.Fatalf("尾部 u32 偏移 = %d，期望 462（sub_14138A210 的 a1+1974）", InheritTrailerOffset)
	}
}

// 新会话样本：A 侧槽 9 / 模板 101040829（增幅 +20），B 侧槽 11 / 模板 101040833（无增幅）。
// 两个模板号都是 NOTI13 背包快照里逐行对出来的，不是猜的。
func TestDecodeInheritRequestLiveSampleNewSession(t *testing.T) {
	b := decodeInheritSample(t, inheritLiveSampleNew)
	r, err := DecodeInheritRequest(b)
	if err != nil {
		t.Fatalf("解析继承请求失败: %v", err)
	}
	if len(r.Entries) != 1 {
		t.Fatalf("有效记录数 = %d，期望 1（其余 13 条模板为 0，应被过滤）", len(r.Entries))
	}
	if len(r.Items) != 2 {
		t.Fatalf("选中装备数 = %d，期望 2", len(r.Items))
	}
	e := r.Entries[0]
	if e.SlotA != 9 || e.TemplateA != 101040829 {
		t.Errorf("A 侧 = 槽%d 模板%d，期望 槽9 模板101040829", e.SlotA, e.TemplateA)
	}
	if e.SlotB != 11 || e.TemplateB != 101040833 {
		t.Errorf("B 侧 = 槽%d 模板%d，期望 槽11 模板101040833", e.SlotB, e.TemplateB)
	}
	if e.Const != 257 {
		t.Errorf("常量字段 = %d，期望 257（客户端 sub_14138A210 硬编码）", e.Const)
	}
	if e.Counter != 0 {
		t.Errorf("计数器 = %d，期望 0（实机恒 0，语义未取证）", e.Counter)
	}
	if e.SpaceA != 0 || e.SpaceB != 0 {
		t.Errorf("容器 = %d/%d，期望 0/0（都在背包）", e.SpaceA, e.SpaceB)
	}
	// 尾部 u32 恒 0，绝不能当记录数用（这里是这条断言存在的理由）。
	if got := binary.LittleEndian.Uint32(b[InheritTrailerOffset:]); got != 0 {
		t.Errorf("尾部 u32 = %d，期望 0", got)
	}
	// 展平后的两件装备必须带上各自的容器，服务层靠它定位。
	if r.Items[0].Slot != 9 || r.Items[0].Template != 101040829 || r.Items[0].Kind != 'A' {
		t.Errorf("展平第 1 件 = %+v，期望 A 侧 槽9/101040829", r.Items[0])
	}
	if r.Items[1].Slot != 11 || r.Items[1].Template != 101040833 || r.Items[1].Kind != 'B' {
		t.Errorf("展平第 2 件 = %+v，期望 B 侧 槽11/101040833", r.Items[1])
	}
}

// 旧会话样本：A 侧装备穿在身上（容器 3），B 侧在背包（容器 0）。
// 这条必须解对，否则「把身上那件继承给背包里那件」会整条失败。
func TestDecodeInheritRequestLiveSampleWornSide(t *testing.T) {
	b := decodeInheritSample(t, inheritLiveSampleOld)
	r, err := DecodeInheritRequest(b)
	if err != nil {
		t.Fatalf("解析继承请求失败: %v", err)
	}
	if len(r.Items) != 2 {
		t.Fatalf("选中装备数 = %d，期望 2", len(r.Items))
	}
	e := r.Entries[0]
	if e.SlotA != 12 || e.TemplateA != 101040830 {
		t.Errorf("A 侧 = 槽%d 模板%d，期望 槽12 模板101040830", e.SlotA, e.TemplateA)
	}
	if e.SlotB != 21 || e.TemplateB != 101021079 {
		t.Errorf("B 侧 = 槽%d 模板%d，期望 槽21 模板101021079", e.SlotB, e.TemplateB)
	}
	if e.SpaceA != 3 {
		t.Errorf("A 侧容器 = %d，期望 3（已穿戴）", e.SpaceA)
	}
	if e.SpaceB != 0 {
		t.Errorf("B 侧容器 = %d，期望 0（背包）", e.SpaceB)
	}
}

// 未使用的记录里容器字段残留 0x2E(46) —— sub_1471B45D0 只做了部分清零。
// 只能靠「哨兵」或「模板号 == 0」过滤，这条用例防止有人改成用尾部 u32 判条数。
func TestDecodeInheritRequestFiltersUnusedRecords(t *testing.T) {
	b := decodeInheritSample(t, inheritLiveSampleOld)
	for k := 1; k < InheritRecordCount; k++ {
		base := InheritPrefixSize + k*InheritRecordSize
		if b[base+30] != 0x2E || b[base+31] != 0x2E {
			t.Fatalf("第 %d 条未使用记录的容器残留应为 0x2E/0x2E，实际 %#x/%#x", k, b[base+30], b[base+31])
		}
	}
	r, _ := DecodeInheritRequest(b)
	if len(r.Entries) != 1 || len(r.Items) != 2 {
		t.Fatalf("残留被当成有效记录：解析出 %d 条 / %d 件", len(r.Entries), len(r.Items))
	}
}

func TestDecodeInheritRequestRejectsShortPayload(t *testing.T) {
	if _, err := DecodeInheritRequest(make([]byte, InheritBodySize-1)); err == nil {
		t.Fatal("少一个字节的继承请求应被拒绝（否则会越界读到补位区）")
	}
}

// ★ 制度性防线：协议包里**不允许**再出现任何 1722 回包构造函数。
//
// 原因：CMD 1722 在客户端双向都没有继承结果通道 —— kind=1 侧是客户端自己发出的
// 命令、没有接收 handler；kind=0 侧（NOTI 表）是小游戏道具计数通知，与继承结果无关。
// 因此任何名字的 1722 结果包构造函数都不许再出现。
//
// 用源码文本扫描而不是反射：构造函数一旦被删，反射版会退化成「永远通过」的
// 空断言，防不住回归。
func TestNoInheritReplyBuilderExists(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"InheritReply", "InheritRefusal", "InheritReplySize", "InheritResult"} {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			src, err := os.ReadFile(e.Name())
			if err != nil {
				t.Fatal(err)
			}
			// 只认「定义」（func/const/var/type 声明），注释里提到不算。
			for _, line := range strings.Split(string(src), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(trimmed, "func "+name+"(") ||
					strings.Contains(trimmed, name+" =") ||
					strings.Contains(trimmed, name+" ") && strings.HasPrefix(trimmed, name) {
					t.Errorf("%s 里又定义了 %s —— CMD 1722 双向都没有继承结果通道，任何回包构造函数都不许存在", e.Name(), name)
				}
			}
		}
	}
}
