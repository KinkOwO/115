package protocol

import (
	"encoding/binary"
	"testing"
)

func TestRosterFatigueNativeTail(t *testing.T) {
	row := CharacterRow{Name: "LanTest01", Level: 1, FatigueRemaining: 156, FatigueBonus: 27}
	p, e := CharacterList(8, []CharacterRow{row})
	if e != nil {
		t.Fatal(e)
	}
	// Native row ends with two signed u16 display values and an unrelated u32;
	// the enclosing list then has its independently recovered ten-byte tail.
	tail := p[len(p)-18:]
	if binary.LittleEndian.Uint16(tail) != 156 || binary.LittleEndian.Uint16(tail[2:]) != 27 {
		t.Fatal("fatigue fields shifted", tail)
	}
	row.FatigueRemaining = 32768
	if _, e = CharacterList(8, []CharacterRow{row}); e == nil {
		t.Fatal("native negative display accepted")
	}
}
