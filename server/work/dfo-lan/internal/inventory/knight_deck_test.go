package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"reflect"
	"testing"
)

func knightDeckTestService(t *testing.T) *WearService {
	t.Helper()
	jobs, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	shields, e := LoadKnightShields("../../configs/equipment-knight-shield.full-candidate.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	full, e := OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { full.Close() })
	rules, e := LoadWearRules("../../configs/equipment-wear.current35.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: &EquipmentCatalog{Source: jobs.Source, Full: full}, Professions: jobs, Rules: rules, Shields: shields}
}

func knightRole(t *testing.T, s *WearService, deck [5]uint32) Role {
	t.Helper()
	b := Bag{Version: "ordinary-bag-v1", KnightShieldDeck: append([]uint32(nil), deck[:]...)}
	if deck[0] != 0 {
		b.Worn = []BagEquipment{{Slot: 24, Template: deck[0]}}
	}
	raw, e := SaveBag(json.RawMessage(`{"level":90,"advancement":1}`), b)
	if e != nil {
		t.Fatal(e)
	}
	return Role{Profession: s.Shields.Profession, ConfigVersion: s.Catalog.Source.SaveIdentity(), State: raw}
}

func TestKnightShieldDeck(t *testing.T) {
	s := knightDeckTestService(t)
	t.Run("reserve swap mirrors worn and preserves unrelated inventory", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{113370003, 113370008, 113370026})
		b, _ := ReadBag(role.State)
		b.Gold = 123
		b.ExpandEquipFlags = 3
		b.WeaponSkins = []uint32{13}
		b.CreatureExperience = map[uint32]uint32{3: 44}
		role.State, _ = SaveBag(role.State, b)
		for _, tc := range []struct {
			src, dst uint16
			want     uint32
		}{{1, 0, 113370008}, {2, 0, 113370026}, {0, 3, 0}, {3, 4, 0}} {
			r := protocol.ItemMoveRequest{SourceList: 31, DestinationList: 31, SourceSlot: tc.src, DestinationSlot: tc.dst, Selection: 0xffffffff}
			raw, e := s.MoveOrdinary(role, r)
			if e != nil {
				t.Fatal(e)
			}
			role.State = raw
			after, e := ReadBag(raw)
			if e != nil {
				t.Fatal(e)
			}
			if after.KnightDeck()[0] != tc.want || after.KnightShieldDeck[0] != tc.want {
				t.Fatalf("move %d->%d deck=%v worn=%v", tc.src, tc.dst, after.KnightShieldDeck, after.Worn)
			}
			if after.Gold != 123 || after.ExpandEquipFlags != 3 || after.CreatureExperience[3] != 44 || !reflect.DeepEqual(after.WeaponSkins, []uint32{13}) {
				t.Fatal("shield move damaged unrelated inventory")
			}
		}
	})
	t.Run("shelf equip and unequip", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{})
		r := protocol.ItemMoveRequest{SourceList: 32, SourceItem: 113370008, DestinationList: 31, DestinationSlot: 0, Selection: 0xffffffff}
		raw, e := s.MoveOrdinary(role, r)
		if e != nil {
			t.Fatal(e)
		}
		role.State = raw
		b, _ := ReadBag(raw)
		if b.KnightDeck()[0] != 113370008 {
			t.Fatal("shelf equip did not mirror slot 24")
		}
		r = protocol.ItemMoveRequest{SourceList: 31, SourceSlot: 0, DestinationList: 32, Selection: 0xffffffff}
		raw, e = s.MoveOrdinary(role, r)
		if e != nil {
			t.Fatal(e)
		}
		b, _ = ReadBag(raw)
		if b.KnightDeck()[0] != 0 {
			t.Fatal("shelf unequip did not clear slot 24")
		}
	})
	t.Run("upload mirrors and repeat is stable", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{113370003, 113370008})
		deck := [5]uint32{113370026, 113370003, 113370008}
		raw, result, e := s.ApplyKnightDeck(role, deck)
		if e != nil || !result.WornChanged || result.Equipped != deck[0] {
			t.Fatal(result, e)
		}
		role.State = raw
		again, result, e := s.ApplyKnightDeck(role, deck)
		if e != nil || result.WornChanged {
			t.Fatal(result, e)
		}
		if string(raw) != string(again) {
			t.Fatal("duplicate upload changed state")
		}
		deck[0] = 0
		raw, result, e = s.ApplyKnightDeck(role, deck)
		if e != nil || !result.WornChanged {
			t.Fatal(result, e)
		}
		b, _ := ReadBag(raw)
		if b.KnightDeck()[0] != 0 {
			t.Fatal("zero socket upload did not unequip")
		}
	})
	t.Run("stored zero comes from wear table", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{113370003, 113370008})
		b, _ := ReadBag(role.State)
		b.KnightShieldDeck[0] = 113370026
		// Simulate an old inconsistent save without the canonical writer.
		var fields map[string]json.RawMessage
		json.Unmarshal(role.State, &fields)
		fields["inventory"], _ = json.Marshal(b)
		role.State, _ = json.Marshal(fields)
		p, e := s.KnightDeckPayload(role)
		if e != nil {
			t.Fatal(e)
		}
		got, e := protocol.DecodeKnightDeck(p)
		if e != nil || got[0] != 113370003 {
			t.Fatal(got, e)
		}
	})
	t.Run("invalid moves are silent and leave state unchanged", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{113370003, 113370008})
		for _, r := range []protocol.ItemMoveRequest{
			{SourceList: 31, DestinationList: 31, SourceSlot: 5, DestinationSlot: 0, Selection: 0xffffffff},
			{SourceList: 31, DestinationList: 31, SourceSlot: 3, DestinationSlot: 0, Selection: 0xffffffff},
			{SourceList: 31, DestinationList: 31, SourceSlot: 1, DestinationSlot: 0, Selection: 0},
			{SourceList: 32, SourceItem: 113370008, DestinationList: 31, DestinationSlot: 1, Selection: 0xffffffff},
			{SourceList: 32, SourceItem: 113370007, DestinationList: 31, DestinationSlot: 0, Selection: 0xffffffff},
			{SourceList: 31, SourceSlot: 0, DestinationList: 0, Selection: 0xffffffff},
		} {
			raw, e := s.MoveOrdinary(role, r)
			if e == nil || raw != nil || MoveRefusalCode(e) != 5 {
				t.Fatal(r, string(raw), e)
			}
		}
		disabled := *s
		disabled.Shields = nil
		if _, e := disabled.MoveOrdinary(role, protocol.ItemMoveRequest{SourceList: 32, DestinationList: 31, Selection: 0xffffffff}); e == nil || MoveRefusalCode(e) != 5 {
			t.Fatal("absent window must refuse silently", e)
		}
		role.Profession = 0
		if _, _, e := s.ApplyKnightDeck(role, [5]uint32{113370008}); e == nil {
			t.Fatal("non-knight accepted")
		}
	})
	t.Run("every level shield passes real gate at 90 and quest shields stay refused", func(t *testing.T) {
		levels, quests := 0, 0
		for _, row := range s.Shields.Rows {
			role := knightRole(t, s, [5]uint32{})
			_, _, e := s.ApplyKnightDeck(role, [5]uint32{row.Item})
			if row.Condition == "quest" {
				quests++
				if e == nil {
					t.Fatalf("quest shield %d accepted", row.Item)
				}
			} else {
				levels++
				if e != nil {
					t.Fatalf("level shield %d refused: %v", row.Item, e)
				}
			}
		}
		if levels != 19 || quests != 6 {
			t.Fatalf("level=%d quest=%d", levels, quests)
		}
		role := knightRole(t, s, [5]uint32{})
		role.State = json.RawMessage(`{"level":24,"advancement":1}`)
		if _, _, e := s.ApplyKnightDeck(role, [5]uint32{113370008}); e == nil {
			t.Fatal("level 25 gate ignored")
		}
	})
	t.Run("legacy empty entry sends no map", func(t *testing.T) {
		role := knightRole(t, s, [5]uint32{})
		role.State = json.RawMessage(`{"level":90,"advancement":1}`)
		p, e := s.KnightDeckPayload(role)
		if e != nil || len(p) != 0 {
			t.Fatal("empty legacy entry asserted window state", e)
		}
	})
}
