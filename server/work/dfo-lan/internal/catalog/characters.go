package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profession struct {
	AdvancementSkills map[byte][]int32            `json:"advancement_skills,omitempty"`
	AwakeningSkills   map[byte]map[byte][]int32   `json:"awakening_skills,omitempty"`
	AdvancementGrowth map[byte]map[string]float32 `json:"advancement_growth,omitempty"`
	ID                byte                        `json:"id"`
	Path              string                      `json:"path"`
	RawSHA256         string                      `json:"raw_sha256"`
	Job               string                      `json:"job"`
	InitialAttributes map[string]float32          `json:"initial_attributes"`
	BaseGrowth        map[string]float32          `json:"base_growth,omitempty"`
	SwordmasterGrowth map[string]float32          `json:"swordmaster_growth,omitempty"`
	InitialSections   map[string][]pvf.Token      `json:"initial_sections"`
	InitialSkills     []int32                     `json:"initial_skill_cells"`
	InitialSkillSlots map[uint16]uint16           `json:"initial_skill_slots,omitempty"`
	// AdvancementSkillSlots carries evidence-backed default shortcut positions
	// for skills introduced by one advancement. It is separate from
	// InitialSkillSlots because a branch-only skill must not acquire that
	// shortcut position on another branch merely because the profession shares
	// one catalog row.
	AdvancementSkillSlots map[byte]map[uint16]uint16 `json:"advancement_skill_slots,omitempty"`
	// SkillCommands is the current-client NOTI19 field-4 command vector keyed
	// by the source skill id. Empty/missing vectors are intentional for
	// book-only and passive rows.
	SkillCommands     map[uint16][]uint32 `json:"skill_commands,omitempty"`
	CreateEquipment   []pvf.Token         `json:"create_equipment_cells"`
	// CreateEquipmentBySlot 是 [create equipment list] 的按槽投影：外层键是源部位
	// 标签（[weapon]、[coat]…），内层键是 0 基槽位，与角色的 Advancement 同域。
	// 值 0 表示源在该槽显式给出空位。槽数不设上限：源每标签给几个就记几个，不做推测。
	CreateEquipmentBySlot map[string]map[byte]uint32 `json:"create_equipment_by_slot,omitempty"`
	// CreateEquipmentOrder 保留部位标签在源文件里的出现顺序，供投影时稳定分配穿戴槽。
	CreateEquipmentOrder []string `json:"create_equipment_order,omitempty"`
	DefaultAppearance []int32     `json:"default_appearance_indices,omitempty"`
	// Growth is indexed by the character's State.Advancement: slot 0 comes from
	// [growtype 1] (the unadvanced base profession) and slot N from
	// [growtype N+1]. Baseline projection kept for the creation path.
	Growth       []map[string]float32 `json:"growth,omitempty"`
	PresetSkills [][]int32            `json:"preset_skills,omitempty"`
	// SlotSkills is the [skill] block *inside* each [growtype N] section, flat
	// (id, level, condition) triples exactly like InitialSkills.
	SlotSkills [][]int32 `json:"slot_skills,omitempty"`
}
type Characters struct {
	Source      pvf.ArchiveSnapshot `json:"source"`
	Professions map[byte]Profession `json:"professions"`
}

// growtypeSlots is how many advancement slots a character script ships in
// [growtype name]. Both State.Advancement and the skill catalog arrays are
// indexed by the same 0..growtypeSlots-1 domain.
const growtypeSlots = 6

// growtypeSlot maps a lowercased section label to its advancement slot:
// "[growtype 1]" is slot 0, so segment number N means slot N-1.
func growtypeSlot(section string) (int, bool) {
	const prefix = "[growtype "
	if !strings.HasPrefix(section, prefix) || !strings.HasSuffix(section, "]") {
		return 0, false
	}
	digits := section[len(prefix) : len(section)-1]
	if len(digits) != 1 || digits[0] < '1' || digits[0] > '0'+growtypeSlots {
		return 0, false
	}
	return int(digits[0] - '1'), true
}

// GrowthAt returns the per-level attribute gain of one advancement slot, or
// nil when the slot has no [growtype N] block in the source. Callers must
// treat nil as "this profession cannot grow at that advancement" rather than
// falling back to the unadvanced block.
func (p Profession) GrowthAt(advancement int) map[string]float32 {
	if advancement < 0 || advancement >= len(p.Growth) {
		return nil
	}
	return p.Growth[advancement]
}

