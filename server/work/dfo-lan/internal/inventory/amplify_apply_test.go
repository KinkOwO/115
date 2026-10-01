package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

// 这一组用例盯住「增幅书打红字」的**业务落地**：它是 CMD205 那条链路里唯一
// 不需要数据库的部分（applyAmplifyGrimoire 只用 s.Catalog + 角色存档里的背包 JSON），
// 所以可以在普通 `go test` 里跑完整条业务逻辑，不必等 PG。
//
// 起因：这条链路曾经「服务层 + 流程层都在、就是没人调」——客户端发 205 之后
// 服务端既不处理也不回包，实机表现就是「增幅书打了没效果」。现在补了分派
// （cmd/wireprobe/main.go 的 frame.ID == 205 + wiring 用例），这里再钉住落库结果。
const (
	amplifyTestGearSlot   = 25
	amplifyTestBookSlot   = 77
	amplifyTestEmptySlot  = 78
	amplifyTestGearTmpl   = 900000001 // 本用例专用，注入到装备目录
	amplifyTestGoldenTmpl = 50022037  // 真实黄金增幅书（配置里权重 8→32 9→40 10→25 11→2 12→1）
	amplifyTestSilverTmpl = 10356325  // 玩家背包里那本白银增幅书（3→40 4→30 5→20 6→10）
	amplifyTestRefineSeal = 0x40      // offset 10 的 bit5-7 = 再封装次数（这里是 2）
)

