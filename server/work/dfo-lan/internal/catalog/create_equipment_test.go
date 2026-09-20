package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestParseCreateEquipmentGroupsByLabelAndSlot(t *testing.T) {
	cells := []pvf.Token{
		{Type: 6, Text: "[weapon]"},
		{Type: 0, Value: 0},
		{Type: 0, Value: 401040091},
		{Type: 0, Value: -1}, // 负值不是模板，跳过且不占槽
		{Type: 0, Value: 101010912},
		{Type: 6, Text: "[coat]"},
		{Type: 0, Value: 400070174},
	}
	bySlot, order := ParseCreateEquipment(cells)
	if len(order) != 2 || order[0] != "[weapon]" || order[1] != "[coat]" {
		t.Fatalf("order: %+v", order)
	}
	if bySlot["[weapon]"][0] != 0 || bySlot["[weapon]"][1] != 401040091 || bySlot["[weapon]"][2] != 101010912 {
		t.Fatalf("weapon slots: %+v", bySlot["[weapon]"])
	}
	if bySlot["[coat]"][0] != 400070174 || len(bySlot["[coat]"]) != 1 {
		t.Fatalf("coat slots: %+v", bySlot["[coat]"])
	}
	if other, order := ParseCreateEquipment(nil); other != nil || order != nil {
		t.Fatalf("missing section must stay empty: %+v %+v", other, order)
	}
}

// 目录文件只带原始单元时，加载阶段要补出按槽投影，槽序必须等于源顺序。
func TestLoadCharactersBackfillsCreateEquipmentSlots(t *testing.T) {
	c, e := LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	prof, ok := c.Professions[0]
	if !ok {
		t.Fatal("profession 0 missing")
	}
	weapon := prof.CreateEquipmentBySlot["[weapon]"]
	if weapon[0] != 0 || weapon[1] != 401040091 || weapon[5] != 101010912 {
		t.Fatalf("weapon slots: %+v", weapon)
	}
	if len(prof.CreateEquipmentOrder) != 6 || prof.CreateEquipmentOrder[0] != "[weapon]" || prof.CreateEquipmentOrder[1] != "[shoulder]" {
		t.Fatalf("order: %+v", prof.CreateEquipmentOrder)
	}
	// 本身即高级职业的 .chr 只在槽 0 给出装备，其余槽为空。
	high, ok := c.Professions[9]
	if !ok {
		t.Fatal("profession 9 missing")
	}
	if high.CreateEquipmentBySlot["[weapon]"][0] == 0 || high.CreateEquipmentBySlot["[weapon]"][5] != 0 {
		t.Fatalf("demonic swordman weapon slots: %+v", high.CreateEquipmentBySlot["[weapon]"])
	}
}
