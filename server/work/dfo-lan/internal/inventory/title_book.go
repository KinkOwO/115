package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
)

// Keep the complete equipment instance while it is in the book. Moving it
// never reconstructs the item from its template or loses its enchantment.
type TitleBookEntry struct {
	Equipment     BagEquipment `json:"equipment"`
	RandomOptions []byte       `json:"random_options,omitempty"`
	Flag          byte         `json:"flag,omitempty"`
}

func (b Bag) OwnsBookTitle(template uint32) bool {
	for _, items := range b.TitleBook {
		for _, i := range items {
			if i.Equipment.Template == template {
				return true
			}
		}
	}
	return false
}
func (b Bag) ValidateTitleBook() error {
	for category, items := range b.TitleBook {
		if category >= 5 {
			return fmt.Errorf("invalid saved title category")
		}
		seen := map[uint16]bool{}
		for _, i := range items {
			e := i.Equipment
			if e.Template == 0 || e.Group != 0 || seen[e.Slot] || len(i.RandomOptions) > 4 {
				return fmt.Errorf("invalid saved title book item")
			}
			seen[e.Slot] = true
			if err := e.ValidateRecord(); err != nil {
				return err
			}
		}
	}
	if b.TitleSkin != 0 && !b.OwnsBookTitle(b.TitleSkin) {
		return fmt.Errorf("saved title appearance is not owned")
	}
	return nil
}
func (b Bag) TitleBookPayload(category byte) ([]byte, error) {
	if err := b.ValidateTitleBook(); err != nil {
		return nil, err
	}
	rows := []protocol.TitleBookRow{}
	for _, i := range b.TitleBook[category] {
		r := EquipmentRow(i.Equipment)
		rows = append(rows, protocol.TitleBookRow{Slot: i.Equipment.Slot, Instance: binary.LittleEndian.Uint32(r[6:]), Template: i.Equipment.Template, Reinforcement: r[10], Durability: i.Equipment.Durability, Sealed: r[13], Enchant: binary.LittleEndian.Uint32(r[14:]), EnchantUpgrade: r[18], AmplifyType: r[19], AmplifyValue: binary.LittleEndian.Uint16(r[20:]), RandomOptions: i.RandomOptions, Clone: binary.LittleEndian.Uint32(r[24:]), Flag: i.Flag})
	}
	return protocol.TitleBookList(uint32(category), rows)
}