// amplifyApplyFixture 装一件可增幅装备 + 一本增幅书。
// gearLevel/priorType 直接写进装备行，用来验证「已有红字 / 已有强化等级」的情形。
func amplifyApplyFixture(t *testing.T, gearLevel, priorType byte, bookAmount uint32, bookTmpl uint32) (*WearService, storage.Character) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	eq.index[amplifyTestGearTmpl] = EquipmentDefinition{
		ID:     amplifyTestGearTmpl,
		Path:   "equipment/character/common/jacket/cloth/amplify_test.equ",
		SHA256: strings.Repeat("a", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "coat"}},
		},
	}
	rec := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(rec[2:], amplifyTestGearTmpl)
	rec[amplifyReinforceOffset] = gearLevel | amplifyTestRefineSeal
	rec[amplifyTypeOffset] = priorType
	bag := Bag{
		Version:   "ordinary-bag-v1",
		Items:     []BagItem{{Slot: amplifyTestBookSlot, Template: bookTmpl, Amount: bookAmount}},
		Equipment: []BagEquipment{{Slot: amplifyTestGearSlot, Template: amplifyTestGearTmpl, Record: rec}},
	}
	state, e := SaveBag(json.RawMessage(`{"level":100,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq}, storage.Character{ID: 1, ConfigVersion: c.Source.SaveIdentity(), State: state}
}

func amplifyRequest(tmpl uint32) protocol.AmplifyOptionRequest {
	return protocol.AmplifyOptionRequest{
		Aux:               2,
		EquipmentSlot:     amplifyTestGearSlot,
		EquipmentTemplate: amplifyTestGearTmpl,
		BookSlot:          amplifyTestBookSlot,
		BookTemplate:      tmpl,
		Type:              3,
	}
}

func amplifyGearRow(t *testing.T, state json.RawMessage) []byte {
	t.Helper()
	bag, e := ReadBag(state)
	if e != nil {
		t.Fatal(e)
	}
	for _, gear := range bag.Equipment {
		if gear.Slot == amplifyTestGearSlot {
			row := EquipmentRow(gear)
			return row[:]
		}
	}
	t.Fatal("打完红字后找不到那件装备")
	return nil
}

// 黄金增幅书：摇值即等级 —— offset 10（等级低五位）、offset 19（红字类型）、
// offset 20（红字数值）三个字节必须一起落，且不能破坏再封装次数。
func TestApplyAmplifyGrimoireGoldenWritesLevelTypeAndValue(t *testing.T) {
	loadGrimoiresForTest(t)
	if !IsGoldenGrimoire(amplifyTestGoldenTmpl) {
		t.Fatalf("%d 应是黄金增幅书", amplifyTestGoldenTmpl)
	}
	if v, ok := AmplifyGrimoireRollValue(amplifyTestGoldenTmpl); !ok || v < 8 || v > 12 {
		t.Fatalf("黄金增幅书摇值应在 8..12，实际 %d ok=%v", v, ok)
	}
	// 原强化 +12，书摇到 10：等级必须被覆盖成 10，且再封装次数（bit5-7）保持不动。
	svc, role := amplifyApplyFixture(t, 12, 0, 3, amplifyTestGoldenTmpl)
	next, out, err := svc.applyAmplifyGrimoire(role, amplifyRequest(amplifyTestGoldenTmpl), 10, true, false)
	if err != nil {
		t.Fatalf("黄金增幅书落库失败: %v", err)
	}
	if out.AmplifyLevel != 10 || out.AmplifyValue != 10 || out.AmplifyType != 3 || !out.Golden {
		t.Fatalf("回执不符: level=%d value=%d type=%d golden=%v", out.AmplifyLevel, out.AmplifyValue, out.AmplifyType, out.Golden)
	}
	if out.PrevReinforceLevel != 12 || out.ReAmplified {
		t.Fatalf("首次打红字应记下原强化 12 且 reAmplified=false，实际 %d/%v", out.PrevReinforceLevel, out.ReAmplified)
	}
	if out.BookRemaining != 2 {
		t.Fatalf("书剩余应 3→2，实际 %d", out.BookRemaining)
	}
	row := amplifyGearRow(t, next)
	if got := row[amplifyReinforceOffset] & reinforceLevelMask; got != 10 {
		t.Errorf("offset10 等级 = %d，期望 10（不能写进 offset20）", got)
	}
	if got := row[amplifyReinforceOffset] >> 5; got != 2 {
		t.Errorf("再封装次数被破坏：%#02x", row[amplifyReinforceOffset])
	}
	if row[amplifyTypeOffset] != 3 {
		t.Errorf("offset19 红字类型 = %d，期望 3", row[amplifyTypeOffset])
	}
	if row[amplifyValueOffset] != 10 {
		t.Errorf("offset20 红字数值 = %d，期望 10", row[amplifyValueOffset])
	}
	if got := amplifyLevel(row); got != 10 {
		t.Errorf("amplifyLevel 读回 %d，期望 10", got)
	}
}

// 黄金增幅书的官方描述禁止「扭转成与之前相同的属性」。这条拒绝不能误伤白银书：
// 普通/白银/强烈书对已有红字装备重打（含同类型）是允许的。
func TestApplyAmplifyGrimoireGoldenRejectsSameType(t *testing.T) {
	loadGrimoiresForTest(t)
	svc, role := amplifyApplyFixture(t, 12, 3, 3, amplifyTestGoldenTmpl) // 已有红字类型 3 = 力量
	_, _, err := svc.applyAmplifyGrimoire(role, amplifyRequest(amplifyTestGoldenTmpl), 10, true, false)
	if err == nil {
		t.Fatal("黄金书对同类型红字应拒绝")
	}
	if !strings.Contains(err.Error(), "无法选择与之前相同") {
		t.Fatalf("拒绝原因不符: %v", err)
	}
	// 同类型对非黄金书必须放行，并且照常落新等级（槽里换成白银书）。
	svc, role = amplifyApplyFixture(t, 12, 3, 3, amplifyTestSilverTmpl)
	next, out, err := svc.applyAmplifyGrimoire(role, amplifyRequest(amplifyTestSilverTmpl), 5, false, false)
	if err != nil {
		t.Fatalf("白银书同类型重打应放行: %v", err)
	}
	if !out.ReAmplified || out.PrevAmplifyType != 3 {
		t.Fatalf("应记录重打与原类型 3，实际 %v/%d", out.ReAmplified, out.PrevAmplifyType)
	}
	if got := amplifyGearRow(t, next)[amplifyReinforceOffset] & reinforceLevelMask; got != 5 {
		t.Errorf("重打后等级 = %d，期望 5", got)
	}
}

// 扣书必须按槽位 + 模板双字段核对：槽里换了别的东西不能被当成这本书扣掉，
// 槽里没有书也要明确拒绝。数量扣到 0 时整行移除。
func TestApplyAmplifyGrimoireConsumesOnlyTheNamedBook(t *testing.T) {
	loadGrimoiresForTest(t)
	svc, role := amplifyApplyFixture(t, 12, 0, 1, amplifyTestGoldenTmpl)
	// 槽里有书，但模板不是请求里那本。
	if _, _, err := svc.applyAmplifyGrimoire(role, amplifyRequest(amplifyTestSilverTmpl), 5, false, false); err == nil ||
		!strings.Contains(err.Error(), "槽位与模板不符") {
		t.Fatalf("槽位/模板不符应拒绝，实际 %v", err)
	}
	// 槽里根本没书。
	req := amplifyRequest(amplifyTestGoldenTmpl)
	req.BookSlot = amplifyTestEmptySlot
	if _, _, err := svc.applyAmplifyGrimoire(role, req, 10, true, false); err == nil ||
		!strings.Contains(err.Error(), "不在背包") {
		t.Fatalf("空槽应拒绝，实际 %v", err)
	}
	// 正常路径：数量 1 → 整行移除。
	next, out, err := svc.applyAmplifyGrimoire(role, amplifyRequest(amplifyTestGoldenTmpl), 10, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.BookRemaining != 0 {
		t.Fatalf("书剩余应 0，实际 %d", out.BookRemaining)
	}
	bag, e := ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range bag.Items {
		if item.Slot == amplifyTestBookSlot {
			t.Fatalf("书用完应整行移除，仍存在: %+v", item)
		}
	}
	// 装备必须还在（书被扣掉不能连装备一起丢）。
	if got := amplifyGearRow(t, next)[amplifyReinforceOffset] & reinforceLevelMask; got != 10 {
		t.Errorf("装备等级 = %d，期望 10", got)
	}
}
