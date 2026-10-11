package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	BakalSlotsSource    = "contents/2022/bakalraid/etc/clientbakalslotscript.cos"
	BakalMonstersSource = "contents/2022/bakalraid/etc/bakalmonster.cos"
	BakalSymbolsSource  = "contents/2022/bakalraid/etc/bakal.etc.symbol"
)

type BakalRaidSlot struct {
	Index   uint32
	Type    string
	Dungeon uint32
	Maps    []uint32
}
type BakalRaidMonster struct {
	BuffCount                            int
	BuffCheck                            string
	BuffPool, AdditionalBuffPool         []BakalWeightedBuff
	HardBuffCount                        int
	HardBuffPool, HardAdditionalBuffPool []BakalWeightedBuff
	Type                                 string
	Template, SecondTemplate             uint32
	SecondGrid                           [2]byte
	Grid                                 [2]byte
	HasGrid                              bool
	Position                             [2]uint32
	SpecificMap                          int
}

// Text COS and binary ETC use the same section/value model. Text COS values
// are delimited by backticks; malformed tokens fail instead of being ignored.
func readBakalCOSTokens(a *pvf.Archive, path string) ([]pvf.Token, error) {
	f, ok := a.FindFile(path)
	if !ok {
		return nil, fmt.Errorf("missing native Bakal table %s", path)
	}
	if f.DataType == 1 {
		return a.Tokens(path)
	}
	text, err := a.FileText(f.Index)
	if err != nil {
		return nil, err
	}
	return bakalTextTokens(text)
}
func bakalTextTokens(text string) ([]pvf.Token, error) {
	var out []pvf.Token
	text = strings.Trim(text, "\x00\ufeff \r\n\t")
	for len(text) > 0 {
		text = strings.TrimLeftFunc(text, unicode.IsSpace)
		if text == "" {
			break
		}
		if strings.HasPrefix(text, "//") {
			if i := strings.IndexByte(text, '\n'); i >= 0 {
				text = text[i+1:]
				continue
			}
			break
		}
		switch text[0] {
		case '[':
			i := strings.IndexByte(text, ']')
			if i < 0 {
				return nil, fmt.Errorf("unclosed COS label")
			}
			out = append(out, pvf.Token{Type: 3, Text: text[:i+1]})
			text = text[i+1:]
		case '`', '"':
			i := strings.IndexByte(text[1:], text[0])
			if i < 0 {
				return nil, fmt.Errorf("unclosed COS string")
			}
			out = append(out, pvf.Token{Type: 6, Text: text[1 : i+1]})
			text = text[i+2:]
		default:
			i := strings.IndexFunc(text, unicode.IsSpace)
			if i < 0 {
				i = len(text)
			}
			word := text[:i]
			text = text[i:]
			if n, e := strconv.ParseInt(word, 10, 32); e == nil {
				out = append(out, pvf.Token{Type: 0, Value: int32(n)})
			} else if n, e := strconv.ParseFloat(word, 32); e == nil {
				out = append(out, pvf.Token{Type: 2, Number: float32(n)})
			} else {
				return nil, fmt.Errorf("invalid COS value %q", word)
			}
		}
	}
	return out, nil
}