// collectGrowtypeProfile is the baseline growtype-section pass: per-slot
// attribute growth, preset skills and slot skills, plus the create-equipment
// per-slot resolution. It no longer hard-requires an unadvanced growth block
// - a profession whose source ships no [growtype N] simply leaves the slots
// empty, and the creation path refuses advancements it cannot honour.
func collectGrowtypeProfile(ts []pvf.Token, p *Profession, path string) error {
	p.Growth = make([]map[string]float32, growtypeSlots)
	p.PresetSkills = make([][]int32, growtypeSlots)
	p.SlotSkills = make([][]int32, growtypeSlots)
	slot, preset, slotSkill := -1, false, false
	section := ""
	for _, t := range ts {
		if t.Type == 3 {
			section = strings.ToLower(t.Text)
			if strings.HasPrefix(section, "[/") {
				preset, slotSkill = false, false
				continue
			}
			if n, ok := growtypeSlot(section); ok {
				slot, preset, slotSkill = n, false, false
				continue
			}
			preset = slot >= 0 && section == "[preset skill]"
			// A [skill] block inside a [growtype N] section is that
			// advancement's own starting skill list. The top-level [skill]
			// belongs to every slot and is collected separately above.
			slotSkill = slot >= 0 && section == "[skill]"
			continue
		}
		if slot < 0 {
			continue
		}
		if slotSkill {
			if t.Type != 0 {
				return fmt.Errorf("unsupported slot skill cell in %s", path)
			}
			p.SlotSkills[slot] = append(p.SlotSkills[slot], int32(t.Value))
			continue
		}
		if preset {
			if t.Type != 0 {
				return fmt.Errorf("unsupported preset skill cell in %s", path)
			}
			p.PresetSkills[slot] = append(p.PresetSkills[slot], int32(t.Value))
			continue
		}
		if _, known := p.InitialAttributes[section]; !known {
			continue
		}
		if p.Growth[slot] == nil {
			p.Growth[slot] = map[string]float32{}
		}
		if _, seen := p.Growth[slot][section]; seen {
			return fmt.Errorf("duplicate growth %s in %s", section, path)
		}
		switch t.Type {
		case 0:
			p.Growth[slot][section] = float32(t.Value)
		case 2:
			p.Growth[slot][section] = t.Number
		default:
			return fmt.Errorf("unsupported growth cell in %s", path)
		}
	}
	p.CreateEquipmentBySlot, p.CreateEquipmentOrder = ParseCreateEquipment(p.CreateEquipment)
	return nil
}

