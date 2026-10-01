package character

import (
	"dfolan/internal/savecontract"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// 五个校验过的 115US 建号请求布局之一：职业 0、12 字节 option、option[8]=1、option[10]=0。
const repairCreateRequest = "000400000063657a7a000000000000ff0001000000000000"

func repairService(t *testing.T, prof catalog.Profession, rows ...inventory.EquipmentDefinition) *Service {
	t.Helper()
	// 转职槽 1 在源里有成长段，applyCreationAdvancement 才会落账。
	prof.AdvancementGrowth = map[byte]map[string]float32{1: {"[hp max]": 100}}
	return &Service{
		Catalog:   catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: creationSum}, Professions: map[byte]catalog.Profession{0: prof}},
		Rules:     Rules{AllJobsPilot: true, InitialLevel: 1},
		Equipment: creationCatalog(t, rows...),
		WearRules: creationWearRules(),
	}
}

func repairRole(t *testing.T, advancement byte, worn []inventory.BagEquipment) storage.Character {
	t.Helper()
	req, e := hex.DecodeString(repairCreateRequest)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(State{Level: 1, Advancement: advancement})
	if e != nil {
		t.Fatal(e)
	}
	if len(worn) > 0 {
		raw, e = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn})
		if e != nil {
			t.Fatal(e)
		}
	}
	return storage.Character{ID: 6, Name: "cezz", Profession: 0, Request: req, ConfigVersion: savecontract.Identity(), State: raw}
}

// 老角色：当时目录没有 growtype 数据，advancement 停在 0、身上也没装备。
// 修复要按建号请求补转职落账，再按同一槽补上六件初始穿戴；并且可重放（第二次为 no-op）。
func TestCreationPreviewRepairsLegacyRole(t *testing.T) {
	svc := repairService(t, creationProfession(sixSlotCreateEquipment()), sixSlotRows()...)
	role := repairRole(t, 0, nil)
	next, changes, err := svc.CreationPreview(role)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || len(changes) != 7 { // 1 条转职落账 + 6 件穿戴
		t.Fatalf("changes: %v", changes)
	}
	var state State
	if e := json.Unmarshal(next, &state); e != nil {
		t.Fatal(e)
	}
	if state.Advancement != 1 || !state.AllJobsPilot || state.EquipmentPending {
		t.Fatalf("state not repaired: %+v", state)
	}
	bag, e := inventory.ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Worn) != 6 {
		t.Fatalf("worn: %+v", bag.Worn)
	}
	// 幂等：对已修好的状态再跑一次必须没有变化。
	repaired := role
	repaired.State = next
	if again, changes, e := svc.CreationPreview(repaired); e != nil || again != nil || len(changes) != 0 {
		t.Fatalf("second pass must be a no-op: next=%v changes=%v err=%v", again, changes, e)
	}
}

// 已有同槽穿戴保留（不覆盖、不重置），只补缺的那些槽。
func TestCreationPreviewKeepsExistingWorn(t *testing.T) {
	svc := repairService(t, creationProfession(sixSlotCreateEquipment()), sixSlotRows()...)
	role := repairRole(t, 1, []inventory.BagEquipment{{Slot: 12, Template: 999, Durability: 7}})
	next, changes, err := svc.CreationPreview(role)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || len(changes) != 5 {
		t.Fatalf("只应补武器槽之外的 5 件: %v", changes)
	}
	bag, e := inventory.ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	var kept bool
	for _, w := range bag.Worn {
		if w.Slot == 12 {
			kept = w.Template == 999 && w.Durability == 7
		}
	}
	if !kept {
		t.Fatalf("已有穿戴被覆盖: %+v", bag.Worn)
	}
	if len(bag.Worn) != 6 {
		t.Fatalf("worn: %+v", bag.Worn)
	}
}

// 目录版本不一致的角色拒绝修复（避免用错代的源数据投影）。
func TestCreationPreviewRejectsSourceMismatch(t *testing.T) {
	svc := repairService(t, creationProfession(sixSlotCreateEquipment()), sixSlotRows()...)
	role := repairRole(t, 0, nil)
	role.ConfigVersion = "5555555555555555555555555555555555555555555555555555555555555555"
	if _, _, err := svc.CreationPreview(role); err == nil {
		t.Fatal("source mismatch must be refused")
	}
	// 没有建号请求、且 advancement 已经是 0 且槽 0 为空的角色没有任何可补内容。
	noRequest := repairRole(t, 0, nil)
	noRequest.Request = nil
	if next, changes, err := svc.CreationPreview(noRequest); err != nil || next != nil || len(changes) != 0 {
		t.Fatalf("nothing to repair: next=%v changes=%v err=%v", next, changes, err)
	}
}