func loadBakalRaidTables(a *pvf.Archive, r *BakalRaidRules) error {
	cells, err := readBakalCOSTokens(a, BakalSlotsSource)
	if err != nil {
		return err
	}
	blocks, err := bakalBlocks(cells, "[location]", "[/location]")
	if err != nil {
		return err
	}
	r.Slots = make(map[uint32]BakalRaidSlot, len(blocks))
	for _, b := range blocks {
		index, e := bakalRowInt(b, "[slot]")
		if e != nil || index <= 0 {
			return fmt.Errorf("invalid native Bakal slot: %v", e)
		}
		names := bakalRowStrings(b, "[type]")
		if len(names) != 1 {
			return fmt.Errorf("Bakal slot %d has no type", index)
		}
		slot := BakalRaidSlot{Index: uint32(index), Type: names[0]}
		if row, ok := bakalRowByLabel(b, "[dungeon]"); ok {
			v := bakalInts(row.Args)
			if len(v) != 1 || v[0] <= 0 {
				return fmt.Errorf("Bakal slot %d invalid dungeon", index)
			}
			slot.Dungeon = uint32(v[0])
		}
		if row, ok := bakalRowByLabel(b, "[map]"); ok {
			for _, v := range bakalInts(row.Args) {
				if v <= 0 {
					return fmt.Errorf("Bakal slot %d invalid map", index)
				}
				slot.Maps = append(slot.Maps, uint32(v))
			}
		}
		if slot.Type != "camp" && (slot.Dungeon == 0 || len(slot.Maps) == 0) {
			return fmt.Errorf("Bakal slot %d incomplete", index)
		}
		if _, dup := r.Slots[slot.Index]; dup {
			return fmt.Errorf("duplicate Bakal slot %d", index)
		}
		r.Slots[slot.Index] = slot
	}
	cells, err = readBakalCOSTokens(a, BakalMonstersSource)
	if err != nil {
		return err
	}
	blocks, err = bakalBlocks(cells, "[monster info]", "[/monster info]")
	if err != nil {
		return err
	}
	r.Monsters = make(map[string]BakalRaidMonster, len(blocks))
	for _, b := range blocks {
		names := bakalRowStrings(b, "[monster type]")
		if len(names) != 1 {
			return fmt.Errorf("native Bakal monster has no type")
		}
		template, e := bakalRowInt(b, "[monster index]")
		if e != nil || template <= 0 {
			return fmt.Errorf("Bakal monster %s invalid index", names[0])
		}
		pos, e := bakalRowInts(b, "[position]")
		if e != nil || len(pos) != 2 || pos[0] < 0 || pos[1] < 0 {
			return fmt.Errorf("Bakal monster %s invalid position", names[0])
		}
		specific, e := bakalRowInt(b, "[specific map]")
		if e != nil {
			return e
		}
		m := BakalRaidMonster{Type: names[0], Template: uint32(template), Position: [2]uint32{uint32(pos[0]), uint32(pos[1])}, SpecificMap: specific}
		if err := loadBakalMonsterBuffs(b, &m); err != nil {
			return err
		}
		if row, ok := bakalRowByLabel(b, "[appear grid]"); ok {
			v := bakalInts(row.Args)
			if len(v) != 2 || v[0] < 0 || v[0] > 255 || v[1] < 0 || v[1] > 255 {
				return fmt.Errorf("invalid native Bakal appear grid")
			}
			m.Grid, m.HasGrid = [2]byte{byte(v[0]), byte(v[1])}, true
		}
		if row, ok := bakalRowByLabel(b, "[monster index2]"); ok {
			v := bakalInts(row.Args)
			if len(v) != 1 || v[0] <= 0 {
				return fmt.Errorf("invalid Bakal second monster")
			}
			m.SecondTemplate = uint32(v[0])
			grid, e := bakalRowInts(b, "[appear grid2]")
			if e != nil || len(grid) != 2 || grid[0] < 0 || grid[0] > 255 || grid[1] < 0 || grid[1] > 255 {
				return fmt.Errorf("invalid Bakal second grid")
			}
			m.SecondGrid = [2]byte{byte(grid[0]), byte(grid[1])}
		}
		if _, dup := r.Monsters[m.Type]; dup {
			return fmt.Errorf("duplicate Bakal monster %s", m.Type)
		}
		r.Monsters[m.Type] = m
	}
	if err := loadBakalBuffDefinitions(a, r); err != nil {
		return err
	}
	cells, err = readBakalCOSTokens(a, BakalSymbolsSource)
	if err != nil {
		return err
	}
	r.Symbols = make(map[string]uint32)
	// Native .symbol is an index: integer ID followed by its label.
	if len(cells)%2 != 0 {
		return fmt.Errorf("incomplete Bakal symbol index")
	}
	for i := 0; i < len(cells); i += 2 {
		if cells[i].Type != 0 || cells[i].Value <= 0 || cells[i+1].Text == "" {
			return fmt.Errorf("invalid Bakal symbol row %d", i/2)
		}
		name := strings.ToUpper(cells[i+1].Text)
		if _, dup := r.Symbols[name]; dup {
			return fmt.Errorf("duplicate Bakal symbol %s", name)
		}
		r.Symbols[name] = uint32(cells[i].Value)
	}
	if len(r.Slots) == 0 || len(r.Monsters) == 0 || len(r.Symbols) == 0 {
		return fmt.Errorf("incomplete native Bakal tables")
	}
	ids := make([]uint32, 0, len(r.Dungeons))
	for _, d := range r.Dungeons {
		ids = append(ids, d.Index)
	}
	dc, err := ImportRuntimeDungeons(a, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := dc.Dungeons[id]; !ok {
			return fmt.Errorf("native Bakal dungeon %d not imported: %v", id, dc.Skipped)
		}
	}
	// Slot alternatives are declared in COS, even when absent from a DGN's
	if err := loadBakalStage(a, r); err != nil {
		return err
	}
	// default room list; retain them in the same source-backed map cache.
	indexScript, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return err
	}
	index, err := ParseIndex(indexScript.Cells)
	if err != nil {
		return err
	}
	paths := map[uint32]string{}
	for _, v := range index {
		paths[v.ID] = v.Path
	}
	for _, slot := range r.Slots {
		for _, id := range slot.Maps {
			if _, ok := dc.Maps[id]; ok {
				continue
			}
			path, ok := paths[id]
			if !ok {
				return fmt.Errorf("native Bakal map %d missing", id)
			}
			s, e := ResolveScript(a, path)
			if e != nil {
				return e
			}
			s.Cells = nil
			dc.Maps[id] = s
		}
	}
	r.DungeonCatalog = &dc
	if err := dc.CloseMapSource(); err != nil {
		return err
	}
	if err := dc.attachMapScripts(a); err != nil {
		return err
	}
	if err := loadBakalRewards(a, r); err != nil {
		return err
	}
	if err := loadBakalWeeklyBidding(a, r); err != nil {
		return err
	}
	return nil
}

