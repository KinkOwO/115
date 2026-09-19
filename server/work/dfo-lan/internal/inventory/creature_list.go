package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
)

type CreatureEntry struct {
	Key     uint32
	Satiety byte   // default 100
	Mode    byte   // default 0
	Exp     uint32 // default 0
	Level   byte   // default 1
	Name    string // custom name if any, empty loads default PVF name
	Tail    byte   // default 1
}

var CreatureDefaultNames = map[uint32]string{
	63000: "Faras",
	63003: "Charp",
	63008: "Botis",
	63009: "Marbas",
	63010: "Balam",
	63011: "Haagenti",
	63012: "Berith",
	63019: "Zagan",
	63023: "Bobo",
	63025: "Amy",
	63027: "Elbon",
	63032: "Belz",
}

// CreatureListPayload constructs NOTI 105 (0x0069, ENUM_NOTIPACKET_CREATURE_ITEM_LIST).
// Native reader sub_1452CA7E0 consumes:
// u8 count; repeat { u32 key, u8 satiety, u8 mode, u32 exp, [mode==1: u8, u8], u8 level, dstr name, u8 tail }.
func CreatureListPayload(state json.RawMessage) ([]byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return []byte{0}, err
	}
	var entries []CreatureEntry
	seenKeys := map[uint32]bool{}

	// 1. Equipped creature at worn slot 26
	for _, it := range b.Worn {
		if it.Slot == 26 && it.Template != 0 {
			key := uint32(1)
			if len(it.Record) == protocol.CurrentItemRecordSize {
				if k := binary.LittleEndian.Uint32(it.Record[6:10]); k != 0 {
					key = k
				}
			}
			seenKeys[key] = true
			name := CreatureDefaultNames[it.Template]
			entries = append(entries, CreatureEntry{
				Key:     key,
				Satiety: 100,
				Mode:    0,
				Exp:     0,
				Level:   1,
				Name:    name,
				Tail:    0,
			})
			break
		}
	}

	// 2. Hatched creatures in space 7 (slots 0..139)
	if b.Special != nil {
		for _, it := range b.Special[7] {
			if it.Template != 0 && it.Slot < 140 {
				if _, isEgg := EggHatchOutputs[it.Template]; isEgg {
					continue // unhatched eggs do not enter creature list
				}
				key := uint32(it.Slot + 2)
				if len(it.Record) == protocol.CurrentItemRecordSize {
					if k := binary.LittleEndian.Uint32(it.Record[6:10]); k != 0 {
						key = k
					}
				}
				if seenKeys[key] {
					key = uint32(len(seenKeys) + 10)
				}
				seenKeys[key] = true
				name := CreatureDefaultNames[it.Template]
				entries = append(entries, CreatureEntry{
					Key:     key,
					Satiety: 100,
					Mode:    0,
					Exp:     0,
					Level:   1,
					Name:    name,
					Tail:    0,
				})
			}
		}
	}

	p := []byte{byte(len(entries))}
	for _, e := range entries {
		p = binary.LittleEndian.AppendUint32(p, e.Key)
		p = append(p, e.Satiety)
		p = append(p, e.Mode)
		p = binary.LittleEndian.AppendUint32(p, e.Exp)
		if e.Mode == 1 {
			p = append(p, 0, 0)
		}
		p = append(p, e.Level)
		p = binary.LittleEndian.AppendUint32(p, uint32(len(e.Name)))
		p = append(p, []byte(e.Name)...)
		p = append(p, e.Tail)
	}
	return p, nil
}

func HasEquippedCreature(state json.RawMessage) bool {
	b, err := ReadBag(state)
	if err != nil {
		return false
	}
	for _, it := range b.Worn {
		if it.Slot == 26 && it.Template != 0 {
			return true
		}
	}
	return false
}
