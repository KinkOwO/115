package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// weaponSkinBag is a katana on worn slot 12, a coat on slot 14, and the skin the
// player applied in the replication window on file.
const (
	testWeaponTemplate = 101010438
	testSkinTemplate   = 101010912
	testCoatTemplate   = 400070177
)

func weaponSkinState(t *testing.T, bag inventory.Bag) json.RawMessage {
	t.Helper()
	bag.Version = "ordinary-bag-v1"
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	return append(state[:len(state)-1], []byte(`,"advancement":0}`)...)
}

// The applied weapon skin has to reach the client through the two projections
// that carry per-slot models, and both must agree:
//
//   - the post-apply refresh (AppearanceProbe) binds Placeholder and Model to
//     the skin - the model cell is [actor+slot*4+0x405], the slot's real
//     appearance source;
//   - entry (EntryBasicProbe -> EquipmentAppearance) writes the skin into the
//     row's placeholder, which is what the town model lookup
//     (145BEFD60 -> 145BD63D0 -> 145BEE6C0) reads, so the skin survives a
//     relog.
func TestAppearanceProbeOverridesWeaponSlotWithSkin(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		WeaponSkin: testSkinTemplate,
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: testWeaponTemplate},
			{Slot: 14, Template: testCoatTemplate},
		},
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(storage.Character{
		WireID: 1, Name: "LanSkin01", Profession: 0,
		State: weaponSkinState(t, bag),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	const at = 176 + len("LanSkin01")
	if count := int(got[at]); count != 2 {
		t.Fatalf("appearance count=%d, want 2 (weapon and coat rows)", count)
	}
	// Slot 12 is rewritten; slot 14 keeps the piece's own template and, being
	// outside the placeholder set, still carries a zero placeholder.
	if slot := got[at+1]; slot != 12 {
		t.Fatalf("row 0 slot=%d, want 12", slot)
	}
	if ph := binary.LittleEndian.Uint32(got[at+2:]); ph != testSkinTemplate {
		t.Fatalf("weapon placeholder=%d, want the skin %d", ph, testSkinTemplate)
	}
	if model := binary.LittleEndian.Uint32(got[at+10:]); model != testSkinTemplate {
		t.Fatalf("weapon model=%d, want the skin %d", model, testSkinTemplate)
	}
	pos := at + 1 + 39
	if slot := got[pos]; slot != 14 {
		t.Fatalf("row 1 slot=%d, want 14", slot)
	}
	if model := binary.LittleEndian.Uint32(got[pos+9:]); model != testCoatTemplate {
		t.Fatalf("coat model=%d, want %d", model, testCoatTemplate)
	}
}

// The entry projection is the other half of the same override: the row's item
// cell is what the town model lookup reads, so it must name the skin too.
func TestEntryBasicProbeOverridesWeaponSlotWithSkin(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		WeaponSkin: testSkinTemplate,
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: testWeaponTemplate},
		},
	}
	s := &Service{Catalog: professions}
	got, e := s.EntryBasicProbe(storage.Character{
		WireID: 1, Name: "LanSkin02", Profession: 0,
		State: weaponSkinState(t, bag),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	const at = 176 + len("LanSkin02")
	if count := int(got[at]); count != 1 {
		t.Fatalf("entry appearance count=%d, want 1", count)
	}
	if slot := int(got[at+1]); slot != 12 {
		t.Fatalf("entry appearance slot=%d, want 12", slot)
	}
	if item := binary.LittleEndian.Uint32(got[at+2:]); item != testSkinTemplate {
		t.Fatalf("entry weapon item=%d, want the skin %d", item, testSkinTemplate)
	}
}

// 佩戴路径的职业门（weaponSkinUsable）读的是存档里的职业与转职，而不是入场快照——
// 佩戴可能排在一次转职之后。它只为「本职业戴不上」返回 inventory.ErrWeaponSkinNotUsable：
// 调用方靠这个哨兵把拒绝与真正的故障分开。解除（skin 0）与缺目录必须永远放行，否则玩家
// 会卡在一件戴不上的外观里出不来。
func TestWeaponSkinUsableReadsTheSave(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("../../configs/equipment.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	var shell struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
	}
	if e = json.Unmarshal(raw, &shell); e != nil {
		t.Fatal(e)
	}
	// 这一份目录不带 Full（运行时的全量索引由 -equipment-full-catalog 提供），光剑在这里
	// 只有 [usable job]，所以这段覆盖的是「别的职业根本用不了」那一半。
	equipment, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", shell.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions, Equipment: equipment}
	state := weaponSkinState(t, inventory.Bag{})
	// 401040091 的 [usable job] 只列 [swordman] / [demonic swordman] / [at swordman]。
	const beamsword uint32 = 401040091
	byJob := func(job string) (byte, bool) {
		for id, p := range professions.Professions {
			if p.Job == job {
				return id, true
			}
		}
		return 0, false
	}
	own, ok := byJob("[swordman]")
	if !ok {
		t.Skip("the catalog has no swordman profession")
	}
	if err := s.weaponSkinUsable(storage.Character{Profession: own, State: state}, beamsword); err != nil {
		t.Fatalf("the weapon's own job was refused: %v", err)
	}
	foreign, ok := byJob("[gunner]")
	if !ok {
		t.Skip("the catalog has no gunner profession")
	}
	err := s.weaponSkinUsable(storage.Character{Profession: foreign, State: state}, beamsword)
	if !errors.Is(err, inventory.ErrWeaponSkinNotUsable) {
		t.Fatalf("gunner wearing a beamsword: %v, want ErrWeaponSkinNotUsable", err)
	}
	if err := s.weaponSkinUsable(storage.Character{Profession: foreign, State: state}, 0); err != nil {
		t.Fatalf("unapply was refused: %v", err)
	}
	// 目录缺失时跳过这道门：不能让缺目录变成"所有佩戴都失败"。
	bare := &Service{Catalog: professions}
	if err := bare.weaponSkinUsable(storage.Character{Profession: foreign, State: state}, beamsword); err != nil {
		t.Fatalf("a missing equipment catalog refused: %v", err)
	}
}

// A skin with no weapon worn must not invent a row: an empty weapon slot filled
// with a skin id makes the client put a weapon the character does not own into
// its hand.
func TestAppearanceProbeSkipsSkinWithoutWeapon(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		WeaponSkin: testSkinTemplate,
		Worn: []inventory.BagEquipment{
			{Slot: 14, Template: testCoatTemplate},
		},
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(storage.Character{
		WireID: 1, Name: "LanSkin03", Profession: 0,
		State: weaponSkinState(t, bag),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	const at = 176 + len("LanSkin03")
	if count := int(got[at]); count != 1 {
		t.Fatalf("appearance count=%d, want 1 (no weapon row)", count)
	}
	if slot := got[at+1]; slot != 14 {
		t.Fatalf("row slot=%d, want 14", slot)
	}
}
