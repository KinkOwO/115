package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestWeaponSkinTownOverrideWithWeaponAvatar(t *testing.T) {
	cat, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: cat}
	for _, cloned := range []bool{false, true} {
		for _, weaponWorn := range []bool{false, true} {
			for _, skin := range []uint32{0, 117020007, 117020019, 0} {
				avatar := inventory.BagEquipment{Slot: 10, Template: 117600026}
				bag := inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: skin, Worn: []inventory.BagEquipment{avatar}}
				if cloned {
					bag.Worn[0].Template = 117600012
					bag.Worn[0].CloneSource = &inventory.CloneAvatarSource{Slot: 39, Template: 117600026}
					bag.Special = map[byte][]inventory.BagEquipment{1: {{Slot: 39, Template: 117600026}}}
				}
				if weaponWorn {
					bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: 12, Template: 117020248})
				}
				raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":3,"awakening":3,"future":{"keep":true}}`), bag)
				if err != nil {
					t.Fatal(err)
				}
				role := Character{WireID: 1, Name: "Deemo", Profession: 16, State: raw}
				before := append([]byte(nil), role.State...)
				want := skin
				if !weaponWorn {
					want = 0
				}
				for name, project := range map[string]func(Character, [2]byte) ([]byte, error){"entry": s.EntryBasicProbe, "refresh": s.AppearanceProbe} {
					p, err := project(role, [2]byte{})
					if err != nil {
						t.Fatal(err)
					}
					// Native raw160 header starts at byte5; 14563FF79 reads
					// header+6c, independent of the variable appearance block.
					if got := binary.LittleEndian.Uint32(p[5+0x6c:]); got != want {
						t.Fatalf("%s cloned=%v weapon=%v skin=%d town override=%d want=%d", name, cloned, weaponWorn, skin, got, want)
					}
					if !bytes.Equal(before, role.State) {
						t.Fatal("projection changed stored equipment")
					}
				}
				rows, err := s.wornAppearance(raw)
				if err != nil {
					t.Fatal(err)
				}
				// The native override chooses the visible layer; real avatar,
				// clone source and appearance row remain available for clear.
				if len(rows) == 0 || rows[0].Slot != 10 || rows[0].Model != 117600026 {
					t.Fatalf("avatar source lost: %+v", rows)
				}
				full, last, err := inventory.CloneAvatarSourcePayloads(raw)
				if err != nil {
					t.Fatal(err)
				}
				wantSources := map[byte]uint16{}
				if cloned {
					wantSources[10] = 39
				}
				wf, wl, err := protocol.CloneAvatarSources(wantSources)
				if err != nil || !bytes.Equal(full, wf) || !bytes.Equal(last, wl) {
					t.Fatal("clone source changed")
				}
			}
		}
	}
}
