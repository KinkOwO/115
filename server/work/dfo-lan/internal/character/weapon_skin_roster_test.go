package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type weaponSkinRosterStore struct {
	Store
	role Character
}

func (s *weaponSkinRosterStore) Characters(context.Context, int64) ([]Character, error) {
	return []Character{s.role}, nil
}
func (*weaponSkinRosterStore) AccountUnifiedOptions(context.Context, int64) (map[uint16]uint16, error) {
	return map[uint16]uint16{1: 2}, nil
}
func (*weaponSkinRosterStore) CharacterUnifiedOptions(context.Context, int64) (map[uint16]uint16, error) {
	return map[uint16]uint16{98: 1}, nil
}
func (*weaponSkinRosterStore) CompletedQuestIDs(context.Context, int64, string, []uint16) (map[int64][]uint16, error) {
	return map[int64][]uint16{18: {12167, 13763}}, nil
}

func TestWeaponSkinRosterNativePriorityAndRestoration(t *testing.T) {
	cat, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	for profession := range cat.Professions {
		for _, clone := range []bool{false, true} {
			for _, worn := range []bool{false, true} {
				var previous []byte
				for _, skin := range []uint32{0, 117020002, 117020007, 0} {
					avatar := inventory.BagEquipment{Slot: 10, Template: 517600023}
					if clone {
						avatar.Template = 517600007
						avatar.CloneSource = &inventory.CloneAvatarSource{Slot: 0, Template: 517600023}
					}
					bag := inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: skin, Worn: []inventory.BagEquipment{avatar}, Special: map[byte][]inventory.BagEquipment{1: {{Slot: 0, Template: 517600023}}}}
					if worn {
						bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: 12, Template: 117020242})
					}
					raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":3,"awakening":3,"future":{"keep":true}}`), bag)
					if err != nil {
						t.Fatal(err)
					}
					before := append([]byte(nil), raw...)
					st := &weaponSkinRosterStore{role: Character{ID: 18, AccountID: 1, Name: "Deemo10110", Profession: profession, State: raw, FixedSlot: 3}}
					svc := &Service{Store: st, Catalog: cat, Rules: Rules{MaxCharacters: 32}, ChannelContext: [2]byte{4, 7}}
					p, err := svc.ListWithFatigue(context.Background(), 1, nil, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					all, ids, err := svc.AllServerRoster(context.Background(), 1, nil, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if len(ids) != 1 || ids[0] != 18 {
						t.Fatal("roster identity changed")
					}
					want := skin
					if !worn {
						want = 0
					}
					// Native 14563E96C reads u32 at the fixed tail: 29 bytes follow
					// this field; mode2 has ten enclosing footer bytes, mode13 none.
					for mode, packet := range map[string][]byte{"mode2": p, "mode13": all} {
						trailer := 0
						if mode == "mode2" {
							trailer = 10
						}
						off := len(packet) - trailer - 33
						if got := binary.LittleEndian.Uint32(packet[off:]); got != want {
							t.Fatalf("%s profession=%d clone=%v worn=%v skin=%d priority=%d want=%d", mode, profession, clone, worn, skin, got, want)
						}
						if binary.LittleEndian.Uint16(packet[off+4:]) != 1 || packet[off+6] != 3 {
							t.Fatal("aura or fixed slot shifted")
						}
						if dest := os.Getenv("DFO_ROSTER_TEST_FIXTURE_DIR"); dest != "" && profession == 16 && clone && worn {
							if err := os.WriteFile(filepath.Join(dest, fmt.Sprintf("%s-%d.bin", mode, skin)), packet, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					if !bytes.Equal(before, st.role.State) {
						t.Fatal("roster projection mutated persisted equipment/skin/clone state")
					}
					if skin == 0 {
						if previous != nil && !bytes.Equal(previous, p) {
							t.Fatal("clearing transmog failed to restore exact original roster")
						}
						previous = append([]byte(nil), p...)
					}
					if clone && avatar.CloneSource.Template != 517600023 {
						t.Fatal("clone source lost")
					}
				}
			}
		}
	}
}
