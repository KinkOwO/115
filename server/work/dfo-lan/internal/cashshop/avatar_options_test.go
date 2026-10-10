package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func selectableAvatar(t *testing.T, ability []pvf.Token) *Pilot {
	t.Helper()
	row := make([]pvf.Token, 14)
	row[0].Value, row[1].Value, row[2].Value, row[5].Value = 3107733, 515540507, 4, 120
	for _, i := range []int{10, 11, 12} {
		row[i].Value = -1
	}
	row[13].Type = 6
	c := syntheticFamily(t, "[dont trade avatar]", row,
		map[uint32]string{515540507: "equipment/character/priest/at_avatar/shoes/515540507.equ"},
		func(name string) (catalog.ScriptRecord, error) {
			return catalog.ScriptRecord{SHA256: strings.Repeat("a", 64), Path: name, Cells: ability}, nil
		})
	return &Pilot{Config: c}
}

func TestAvatarPurchaseSelectedOptions(t *testing.T) {
	// Sparse source keys: accepting a range based on the number of rows is wrong.
	p := selectableAvatar(t, []pvf.Token{{Type: 3, Text: "[avatar select ability]"},
		{Value: 2}, {Type: 6, Text: "[ATTACK_SPEED]"}, {Type: 6, Text: "+"}, {Value: 50},
		{Value: 8}, {Type: 6, Text: "[SKILL_LEVEL]"}, {Type: 6, Text: "[priest]"}, {Value: 100}, {Value: 1}})
	p.avatarSockets = func(uint32) []byte { return []byte{4, 0, 0, 0, 0, 0} }
	l := &packLedger{state: json.RawMessage(`{"level":55}`)}
	cart := []protocol.CeraCartItem{{Product: 3107733, Quantity: 1, Kind: 4, Option: 2}, {Product: 3107733, Quantity: 1, Kind: 4, Option: 8}}
	_, applied, err := p.Purchase(context.Background(), l, 1, 1, "avatar-options-0001", cart)
	if err != nil || !applied {
		t.Fatal("same SKU, different selected attributes", err)
	}
	b, err := inventory.ReadBag(l.state)
	if err != nil || len(b.Special[1]) != 2 {
		t.Fatal("wardrobe", b, err)
	}
	for i, option := range []uint16{2, 8} {
		r := b.Special[1][i]
		if r.Durability != option || r.Template != 515540507 || len(r.AvatarOptions) != 6 || r.AvatarOptions[0] != 4 || l.order.Lines[i].AvatarOption != byte(option) {
			t.Fatal("attribute/socket/order persistence", r, l.order)
		}
	}
	for _, cart := range [][]protocol.CeraCartItem{
		{{Product: 3107733, Quantity: 1, Kind: 4, Option: 3}},
		{{Product: 3107733, Quantity: 1, Kind: 4, Option: 255}},
		{{Product: 3107733, Quantity: 1, Kind: 3, Option: 2}},
		{{Product: 3107733, Quantity: 1, Kind: 4, Option: 2}, {Product: 3107733, Quantity: 1, Kind: 4, Option: 3}},
	} {
		before := string(l.state)
		if _, applied, err := p.Purchase(context.Background(), l, 1, 1, "avatar-invalid-0001", cart); err == nil || applied || string(l.state) != before {
			t.Fatal("invalid selection mutated inventory", cart, err)
		}
	}
	// Preserve zero-option legacy purchases, without changing source sockets.
	if _, _, err := p.Purchase(context.Background(), l, 1, 1, "avatar-default-0001", []protocol.CeraCartItem{{Product: 3107733, Quantity: 1, Kind: 4}}); err != nil {
		t.Fatal(err)
	}
	b, _ = inventory.ReadBag(l.state)
	if b.Special[1][2].Durability != 0 {
		t.Fatal("legacy default changed")
	}
	if _, err := p.deliverSelectedAmount(json.RawMessage(`{}`), 1, 1, 2); err == nil {
		t.Fatal("selected option applied to currency")
	}
}

func TestAvatarPurchaseAbilityCase(t *testing.T) {
	p := selectableAvatar(t, []pvf.Token{{Type: 3, Text: "[ability case index]"}, {Value: 7}})
	err := p.Config.loadAvatarAbilityCases(func(name string) (catalog.ScriptRecord, error) {
		return catalog.ScriptRecord{Path: name, SHA256: strings.Repeat("b", 64), Cells: []pvf.Token{
			{Type: 3, Text: "[ability case]"}, {Value: 7},
			{Value: 9}, {Type: 6, Text: "[SKILL_LEVEL]"}, {Type: 6, Text: "[priest]"}, {Value: 100}, {Value: 1},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	l := &packLedger{state: json.RawMessage(`{}`)}
	if _, _, err := p.Purchase(context.Background(), l, 1, 1, "avatar-case-00001", []protocol.CeraCartItem{{Product: 3107733, Quantity: 1, Kind: 4, Option: 9}}); err != nil {
		t.Fatal(err)
	}
	b, _ := inventory.ReadBag(l.state)
	if b.Special[1][0].Durability != 9 {
		t.Fatal("case option not saved", b)
	}
}

func TestAvatarPurchaseNativeSelectedOptions(t *testing.T) {
	p := nativePilot(t, true)
	products, err := p.products()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		product      uint32
		kind, option byte
	}{
		{3101306, 4, 2}, {3101309, 4, 2}, {3101420, 3, 1}, {3101421, 3, 1}, {3101425, 3, 1},
		{20008418, 4, 4}, {20008420, 4, 2}, {20003100, 4, 8}, {20009272, 4, 4},
	} {
		t.Run(fmt.Sprint(tc.product), func(t *testing.T) {
			// MOD capture SKUs are optional on an unmodified native PVF.
			if _, present := products[tc.product]; !present && tc.product >= 20000000 {
				t.Skip("captured MOD fixture is not installed in this PVF")
			}
			l := &packLedger{state: json.RawMessage(`{}`)}
			if _, applied, err := p.Purchase(context.Background(), l, 1, 1, "native-avatar-selected-0001", []protocol.CeraCartItem{{Product: tc.product, Quantity: 1, Kind: tc.kind, Option: tc.option}}); err != nil || !applied {
				t.Fatal(err)
			}
			b, err := inventory.ReadBag(l.state)
			if err != nil || len(b.Special[1]) != 1 || b.Special[1][0].Durability != uint16(tc.option) {
				t.Fatal("selected attribute not saved", b, err)
			}
		})
	}
}
