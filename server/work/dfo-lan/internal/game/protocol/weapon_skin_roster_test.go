package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCharacterListWeaponSkinNativeFieldPreservesOtherBytes(t *testing.T) {
	for _, model := range []uint32{0, 123} {
		row := CharacterRow{Name: "测试角色", Level: 115, Profession: 16, Advancement: 3, FixedSlot: 2, AuraVisible: true, Fame: 34319, FatigueRemaining: 156, FatigueBonus: 27, ContentClearFlags: [28]byte{1, 2, 3}, Equipment: []Equipment{{Slot: 10, Item: 517600023, Model: model}, {Slot: 12, Item: 117020002, UpgradeLevel: 12}}}
		for _, mode := range []byte{2, 13} {
			build := func(r CharacterRow) ([]byte, error) {
				if mode == 2 {
					return CharacterList(8, []CharacterRow{r})
				}
				return AllServerCharacterList(4, 8, []CharacterRow{r})
			}
			before, err := build(row)
			if err != nil {
				t.Fatal(err)
			}
			row.WeaponSkinID = 0x11223344
			after, err := build(row)
			if err != nil {
				t.Fatal(err)
			}
			trailer := 0
			if mode == 2 {
				trailer = 10
			}
			off := len(after) - trailer - 33
			if len(after) != len(before) || binary.LittleEndian.Uint32(after[off:]) != row.WeaponSkinID {
				t.Fatal("native scalar offset/width changed")
			}
			if !bytes.Equal(before[:off], after[:off]) || !bytes.Equal(before[off+4:], after[off+4:]) {
				t.Fatal("roster changed outside native override scalar")
			}
			row.WeaponSkinID = 0
			clear, err := build(row)
			if err != nil || !bytes.Equal(clear, before) {
				t.Fatal("clear did not restore original bytes")
			}
		}
	}
}