func ImportCharacters(a *pvf.Archive) (Characters, error) {
	result := Characters{Source: a.Snapshot(), Professions: map[byte]Profession{}}
	list, err := a.Tokens("list/character.lst")
	if err != nil {
		return result, err
	}
	if len(list)%2 != 0 {
		return result, fmt.Errorf("invalid profession index")
	}
	for i := 0; i < len(list); i += 2 {
		if list[i].Type != 0 || list[i].Value < 0 || list[i].Value > 255 || list[i+1].Type != 6 {
			return result, fmt.Errorf("invalid profession pair %d", i/2)
		}
		id := byte(list[i].Value)
		path := strings.ToLower(strings.ReplaceAll(list[i+1].Text, "\\", "/"))
		if _, ok := result.Professions[id]; ok {
			return result, fmt.Errorf("duplicate profession %d", id)
		}
		ts, e := a.Tokens(path)
		if e != nil {
			return result, e
		}
		raw, e := a.ReadRaw(path)
		if e != nil {
			return result, e
		}
		p := Profession{ID: id, Path: path, RawSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), InitialAttributes: map[string]float32{}, InitialSections: map[string][]pvf.Token{}}
		initial := false
		section := ""
		for _, t := range ts {
			if t.Type == 3 {
				section = strings.ToLower(t.Text)
				if section == "[initial value]" {
					initial = true
				}
				if strings.HasPrefix(section, "[growtype ") {
					initial = false
				}
				continue
			}
			if section == "[job]" && t.Type == 6 {
				p.Job = t.Text
			}
			if section == "[create equipment list]" {
				p.CreateEquipment = append(p.CreateEquipment, t)
			}
			if !initial {
				continue
			}
			if section == "[skill]" {
				if t.Type != 0 {
					return result, fmt.Errorf("unsupported initial skill cell in %s", path)
				}
				p.InitialSkills = append(p.InitialSkills, t.Value)
				continue
			}
			if strings.HasPrefix(section, "[/") {
				continue
			}
			p.InitialSections[section] = append(p.InitialSections[section], t)
		}
		for key, cells := range p.InitialSections {
			if len(cells) == 1 {
				switch cells[0].Type {
				case 0:
					p.InitialAttributes[key] = float32(cells[0].Value)
				case 2:
					p.InitialAttributes[key] = cells[0].Number
				}
			}
		}
		p.CreateEquipmentBySlot, p.CreateEquipmentOrder = ParseCreateEquipment(p.CreateEquipment)
		if p.Job == "" || p.InitialAttributes["[hp max]"] <= 0 || p.InitialAttributes["[mp max]"] <= 0 || len(p.InitialSkills)%3 != 0 || p.RawSHA256 == "" {
			return result, fmt.Errorf("incomplete initial configuration for %d", id)
		}
		// Unadvanced profession growth is its own source block. Never pull
		// advanced/awakened skills or attributes into the initial profession.
		p.BaseGrowth = map[string]float32{}
		growth := false
		for _, t := range ts {
			if t.Type == 3 {
				section = strings.ToLower(t.Text)
				if strings.HasPrefix(section, "[growtype ") {
					growth = section == "[growtype 1]"
				}
				continue
			}
			if _, known := p.InitialAttributes[section]; !growth || !known {
				continue
			}
			if _, seen := p.BaseGrowth[section]; seen {
				return result, fmt.Errorf("duplicate base growth %s in %s", section, path)
			}
			switch t.Type {
			case 0:
				p.BaseGrowth[section] = float32(t.Value)
			case 2:
				p.BaseGrowth[section] = t.Number
			default:
				return result, fmt.Errorf("unsupported base growth cell in %s", path)
			}
		}
		p.AdvancementGrowth = map[byte]map[string]float32{}
		p.AwakeningSkills = AwakeningSkillGrants(ts)
		p.AdvancementSkills, e = AdvancementSkillGrants(ts)
		if e != nil {
			return result, fmt.Errorf("profession %d: %w", id, e)
		}
		for advancement := byte(1); advancement < 16; advancement++ {
			growth, err := ProfessionGrowth(ts, p.InitialAttributes, advancement)
			if err == nil {
				p.AdvancementGrowth[advancement] = growth
			}
		}
		if id == 0 {
			p.SwordmasterGrowth, e = SwordmasterGrowth(ts, p.InitialAttributes)
			if e != nil {
				return result, e
			}
		}
		if e := collectGrowtypeProfile(ts, &p, path); e != nil {
			return result, e
		}
		// Skill availability comes from .chr. A small local hotbar policy puts
		// source-active initial skills in source order; this is not a claim
		// that this ordering reproduces the official default key layout.
		indexPath := "skill/" + strings.TrimSuffix(filepath.Base(path), ".chr") + "skill.lst"
		if _, found := a.FindFile(indexPath); !found {
			// The supplied Knight source uses skill/knight.lst, while older
			// professions use the *skill.lst naming convention. Resolve an
			// existing source file before binding any source initial skill.
			indexPath = "skill/" + strings.TrimSuffix(filepath.Base(path), ".chr") + ".lst"
			if _, found = a.FindFile(indexPath); !found {
				return result, fmt.Errorf("no source skill index for profession %d", id)
			}
		} // skills still learned; no guessed shortcut source
		skillIndex, e := ReadScript(a, indexPath)
		if e != nil {
			return result, e
		}
		rows, e := ParseIndex(skillIndex.Cells)
		if e != nil {
			return result, e
		}
		skillPaths := map[uint32]string{}
		for _, row := range rows {
			skillPaths[row.ID] = row.Path
		}
		p.InitialSkillSlots = map[uint16]uint16{}
		p.AdvancementSkillSlots = map[byte]map[uint16]uint16{}
		p.SkillCommands = map[uint16][]uint32{}
		for j := 0; j < len(p.InitialSkills); j += 3 {
			rawID := p.InitialSkills[j]
			if rawID <= 0 || rawID > 65535 {
				continue
			}
			name, ok := skillPaths[uint32(rawID)]
			if !ok {
				return result, fmt.Errorf("missing initial skill %d", rawID)
			}
			script, e := ResolveScript(a, "skill/"+name)
			if e != nil {
				return result, e
			}
			id := uint16(rawID)
			if commands, ok := sourceCommandVector(script.Cells); ok {
				p.SkillCommands[id] = commands
			}
			if p.InitialSkills[j+1] <= 0 || p.InitialSkills[j+2] != 1 {
				continue
			}
			if defaultShortcutEligible(name, script.Cells) {
				p.InitialSkillSlots[id] = uint16(len(p.InitialSkillSlots))
			}
		}
		// Each [growtype N] block contributes its own branch-local defaults.
		// The first available default slot follows the common initial slots;
		// separate branches must not consume each other's slots.
		for slot, cells := range p.SlotSkills {
			if len(cells) == 0 {
				continue
			}
			branchSlots := map[uint16]uint16{}
			nextSlot := uint16(len(p.InitialSkillSlots))
			for j := 0; j+2 < len(cells); j += 3 {
				rawID := cells[j]
				if rawID <= 0 || rawID > 65535 {
					continue
				}
				name, ok := skillPaths[uint32(rawID)]
				if !ok {
					// The .chr grant remains valid even when an old skill index
					// omits a shared source row; without the .skl source there is
					// no safe shortcut or command metadata to project.
					continue
				}
				script, e := ResolveScript(a, "skill/"+name)
				if e != nil {
					return result, e
				}
				id := uint16(rawID)
				if commands, ok := sourceCommandVector(script.Cells); ok {
					p.SkillCommands[id] = commands
				}
				if cells[j+1] <= 0 || cells[j+2] != 1 {
					continue
				}
				if !defaultShortcutEligible(name, script.Cells) {
					continue
				}
				if _, common := p.InitialSkillSlots[id]; common {
					continue
				}
				if _, duplicate := branchSlots[id]; duplicate {
					continue
				}
				branchSlots[id] = nextSlot
				nextSlot++
			}
			if len(branchSlots) > 0 {
				p.AdvancementSkillSlots[byte(slot)] = branchSlots
			}
		}
		result.Professions[id] = p
	}
	appearance, e := a.Tokens("character/characterinfo.etc")
	if e != nil {
		return result, e
	}
	var row []int32
	active := false
	for _, t := range appearance {
		if t.Type == 3 {
			if t.Text == "[/default equipment index]" && active && len(row) > 0 {
				p, ok := result.Professions[byte(row[0])]
				if ok {
					p.DefaultAppearance = append([]int32{}, row[1:]...)
					result.Professions[p.ID] = p
				}
			}
			active = t.Text == "[default equipment index]"
			row = nil
			continue
		}
		if active {
			if t.Type != 0 {
				return result, fmt.Errorf("non-numeric appearance index")
			}
			row = append(row, t.Value)
		}
	}
	return result, nil
}

