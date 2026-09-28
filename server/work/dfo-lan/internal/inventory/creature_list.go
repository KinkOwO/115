package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"sort"
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

// CreatureSkinKey 给出幻化槽（穿戴槽 32）那一行携带的生物实例 key：行里已记的
// 实例值优先，没有就用兜底常量。NOTI105 的生物条目、list 3 行的 offset 6/24、
// 以及装备外观块的 Model 必须用同一个值，客户端才能把这三处对上（外观块那格是
// 幻化槽唯一的物品来源，见 protocol.Equipment.Model）。
func CreatureSkinKey(it BagEquipment) uint32 {
	return creatureKey(it, CreatureSkinFallbackKey)
}

func equippedCreature(b Bag) (uint32, bool) {
	for _, it := range b.Worn {
		if it.Slot == 26 && it.Template != 0 {
			return creatureKey(it, 1), true
		}
	}
	return 0, false
}

func creatureSatiety(b Bag, key uint32) byte {
	if value, ok := b.CreatureSatiety[key]; ok {
		return value
	}
	return 100
}

// creatureEntry 按生物实例 key 组装 NOTI105 里的一条生物条目。
func creatureEntry(b Bag, key, template uint32) CreatureEntry {
	exp := b.CreatureExperience[key]
	return CreatureEntry{
		Key:     key,
		Satiety: creatureSatiety(b, key),
		Mode:    0,
		Exp:     exp,
		Level:   creatureLevel(exp),
		Name:    CreatureDefaultNames[template],
		Tail:    0,
	}
}

// AdvanceCreatureLoyalty applies the official hourly rates to elapsed time.
// The previous dungeon key owns the interval being settled; the new key owns
// the next interval. A missing timestamp in an older save starts at now.
func AdvanceCreatureLoyalty(state json.RawMessage, now int64, inDungeon bool, catalog catalog.LootCatalog) (json.RawMessage, bool, bool, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, false, false, err
	}
	currentKey, equipped := equippedCreature(b)
	nextDungeonKey := uint32(0)
	if inDungeon && equipped {
		nextDungeonKey = currentKey
	}
	if b.CreatureLoyaltyUpdatedAt == 0 {
		b.CreatureLoyaltyUpdatedAt = now
		b.CreatureLoyaltyDungeonKey = nextDungeonKey
		state, err = SaveBag(state, b)
		return state, false, false, err
	}
	if now < b.CreatureLoyaltyUpdatedAt {
		now = b.CreatureLoyaltyUpdatedAt
	}
	seconds := now - b.CreatureLoyaltyUpdatedAt
	changed, fed := false, false
	keys := map[uint32]bool{}
	for key := range b.CreatureExperience {
		keys[key] = true
	}
	for key := range b.CreatureSatiety {
		keys[key] = true
	}
	if equipped {
		keys[currentKey] = true
	}
	for _, item := range b.Special[7] {
		if item.Slot < 140 && item.Template != 0 {
			if _, egg := EggHatchOutputs[item.Template]; !egg {
				keys[creatureKey(item, uint32(item.Slot+2))] = true
			}
		}
	}
	if b.CreatureSatiety == nil {
		b.CreatureSatiety = make(map[uint32]byte)
	}
	if b.CreatureLoyaltyFraction == nil {
		b.CreatureLoyaltyFraction = make(map[uint32]int64)
	}
	orderedKeys := make([]uint32, 0, len(keys))
	for key := range keys {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Slice(orderedKeys, func(i, j int) bool {
		if orderedKeys[i] == currentKey && orderedKeys[j] != currentKey {
			return true
		}
		if orderedKeys[j] == currentKey && orderedKeys[i] != currentKey {
			return false
		}
		return orderedKeys[i] < orderedKeys[j]
	})
	for _, key := range orderedKeys {
		old := creatureSatiety(b, key)
		rate := int64(1)
		if key == b.CreatureLoyaltyDungeonKey {
			rate = -6
		}
		fraction := b.CreatureLoyaltyFraction[key]
		points := int64(old)
		remaining := seconds
		feedIndex := func() int {
			for i, row := range b.PetItems {
				definition, ok := catalog.Items[row.Template]
				if ok && IsPetFeed(definition.StackableType) && row.Amount > 0 {
					return i
				}
			}
			return -1
		}
		consumeFeed := func(index int) {
			b.PetItems = append([]BagItem(nil), b.PetItems...)
			b.PetItems[index].Amount--
			if b.PetItems[index].Amount == 0 {
				b.PetItems = append(b.PetItems[:index], b.PetItems[index+1:]...)
			}
			points, fraction, fed = 100, 0, true
		}
		for {
			index := feedIndex()
			if points <= 10 && index >= 0 {
				consumeFeed(index)
				continue
			}
			if rate < 0 && points > 10 && index >= 0 {
				need := (points-10)*360 + fraction
				toThreshold := (need + 5) / 6
				if toThreshold <= remaining {
					fraction -= 6 * toThreshold
					points += fraction / 360
					fraction %= 360
					remaining -= toThreshold
					consumeFeed(index)
					continue
				}
			}
			fraction += rate * remaining
			points += fraction / 360
			fraction %= 360
			break
		}
		if points < 0 {
			points, fraction = 0, 0
		}
		if points > 100 || (points == 100 && rate > 0) {
			points, fraction = 100, 0
		}
		if byte(points) != old {
			changed = true
		}
		b.CreatureSatiety[key] = byte(points)
		b.CreatureLoyaltyFraction[key] = fraction
	}
	b.CreatureLoyaltyUpdatedAt = now
	b.CreatureLoyaltyDungeonKey = nextDungeonKey
	state, err = SaveBag(state, b)
	return state, changed, fed, err
}

