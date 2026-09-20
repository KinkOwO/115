package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const creationSum = "2222222222222222222222222222222222222222222222222222222222222222"

// 真实部位到穿戴槽的映射（configs/equipment-wear.*.json，native 依据 1470cb2a0）。
func creationWearRules() inventory.WearRules {
	return inventory.WearRules{Source: creationSum, Slots: map[string]uint16{
		"[weapon]": 12, "[coat]": 14, "[shoulder]": 15, "[pants]": 16,
		"[shoes]": 17, "[waist]": 18,
	}}
}

func creationPiece(id uint32, kind, job string, level, durability int32) inventory.EquipmentDefinition {
	return inventory.EquipmentDefinition{ID: id, Path: "equipment/test.equ", SHA256: creationSum, Fields: map[string][]pvf.Token{
		"[rarity]":         {{Type: 0, Value: 0}},
		"[equipment type]": {{Type: 6, Text: kind}},
		"[minimum level]":  {{Type: 0, Value: level}},
		"[usable job]":     {{Type: 6, Text: job}},
		"[durability]":     {{Type: 0, Value: durability}},
	}}
}

func creationCatalog(t *testing.T, rows ...inventory.EquipmentDefinition) *inventory.EquipmentCatalog {
	t.Helper()
	c := inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: creationSum}, Rows: rows}
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "equipment.json")
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	loaded, e := inventory.LoadEquipmentCatalog(path, creationSum)
	if e != nil {
		t.Fatal(e)
	}
	return loaded
}

func creationProfession(slots map[string]map[byte]uint32) catalog.Profession {
	order := make([]string, 0, len(slots))
	for _, label := range []string{"[weapon]", "[coat]", "[shoulder]", "[pants]", "[waist]", "[shoes]"} {
		if _, ok := slots[label]; ok {
			order = append(order, label)
		}
	}
	return catalog.Profession{ID: 0, Job: "[swordman]", RawSHA256: creationSum, CreateEquipmentBySlot: slots, CreateEquipmentOrder: order}
}

// 六个部位各给一个槽：槽 0 为空、槽 1 是完整的转职套装。
func sixSlotCreateEquipment() map[string]map[byte]uint32 {
	return map[string]map[byte]uint32{
		"[weapon]":   {0: 0, 1: 401040091},
		"[coat]":     {0: 0, 1: 400070174},
		"[shoulder]": {0: 0, 1: 400170168},
		"[pants]":    {0: 0, 1: 400120169},
		"[waist]":    {0: 0, 1: 400220168},
		"[shoes]":    {0: 0, 1: 400270172},
	}
}

func sixSlotRows() []inventory.EquipmentDefinition {
	return []inventory.EquipmentDefinition{
		creationPiece(401040091, "[weapon]", "[swordman]", 1, 60),
		creationPiece(400070174, "[coat]", "[swordman]", 1, 104),
		creationPiece(400170168, "[shoulder]", "[swordman]", 1, 144),
		creationPiece(400120169, "[pants]", "[swordman]", 1, 124),
		creationPiece(400220168, "[waist]", "[swordman]", 1, 164),
		creationPiece(400270172, "[shoes]", "[swordman]", 1, 184),
	}
}

func TestCreationWornProjectsSelectedGrowthSlot(t *testing.T) {
	prof := creationProfession(sixSlotCreateEquipment())
	s := &Service{Catalog: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}}, Equipment: creationCatalog(t, sixSlotRows()...), WearRules: creationWearRules()}
	worn := s.creationWorn(prof, 1, 1)
	if len(worn) != 6 {
		t.Fatalf("expected six worn pieces, got %+v", worn)
	}
	bySlot := map[uint16]inventory.BagEquipment{}
	for _, w := range worn {
		bySlot[w.Slot] = w
	}
	// 逐件等于源 [create equipment list] 在该转职槽的值，不做任何职业特例。
	for slot, template := range map[uint16]uint32{12: 401040091, 14: 400070174, 15: 400170168, 16: 400120169, 18: 400220168, 17: 400270172} {
		if bySlot[slot].Template != template {
			t.Fatalf("slot %d: got %+v, want template %d", slot, bySlot[slot], template)
		}
	}
	if bySlot[12].Durability != 60 || bySlot[18].Durability != 164 {
		t.Fatalf("durability must come from the equipment source: %+v", bySlot)
	}
}

