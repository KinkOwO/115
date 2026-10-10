package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
)

func (s *Service) titleTemplate(template uint32) error {
	if s == nil || s.Equipment == nil {
		return fmt.Errorf("title source unavailable")
	}
	kind, err := s.Equipment.EquipmentKind(template)
	if err != nil {
		return err
	}
	if kind != "[title name]" {
		return fmt.Errorf("item is not a title")
	}
	return nil
}
func (s *Service) applyTitleBook(cur Character, r protocol.TitleBookTransfer, put bool, slots [2]uint16) (json.RawMessage, error) {
	if err := s.titleTemplate(r.Template); err != nil {
		return nil, err
	}
	if r.Category >= 5 || r.Slot > 65534 || r.BookSlot > 65534 || (r.Space != 0 && r.Space != 3) || (r.Space == 3 && r.Slot != 13) {
		return nil, fmt.Errorf("invalid title book location")
	}
	b, err := inventory.ReadBag(cur.State)
	if err != nil {
		return nil, err
	}
	if b.TitleBook == nil {
		b.TitleBook = map[byte][]inventory.TitleBookEntry{}
	}
	category := byte(r.Category)
	book := append([]inventory.TitleBookEntry(nil), b.TitleBook[category]...)
	ordinary := append([]inventory.BagEquipment(nil), b.Equipment...)
	if r.Space == 3 {
		ordinary = append([]inventory.BagEquipment(nil), b.Worn...)
	}
	findOrd, findBook := -1, -1
	for n, i := range ordinary {
		if uint32(i.Slot) == r.Slot && i.Group == 0 {
			findOrd = n
		}
	}
	for n, i := range book {
		if uint32(i.Equipment.Slot) == r.BookSlot {
			findBook = n
		}
	}
	if put {
		if findOrd < 0 || ordinary[findOrd].Template != r.Template || findBook >= 0 {
			return nil, fmt.Errorf("stale title source or occupied book slot")
		}
		item := ordinary[findOrd]
		item.Slot = uint16(r.BookSlot)
		book = append(book, inventory.TitleBookEntry{Equipment: item})
		ordinary = append(ordinary[:findOrd], ordinary[findOrd+1:]...)
	} else {
		if findBook < 0 || book[findBook].Equipment.Template != r.Template || findOrd >= 0 {
			return nil, fmt.Errorf("stale title book source or occupied destination")
		}
		if r.Space == 0 {
			for _, i := range b.Items {
				if uint32(i.Slot) == r.Slot {
					return nil, fmt.Errorf("title destination holds stackable")
				}
			}
			if slots[0] == 0 || r.Slot < uint32(slots[0]) || r.Slot > uint32(slots[1]) {
				return nil, fmt.Errorf("title destination outside equipment bag")
			}
		}
		item := book[findBook].Equipment
		item.Slot = uint16(r.Slot)
		ordinary = append(ordinary, item)
		book = append(book[:findBook], book[findBook+1:]...)
	}
	b.TitleBook[category] = book
	if r.Space == 3 {
		b.Worn = ordinary
	} else {
		b.Equipment = ordinary
	}
	if b.TitleSkin != 0 && !b.OwnsBookTitle(b.TitleSkin) {
		b.TitleSkin = 0
	}
	if b.TitleBookSeq == math.MaxUint64 {
		return nil, fmt.Errorf("title sequence exhausted")
	}
	b.TitleBookSeq++
	return inventory.SaveBag(cur.State, b)
}
func (s *Service) TransferTitleBook(ctx context.Context, role Character, key string, r protocol.TitleBookTransfer, put bool, slots [2]uint16) (Character, bool, error) {
	if s == nil || s.Store == nil || role.AccountID == 0 || role.ID == 0 || key == "" {
		return role, false, fmt.Errorf("title book requires owned character")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "title-book-transfer-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		next, err := s.applyTitleBook(cur, r, put, slots)
		return next, json.RawMessage(`{}`), err
	})
}
func (s *Service) ApplyTitleAppearance(ctx context.Context, role Character, key string, template uint32) (Character, bool, error) {
	if s == nil || s.Store == nil || role.AccountID == 0 || role.ID == 0 || key == "" {
		return role, false, fmt.Errorf("title appearance requires owned character")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "title-book-appearance-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		b, err := inventory.ReadBag(cur.State)
		if err != nil {
			return nil, nil, err
		}
		if template != 0 {
			if err = s.titleTemplate(template); err != nil {
				return nil, nil, err
			}
			if !b.OwnsBookTitle(template) {
				return nil, nil, fmt.Errorf("title appearance is not owned")
			}
		}
		if b.TitleBookSeq == math.MaxUint64 {
			return nil, nil, fmt.Errorf("title sequence exhausted")
		}
		b.TitleBookSeq++
		b.TitleSkin = template
		next, err := inventory.SaveBag(cur.State, b)
		return next, json.RawMessage(`{}`), err
	})
}