// BeginCreatureLoyaltySession does not charge offline time, whose location is
// unknown after a disconnect. It preserves the saved loyalty and fraction.
func BeginCreatureLoyaltySession(state json.RawMessage, now int64) (json.RawMessage, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, err
	}
	b.CreatureLoyaltyUpdatedAt = now
	b.CreatureLoyaltyDungeonKey = 0
	return SaveBag(state, b)
}

func FeedEquippedCreatureBag(b Bag) (Bag, bool) {
	key, ok := equippedCreature(b)
	if !ok || creatureSatiety(b, key) == 100 {
		return b, false
	}
	if b.CreatureSatiety == nil {
		b.CreatureSatiety = make(map[uint32]byte)
	}
	b.CreatureSatiety[key] = 100
	if b.CreatureLoyaltyFraction != nil {
		b.CreatureLoyaltyFraction[key] = 0
	}
	return b, true
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
	if creatureSatiety(b, key) == 0 {
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
			entries = append(entries, creatureEntry(b, key, it.Template))
			break
		}
	}

	// 2. 幻化槽（穿戴槽 32）里的宠物必须也出现在这份列表里。
	//
	// 实机 2026-09-26：小退重登后 F6 的 Skin 框空白、但外观仍是幻化槽宠物。
	// 客户端画那个框走的是"按 key 解析到的生物对象"，不是 list 3 那一行 ——
	// 刚拖入时之所以能画出来，只是因为客户端本地还留着该生物（它上一轮还在
	// 宠物栏）；重登后客户端从零构建生物集合，而本函数原先只收录穿戴槽 26 与
	// 宠物栏，幻化槽宠物的 key 无处解析，框就空了。外观不受影响是因为它走
	// mode-0 生物段（internal/character 的 wornCreature 直接读存档）。
	//
	// key 用行里已有的实例 key（与 equipmentRows 对槽 32 的归一同一份来源），
	// 撞到本体那只时不再收录：客户端会解析到本体条目，总比两份重复条目好。
	for _, it := range b.Worn {
		if it.Slot == CreatureSkinSlot && it.Template != 0 {
			key := CreatureSkinKey(it)
			if !seenKeys[key] {
				seenKeys[key] = true
				entries = append(entries, creatureEntry(b, key, it.Template))
			}
			break
		}
	}

	// 3. Hatched creatures in space 7 (slots 0..139)
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
				entries = append(entries, creatureEntry(b, key, it.Template))
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