func TestCreationWornSkipsEmptyAndUnavailablePieces(t *testing.T) {
	slots := sixSlotCreateEquipment()
	slots["[coat]"] = map[byte]uint32{1: 400070174}
	delete(slots, "[shoes]") // 源没有该部位 → 不投影、不报错
	rows := sixSlotRows()
	rows[1] = creationPiece(400070174, "[coat]", "[fighter]", 1, 104)    // 职业不符
	rows[3] = creationPiece(400120169, "[pants]", "[swordman]", 50, 124) // 等级不符
	rows[4] = creationPiece(400220168, "[shoes]", "[swordman]", 1, 164)  // 部位与槽标签不符
	prof := creationProfession(slots)
	s := &Service{Catalog: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}}, Equipment: creationCatalog(t, rows...), WearRules: creationWearRules()}
	worn := s.creationWorn(prof, 1, 1)
	if len(worn) != 2 {
		t.Fatalf("expected weapon and shoulder only, got %+v", worn)
	}
	for _, w := range worn {
		if w.Template != 401040091 && w.Template != 400170168 {
			t.Fatalf("unexpected piece survived: %+v", w)
		}
	}
}

func TestCreationWornUsesAdvancementZeroSlot(t *testing.T) {
	// 本身即高级职业的 .chr（例如 demonic swordman）只在槽 0 给出装备。
	slots := map[string]map[byte]uint32{"[weapon]": {0: 101010912}}
	rows := []inventory.EquipmentDefinition{creationPiece(101010912, "[weapon]", "[swordman]", 1, 40)}
	prof := creationProfession(slots)
	s := &Service{Catalog: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}}, Equipment: creationCatalog(t, rows...), WearRules: creationWearRules()}
	if worn := s.creationWorn(prof, 0, 1); len(worn) != 1 || worn[0].Slot != 12 || worn[0].Template != 101010912 {
		t.Fatalf("slot 0 must project for a profession that has no growth branches: %+v", worn)
	}
	if worn := s.creationWorn(prof, 1, 1); len(worn) != 0 {
		t.Fatalf("advancement 1 has no source data and must stay empty: %+v", worn)
	}
}

func TestCreationWornSkipsMissingDurabilityAndMissingDependencies(t *testing.T) {
	slots := map[string]map[byte]uint32{"[weapon]": {1: 401040091}, "[coat]": {1: 400070174}}
	weapon := creationPiece(401040091, "[weapon]", "[swordman]", 1, 60)
	coat := creationPiece(400070174, "[coat]", "[swordman]", 1, 104)
	delete(coat.Fields, "[durability]") // 源里没有耐久的部位一律跳过，不猜
	prof := creationProfession(slots)
	eq := creationCatalog(t, weapon, coat)
	chars := catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}}
	s := &Service{Catalog: chars, Equipment: eq, WearRules: creationWearRules()}
	if worn := s.creationWorn(prof, 1, 1); len(worn) != 1 || worn[0].Template != 401040091 {
		t.Fatalf("piece without source durability must be skipped: %+v", worn)
	}
	for name, svc := range map[string]*Service{
		"no equipment catalog": {Catalog: chars, WearRules: creationWearRules()},
		"no wear rules":        {Catalog: chars, Equipment: eq},
		"foreign source": {
			Catalog:   catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}},
			Equipment: eq,
			WearRules: inventory.WearRules{Source: "3333333333333333333333333333333333333333333333333333333333333333", Slots: map[string]uint16{"[weapon]": 12}},
		},
	} {
		if worn := svc.creationWorn(prof, 1, 1); len(worn) != 0 {
			t.Fatalf("%s must not project: %+v", name, worn)
		}
	}
	// 只认与 Catalog 同一份源快照的装备目录。
	profession := creationProfession(slots)
	foreign := catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: "4444444444444444444444444444444444444444444444444444444444444444"}}
	other := &Service{Catalog: foreign, Equipment: eq, WearRules: creationWearRules()}
	if worn := other.creationWorn(profession, 1, 1); len(worn) != 0 {
		t.Fatalf("character/equipment source mismatch must not project: %+v", worn)
	}
}