func loadBakalRewards(a *pvf.Archive, r *BakalRaidRules) error {
	s, err := ResolveScript(a, "contents/system/raidsystem/raidreward.etc")
	if err != nil {
		return err
	}
	blocks, err := bakalBlocks(s.Cells, "[raid]", "[/raid]")
	if err != nil {
		return err
	}
	for _, b := range blocks {
		d, err := bakalRowInt(b, "[representative dungeon index]")
		if err != nil {
			return err
		}
		if uint32(d) != uint32(r.NormalPhase.SettlementDungeon) {
			continue
		}
		rows := bakalRows(b)
		if len(rows) == 0 || len(bakalInts(rows[0].Args)) != 1 {
			return fmt.Errorf("Bakal reward raid ID missing")
		}
		r.RaidID = bakalInts(rows[0].Args)[0]
		for _, phase := range []struct{ start, end string }{{"[phase]", "[/phase]"}, {"[raid phase of hardmode]", "[/raid phase of hardmode]"}} {
			parts, e := bakalBlocks(b, phase.start, phase.end)
			if e != nil {
				return e
			}
			for _, part := range parts {
				for _, row := range bakalRows(part) {
					if !bakalEq(row.Label, "[state reward]") {
						continue
					}
					names, v := bakalStrings(row.Args), bakalInts(row.Args)
					if len(names) == 1 && names[0] == "bidding_item" {
						if len(v) != 5 || v[1] <= 0 || v[2] <= 0 {
							return fmt.Errorf("invalid native Bakal bidding reward")
						}
						r.Bidding.Items = append(r.Bidding.Items, BakalRewardEntry{Category: names[0], Difficulty: v[0], Weight: uint32(v[1]), Template: uint32(v[2]), Amount: 1})
					}
				}
			}
		}
		phases, e := bakalBlocks(b, "[phase]", "[/phase]")
		if e != nil || len(phases) != 1 {
			return fmt.Errorf("Bakal normal reward phase ambiguous")
		}
		for _, row := range bakalRows(phases[0]) {
			if !bakalEq(row.Label, "[state reward]") {
				continue
			}
			names := bakalStrings(row.Args)
			v := bakalInts(row.Args)
			if len(names) != 1 || len(v) != 5 {
				return fmt.Errorf("invalid Bakal state reward")
			}
			if names[0] != "party_card" && names[0] != "squad_item" {
				continue
			}
			if v[1] <= 0 || v[2] <= 0 {
				return fmt.Errorf("invalid native Bakal reward weight/template")
			}
			r.Rewards = append(r.Rewards, BakalRewardEntry{Category: names[0], Difficulty: v[0], Weight: uint32(v[1]), Template: uint32(v[2]), Amount: 1})
		}
		if len(r.Rewards) == 0 {
			return fmt.Errorf("native normal raid rewards missing")
		}
		return nil
	}
	return fmt.Errorf("native normal raid reward source missing")
}

func (r *BakalRaidRules) SlotForMap(dungeon, mapID uint32) (BakalRaidSlot, bool) {
	for _, slot := range r.Slots {
		if slot.Dungeon != dungeon {
			continue
		}
		for _, id := range slot.Maps {
			if id == mapID {
				return slot, true
			}
		}
	}
	return BakalRaidSlot{}, false
}