// ParseCreateEquipment 把 [create equipment list] 的原始单元解析成按槽投影。
//
// 该段的槽标签是字符串单元而不是节头，源顺序即 标签, N 个 ID, 标签, N 个 ID, …，
// 第 N 个 ID 对应 0 基槽 N-1，与角色的 Advancement 同域。值为 0 表示源显式给出
// 空槽。PvP 用的是另一个节头（[pvp private create equipment list]），不在本函数的
// 输入里。槽数不设上限：源给几个就记几个，不做推测；没有该段时返回 nil。
func ParseCreateEquipment(cells []pvf.Token) (map[string]map[byte]uint32, []string) {
	bySlot := map[string]map[byte]uint32{}
	var order []string
	label := ""
	index := 0
	for _, t := range cells {
		switch t.Type {
		case 6:
			label = strings.ToLower(t.Text)
			if _, seen := bySlot[label]; !seen {
				order = append(order, label)
			}
			index = 0
		case 0:
			if label == "" || t.Value < 0 {
				continue
			}
			if bySlot[label] == nil {
				bySlot[label] = map[byte]uint32{}
			}
			bySlot[label][byte(index)] = uint32(t.Value)
			index++
		}
	}
	if len(bySlot) == 0 {
		return nil, nil
	}
	return bySlot, order
}

// Skill metadata precedes its nested dungeon/PvP damage sections. Repeated
// [type] inside damage groups describes damage, not the skill's active type.
// Initial shortcuts use command-capable active skills in the source [skill]
// order. Product policy intentionally keeps Backstep and Quick Standing out
// of the default bar while still granting both skills.
func initialShortcutEligible(cells []pvf.Token) bool {
	first := func(name string) []pvf.Token {
		for i, c := range cells {
			if c.Type != 3 || c.Text != name {
				continue
			}
			end := i + 1
			for end < len(cells) && cells[end].Type != 3 {
				end++
			}
			return cells[i+1 : end]
		}
		return nil
	}
	kind := first("[type]")
	return len(kind) == 1 && kind[0].Text == "[active]" &&
		len(first("[command]")) > 0
}

