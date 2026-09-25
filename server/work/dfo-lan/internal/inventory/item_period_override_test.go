package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"testing"
)

func TestConfiguredMaximumPeriodCoversEquipmentAndSpecialRows(t *testing.T) {
	const gear = uint32(100051399)
	const avatar = uint32(501552792)
	const creature = uint32(100991331)
	protocol.ConfigureMaxItemPeriods([]uint32{gear, avatar, creature})
	t.Cleanup(func() { protocol.ConfigureMaxItemPeriods(nil) })

	record := protocol.OrdinaryItem(14, gear, 0, 123)
	binary.LittleEndian.PutUint32(record[56:], 123)
	row := EquipmentRow(BagEquipment{Slot: 14, Template: gear, Record: record[:]})
	if got := binary.LittleEndian.Uint32(row[56:60]); got != protocol.MaxItemPeriod {
		t.Fatalf("equipment row period = %d", got)
	}

	av, err := EquipmentPayload(1, []BagEquipment{{Slot: 0, Template: avatar}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(av[len(av)-4:]); got != protocol.MaxItemPeriod {
		t.Fatalf("avatar trailing period = %d", got)
	}

	cr, err := EquipmentPayload(7, []BagEquipment{{Slot: 0, Template: creature}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(cr[3+56 : 3+60]); got != protocol.MaxItemPeriod {
		t.Fatalf("creature row period = %d", got)
	}
}
