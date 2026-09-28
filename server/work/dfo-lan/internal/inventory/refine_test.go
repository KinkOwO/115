package inventory

// 锻造（Refine / CMD430）的回归测试。
//
// 规则数据来自 configs/refine.json：成功率由服主提供（115 版本），
// 材料消耗 PVF 无表、走配置默认值；行内偏移 record_offset 仍待实机校准。

import (
	"dfolan/internal/game/protocol"
	"path/filepath"
	"strings"
	"testing"
)

func loadRefineRulesForTest(t *testing.T) {
	t.Helper()
	if refineRules != nil {
		return
	}
	path := filepath.Join("..", "..", "configs", "refine.json")
	if err := LoadRefineRules(path); err != nil {
		t.Fatalf("装载锻造规则失败: %v", err)
	}
	if !RefineRulesLoaded() {
		t.Skip("configs/refine.json 不存在，跳过锻造用例")
	}
}

// 材料：Powerful Energy 3326；强化材料 3037（无色小晶块）不能被当成锻造材料。
func TestRefineMaterial(t *testing.T) {
	loadRefineRulesForTest(t)
	if !IsRefineMaterial(3326) {
		t.Error("3326（Powerful Energy）应被识别为锻造材料")
	}
	if IsRefineMaterial(3037) || IsRefineMaterial(3242) {
		t.Error("3037 / 3242 是强化与增幅材料，不该被当成锻造材料")
	}
	if got := RefineMaterialTemplate(); got != 3326 {
		t.Errorf("材料模板 = %d，期望 3326", got)
	}
}

// 成功率表（服主提供的 115 神界原版）：+0~+2 必成，之后每级递减 10。
func TestRefineSuccessRate(t *testing.T) {
	loadRefineRulesForTest(t)
	want := map[int]int{0: 100, 1: 100, 2: 100, 3: 80, 4: 70, 5: 60, 6: 50, 7: 40}
	for level, rate := range want {
		if got := RefineSuccessPercent(level); got != rate {
			t.Errorf("+%d 成功率 = %d，期望 %d", level, got, rate)
		}
	}
	// 上限 Refine 8，取不到成功率也不该取到消耗。
	if got := RefineMaxLevel(); got != 8 {
		t.Errorf("上限 = %d，期望 8", got)
	}
	if _, ok := RefineMaterialCount(RefineMaxLevel()); ok {
		t.Error("已满级不该再取到材料消耗")
	}
}

// 单次消耗 = 7 *（等级 + 1）：+0→1 收 7 个 Powerful Energy，+7→8 收 56 个。
// ★ 成功失败都扣（官方：失败只是没提升，本次提交的气息照样消耗）。
func TestRefineMaterialCountIsSevenPerLevel(t *testing.T) {
	loadRefineRulesForTest(t)
	want := map[int]int{0: 7, 1: 14, 2: 21, 3: 28, 4: 35, 5: 42, 6: 49, 7: 56}
	for level, count := range want {
		got, ok := RefineMaterialCount(level)
		if !ok {
			t.Fatalf("+%d 取不到材料消耗", level)
		}
		if got != uint32(count) {
			t.Errorf("+%d 消耗 = %d，期望 %d", level, got, count)
		}
	}
}

// ★ 锻造不收金币 —— 与强化/增幅不一样，客户端锻造面板根本没有金币一栏。
// 业务侧也不允许出现任何扣金逻辑，这条用例把「配置声明」和「代码不扣钱」一起钉住。
func TestRefineChargesNoGold(t *testing.T) {
	loadRefineRulesForTest(t)
	if got := RefineGoldCost(); got != 0 {
		t.Errorf("锻造金币费用 = %d，期望 0（原版锻造不收金币）", got)
	}
}

// 锻造等级读写：服务端状态 BagEquipment.Refine 是权威值，行内那格只是镜像。
//
// 为什么必须有服务端状态：行内偏移还没实机校准，一旦改了偏移，只靠行内字节读会把
// 玩家已经锻出来的等级读成 0（或读到上一次写歪的残留值）。
func TestRefineLevelServerAuthoritative(t *testing.T) {
	loadRefineRulesForTest(t)
	var row [protocol.CurrentItemRecordSize]byte
	off := RefineRecordOffset()
	if off < 0 || off >= protocol.CurrentItemRecordSize {
		t.Fatalf("行内偏移 %d 越界", off)
	}

	// 1) 从没锻过：行内字节当权威（兼容手工改过的旧存档）。
	row[off] = 3
	if got := refineLevel(BagEquipment{}, row[:]); got != 3 {
		t.Errorf("首次读取 = %d，期望 3", got)
	}
	// 2) 服务端状态存在时以它为准：行内残留写歪了也不影响判定。
	gear := BagEquipment{Refine: 5}
	row[off] = 1
	if got := refineLevel(gear, row[:]); got != 5 {
		t.Errorf("服务端状态应优先，得到 %d，期望 5", got)
	}
	// 3) 写入同时落状态与行内镜像。
	setRefineLevel(&gear, row[:], 6)
	if gear.Refine != 6 || row[off] != 6 {
		t.Errorf("写入后 gear.Refine=%d row[%d]=%d，期望都是 6", gear.Refine, off, row[off])
	}
	// 4) 超过上限要被夹住（协议层对结果码与等级自洽还有一道校验）。
	setRefineLevel(&gear, row[:], 20)
	if gear.Refine != byte(RefineMaxLevel()) || row[off] != byte(RefineMaxLevel()) {
		t.Errorf("超限写入 gear.Refine=%d row[%d]=%d，期望夹到 %d",
			gear.Refine, off, row[off], RefineMaxLevel())
	}
	// 5) 行内字节超过上限时不该被当成有效等级（防止读到别的语义的字节）。
	row[off] = 200
	if got := refineLevel(BagEquipment{}, row[:]); got != 0 {
		t.Errorf("行内 200 应判为无效等级，得到 %d", got)
	}
}

// 失败结果：官方规则是「等级不变、装备不碎」，所以只会回 result=1 且 old == new。
// protocol.RefineReply 对这两条有硬校验，这里守住业务侧不会产出别的组合。
func TestRefineReplyFailureKeepsLevel(t *testing.T) {
	loadRefineRulesForTest(t)
	ack, err := protocol.RefineReply(141, 84, 3, 3, 1, 3, 12)
	if err != nil {
		t.Fatalf("失败回包构造失败: %v", err)
	}
	if len(ack) != 13 || ack[7] != 3 || ack[8] != 1 || ack[9] != 3 {
		t.Fatalf("失败回包布局不对: % x", ack)
	}
	if _, err = protocol.RefineReply(141, 84, 3, 2, 1, 3, 12); err == nil {
		t.Error("失败不变时新等级必须等于旧等级，构造应当报错")
	}
	if _, err = protocol.RefineReply(141, 84, 3, 5, 0, 3, 12); err == nil {
		t.Error("成功时新等级必须是旧等级 +1，构造应当报错")
	}
}

// 非武器拒绝文案要能被玩家看懂（dstr 35128 "Items that are not weapons cannot be refined."）。
func TestRefineRejectsNonWeapon(t *testing.T) {
	loadRefineRulesForTest(t)
	msg := "只有武器可以锻造（当前是 [armor]）"
	if !strings.Contains(msg, "只有武器") {
		t.Errorf("非武器拒绝文案不对: %q", msg)
	}
}