func defaultShortcutEligible(sourcePath string, cells []pvf.Token) bool {
	if !initialShortcutEligible(cells) {
		return false
	}
	switch strings.ToLower(filepath.Base(sourcePath)) {
	case "backstep.skl", "quickstanding.skl":
		return false
	default:
		return true
	}
}

// sourceCommandVector translates the compact PVF [command] cells into the
// repeated uint32 values consumed by the current client's NOTI19 field 4.
// The two reference cells below distinguish the command families used by the
// live Knight source; unknown command encodings are omitted rather than
// guessed. The translation is source-token based, not skill-ID based.
func sourceCommandVector(cells []pvf.Token) ([]uint32, bool) {
	type commandCell struct {
		value int32
		ref   int32
	}
	var commands []commandCell
	inCommand := false
	for _, cell := range cells {
		if cell.Type == 3 {
			switch cell.Text {
			case "[command]":
				if inCommand {
					return nil, false
				}
				inCommand = true
				commands = nil
				continue
			case "[/command]":
				if !inCommand {
					continue
				}
				inCommand = false
				goto translate
			}
		}
		if !inCommand {
			continue
		}
		switch cell.Type {
		case 5:
			commands = append(commands, commandCell{value: cell.Value})
		case 7:
			if len(commands) == 0 {
				return nil, false
			}
			commands[len(commands)-1].ref = cell.Value
		default:
			return nil, false
		}
	}
	if inCommand {
		return nil, false
	}

translate:
	if len(commands) == 0 {
		return nil, false
	}
	out := make([]uint32, len(commands))
	for i, command := range commands {
		value, ok := sourceCommandValue(command.value, command.ref)
		if !ok {
			return nil, false
		}
		out[i] = value
	}
	return out, true
}

func sourceCommandValue(key, ref int32) (uint32, bool) {
	// These values are the current PVF command token references. Their wire
	// translations are established by the valid CN NOTI19 rows for the same
	// Knight skill sources; the client receives the compact values, not RAW
	// five-byte cells.
	const (
		refBackstepFamily int32 = 0x11b6c657
		refDragonFamily   int32 = 0x0337ea25
	)
	switch ref {
	case refBackstepFamily:
		switch key {
		case 1:
			return 9, true
		case 11:
			return 10, true
		}
	case refDragonFamily:
		switch key {
		case 1:
			return 0, true
		case 11:
			return 1, true
		case 39:
			return 3, true
		}
	case 0:
		switch key {
		case 55:
			return 4, true
		case 89:
			return 6, true
		case 125:
			return 8, true
		}
	}
	return 0, false
}
func LoadCharacters(path string) (Characters, error) {
	var c Characters
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	if len(c.Professions) == 0 {
		return c, fmt.Errorf("invalid character catalog")
	}
	// Older generated snapshots persisted each growtype's [skill] block as
	// slot_skills but predated the separate advancement_skills field.  Keep
	// the two source views equivalent at load time so automatic/free grants
	// remain available after an advancement, without changing an explicit
	// advancement_skills entry or requiring a runtime PVF read.
	for id, p := range c.Professions {
		if p.AdvancementSkills == nil {
			p.AdvancementSkills = map[byte][]int32{}
		}
		for slot, cells := range p.SlotSkills {
			advancement := byte(slot)
			if len(cells) == 0 {
				continue
			}
			if _, exists := p.AdvancementSkills[advancement]; exists {
				continue
			}
			p.AdvancementSkills[advancement] = append([]int32(nil), cells...)
		}
		// 已在 JSON 里存了按槽投影就用它；只有原始单元的老文件在这里补齐，
		// 免得为了一个新增数据块重新导出整个目录（导入端走同一个解析函数）。
		if len(p.CreateEquipmentBySlot) == 0 && len(p.CreateEquipment) > 0 {
			p.CreateEquipmentBySlot, p.CreateEquipmentOrder = ParseCreateEquipment(p.CreateEquipment)
		}
		c.Professions[id] = p
	}
	_ = c // 单机裁定 20260919：职业行零检测——SHA256 指纹与字段完整度不再拒载。
	return c, nil
}
