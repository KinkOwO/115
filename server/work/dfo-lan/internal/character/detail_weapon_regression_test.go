package character

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// playguide_5.co checks the actor's item object at slot 12, rather than the
// mode-0 appearance index. Mode 1 must list owned ordinary worn objects:
// sub_1452C1540 deletes omitted slots before the Seria-room guide can check.
func TestEntryAdditionRestoresOrdinaryWornObjects(t *testing.T) {
	service := Service{DetailedWornCandidate: true}
	for _, tc := range []struct {
		name      string
		inventory string
		want      []struct {
			slot       byte
			template   uint32
			durability uint16
		}
	}{
		{"weapon-and-armour", `"worn":[{"slot":12,"template":101000013,"durability":44},{"slot":14,"template":10005,"durability":27},{"slot":25,"template":100610096,"durability":9}]`, []struct {
			slot       byte
			template   uint32
			durability uint16
		}{{12, 101000013, 44}, {14, 10005, 27}, {25, 100610096, 9}}},
		{"weapon-only-in-bag", `"equipment":[{"slot":12,"template":101000013,"durability":44}],"worn":[]`, nil},
		{"armour-without-weapon", `"worn":[{"slot":14,"template":10005,"durability":27}]`, []struct {
			slot       byte
			template   uint32
			durability uint16
		}{{14, 10005, 27}}},
		{"empty", `"worn":[]`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{` + tc.inventory + `}}`)
			before := append([]byte(nil), state...)
			packet, err := service.EntryAddition(Character{WireID: 503, State: state})
			if err != nil {
				t.Fatal(err)
			}
			if got := int(packet[361]); got != len(tc.want) {
				t.Fatalf("mode-1 worn count=%d, want %d", got, len(tc.want))
			}
			// Native ordinary rows: 40-byte header + 87-byte tail, without
			// avatar length cells or the type-26 creature extension.
			for i, want := range tc.want {
				row := packet[362+i*127 : 362+(i+1)*127]
				if row[0] != want.slot || binary.LittleEndian.Uint32(row[1:]) != want.template || binary.LittleEndian.Uint16(row[10:]) != want.durability {
					t.Fatalf("ordinary row %d did not restore the owned slot: %x", i, row[:12])
				}
			}
			if !bytes.Equal(state, before) {
				t.Fatal("entry projection changed stored equipment")
			}
		})
	}
}
