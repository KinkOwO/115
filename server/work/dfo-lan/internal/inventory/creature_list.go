package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
)

type CreatureEntry struct {
	Key     uint32
	Satiety byte // default 100
	Mode    byte // default 0
	Exp     uint32
	Level   byte
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

// Current 115 client Script.inner.pvf creature/exptable.tbl (55 entries).
// The native growth reader sub_145E349E0 uses table[level-2] as the
// cumulative threshold for the current level. The final value is the cap.
var creatureExperienceThresholds = [...]uint32{
	2, 5, 9, 15, 23, 33, 44, 57, 71, 88, 106, 126, 148, 171,
	196, 223, 252, 283, 315, 350, 388, 429, 472, 517, 565, 615,
	668, 725, 785, 849, 918, 993, 1073, 1158, 1245, 1335, 1430,
	1532, 1642, 1759, 1879, 2011, 2151, 2301, 2464, 2639,
	2829, 3049, 3294, 3564, 3879, 4229, 4629, 5079, 5579,
}

func creatureLevel(exp uint32) byte {
	level := byte(1)
	for i := 0; i < len(creatureExperienceThresholds)-1 && exp >= creatureExperienceThresholds[i]; i++ {
		level++
	}
	return level
}

func creatureKey(it BagEquipment, fallback uint32) uint32 {
	if len(it.Record) == protocol.CurrentItemRecordSize {
		if key := binary.LittleEndian.Uint32(it.Record[6:10]); key != 0 {
			return key
		}
	}
	return fallback
}

func equippedCreature(b Bag) (uint32, bool) {
	for _, it := range b.Worn {
		if it.Slot == 26 && it.Template != 0 {
			return creatureKey(it, 1), true
		}
	}
	return 0, false
}

// AwardEquippedCreatureExperience preserves all other character state and
// only credits the creature worn when the clear is committed.
func AwardEquippedCreatureExperience(state json.RawMessage, gain uint32) (json.RawMessage, uint32, error) {
	if gain == 0 {
		return state, 0, nil
	}
	b, err := ReadBag(state)
	if err != nil {
		return nil, 0, err
	}
	key, ok := equippedCreature(b)
	if !ok {
		return state, 0, nil
	}
	if b.CreatureExperience == nil {
		b.CreatureExperience = make(map[uint32]uint32)
	}
	before := b.CreatureExperience[key]
	maximum := creatureExperienceThresholds[len(creatureExperienceThresholds)-1]
	if before >= maximum {
		return state, 0, nil
	}
	after := before + gain
	if after < before || after > maximum {
		after = maximum
	}
	b.CreatureExperience[key] = after
	state, err = SaveBag(state, b)
	return state, after - before, err
}

// CreatureGrowthPayload follows the native NOTI 102 reader sub_1452D0DA0:
// u8 level, u8 mode, u32 cumulative experience (two extra bytes for mode 1).
func CreatureGrowthPayload(state json.RawMessage) ([]byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, err
	}
	key, ok := equippedCreature(b)
	if !ok {
		return nil, nil
	}
	exp := b.CreatureExperience[key]
	p := []byte{creatureLevel(exp), 0}
	p = binary.LittleEndian.AppendUint32(p, exp)
	return p, nil
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
			key := creatureKey(it, 1)
			seenKeys[key] = true
			name := CreatureDefaultNames[it.Template]
			entries = append(entries, CreatureEntry{
				Key:     key,
				Satiety: 100,
				Mode:    0,
				Exp:     b.CreatureExperience[key],
				Level:   creatureLevel(b.CreatureExperience[key]),
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
				key := creatureKey(it, uint32(it.Slot+2))
				if seenKeys[key] {
					key = uint32(len(seenKeys) + 10)
				}
				seenKeys[key] = true
				name := CreatureDefaultNames[it.Template]
				entries = append(entries, CreatureEntry{
					Key:     key,
					Satiety: 100,
					Mode:    0,
					Exp:     b.CreatureExperience[key],
					Level:   creatureLevel(b.CreatureExperience[key]),
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
