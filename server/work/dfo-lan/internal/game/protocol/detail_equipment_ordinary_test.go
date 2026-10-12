package protocol

import (
	"encoding/binary"
	"testing"
)

func TestDetailedEquipmentOrdinaryNativeRows(t *testing.T) {
	rows := []DetailedWorn{{Slot: 12, Template: 101000013, Durability: 44, Period: 123}, {Slot: 25, Template: 100610096, Durability: 9}}
	p, err := DetailedEquipment(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1+2*127+13 || p[0] != 2 {
		t.Fatalf("ordinary block length/count=%d/%d", len(p), p[0])
	}
	for i, want := range rows {
		row := p[1+i*127 : 1+(i+1)*127]
		if row[0] != byte(want.Slot) || binary.LittleEndian.Uint32(row[1:]) != want.Template || binary.LittleEndian.Uint16(row[10:]) != want.Durability || row[40] != 0 || binary.LittleEndian.Uint32(row[41:]) != want.Period || row[45] != 0 {
			t.Fatalf("ordinary row differs from native 1452C1540 layout: %x", row)
		}
	}
}
