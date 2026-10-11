package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// ApocalypseDoor joins a source object, its shortcut-check action and the COS
// destination table. The destination index must not be inferred from Slot+2.
type ApocalypseDoor struct {
	Object                 uint32
	Slot, Target           byte
	Dungeon                uint32
	ObjectPath, ObjectHash string
	BasicPath, BasicHash   string
	ActionPath, ActionHash string
}

func importApocalypseDoors(a *pvf.Archive) ([]ApocalypseDoor, error) {
	contents, err := ImportLegionContents(a)
	if err != nil {
		return nil, err
	}
	content, ok := contents.Contents["Apocalypse"]
	if !ok || !content.Complete {
		return nil, fmt.Errorf("apocalypse door destination source unavailable")
	}
	listing, err := ReadScript(a, "list/passiveobject.lst")
	if err != nil {
		return nil, err
	}
	// Unrelated index entries include signed sentinel IDs. Validate the actual
	// scoped bindings, rather than making this feature depend on every object
	// in the game. Scope is a canonical source path, not a content ID table.
	scope := path.Join(path.Dir(path.Dir(ApocalypseSource)), "etcpassiveobject") + "/"
	var selected []pvf.Token
	for i, t := range listing.Cells {
		if t.Type != 6 || !strings.HasPrefix(strings.ToLower(strings.ReplaceAll(t.Text, "\\", "/")), scope) {
			continue
		}
		if i == 0 {
			return nil, fmt.Errorf("unpaired apocalypse passive object binding")
		}
		selected = append(selected, listing.Cells[i-1], t)
	}
	rows, err := ParseIndex(selected)
	if err != nil {
		return nil, err
	}
	var doors []ApocalypseDoor
	cache := map[string]ScriptRecord{}
	slots := map[byte]bool{}
	for _, row := range rows {
		obj, ok := cache[row.Path]
		if !ok {
			obj, err = ReadScript(a, row.Path)
			if err != nil {
				return nil, err
			}
			cache[row.Path] = obj
		}
		kind := scriptWarpField(obj.Cells, "[passive object sub type]")
		if len(kind) != 1 || kind[0].Type != 6 || kind[0].Text != "[warp object by formula]" {
			continue
		}
		actions := scriptWarpField(obj.Cells, "[etc action]")
		basicRefs := scriptWarpField(obj.Cells, "[basic action]")
		if len(basicRefs) != 1 || basicRefs[0].Type != 6 || len(actions) == 0 {
			return nil, fmt.Errorf("apocalypse warp object has no initial custom action: %s", obj.Path)
		}
		basic, err := ReadScript(a, path.Join(path.Dir(obj.Path), strings.ReplaceAll(basicRefs[0].Text, "\\", "/")))
		if err != nil {
			return nil, err
		}
		initial, err := apocalypseInitialCustomAction(basic.Cells, len(actions))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", basic.Path, err)
		}
		if actions[initial].Type != 6 {
			return nil, fmt.Errorf("invalid apocalypse custom action reference")
		}
		act, err := ReadScript(a, path.Join(path.Dir(obj.Path), strings.ReplaceAll(actions[initial].Text, "\\", "/")))
		if err != nil {
			return nil, err
		}
		door, err := parseApocalypseDoor(row.ID, obj, act, a.ResolveString, content.Dungeons)
		if err != nil {
			return nil, err
		}
		door.BasicPath, door.BasicHash = basic.Path, basic.SHA256
		if slots[door.Slot] {
			return nil, fmt.Errorf("multiple apocalypse warp objects own shortcut slot %d", door.Slot)
		}
		slots[door.Slot] = true
		doors = append(doors, door)
	}
	if len(doors) == 0 {
		return nil, fmt.Errorf("apocalypse source has no shortcut warp objects")
	}
	sort.Slice(doors, func(i, j int) bool { return doors[i].Slot < doors[j].Slot })
	return doors, nil
}

// Only the unambiguous literal transition in the native basic action is
// supported. Do not assume the shortcut check is always custom action zero.
func apocalypseInitialCustomAction(cells []pvf.Token, count int) (int, error) {
	index, found := 0, false
	for i, cell := range cells {
		if cell.Type != 3 || cell.Text != "[SET CUSTOM ACTION]" {
			continue
		}
		if found || i+1 >= len(cells) || cells[i+1].Type != 0 || int32(cells[i+1].Value) < 0 || uint64(cells[i+1].Value) >= uint64(count) {
			return 0, fmt.Errorf("unsupported or ambiguous initial custom action")
		}
		index, found = int(cells[i+1].Value), true
	}
	if !found {
		return 0, fmt.Errorf("missing initial custom action")
	}
	return index, nil
}

func parseApocalypseDoor(id uint32, obj, act ScriptRecord, resolve func(int) string, destinations []LegionDungeon) (ApocalypseDoor, error) {
	d := ApocalypseDoor{Object: id, ObjectPath: obj.Path, ObjectHash: obj.SHA256, ActionPath: act.Path, ActionHash: act.SHA256}
	blocks := scriptWarpBlocks(obj.Cells, "[create var]")
	if len(blocks) != 1 || len(blocks[0])%2 != 0 || resolve == nil || id == 0 {
		return d, fmt.Errorf("invalid apocalypse warp source variables")
	}
	vars := blocks[0]
	values := map[string]int64{}
	for i := 0; i < len(vars); i += 2 {
		key := vars[i]
		if key.Type != 6 {
			return d, fmt.Errorf("invalid apocalypse warp variable name")
		}
		if key.Text != "o:move_to_dungeon" && key.Text != "o:wrap_dungeon_direct_move" {
			continue
		}
		if _, found := values[key.Text]; found || vars[i+1].Type != 10 {
			return d, fmt.Errorf("ambiguous apocalypse warp formula %s", key.Text)
		}
		v, err := strconv.ParseInt(resolve(int(vars[i+1].Value)), 10, 32)
		if err != nil {
			return d, fmt.Errorf("unsupported nonliteral apocalypse warp formula %s", key.Text)
		}
		values[key.Text] = v
	}
	if values["o:move_to_dungeon"] <= 0 || values["o:wrap_dungeon_direct_move"] != 1 {
		return d, fmt.Errorf("apocalypse shortcut lacks a source direct-move target")
	}
	d.Dungeon = uint32(values["o:move_to_dungeon"])
	matched := 0
	for index, destination := range destinations {
		if destination.Dungeon == d.Dungeon {
			if index >= 6 {
				return d, fmt.Errorf("apocalypse warp destination exceeds native six records")
			}
			d.Target = byte(index)
			matched++
		}
	}
	if matched != 1 {
		return d, fmt.Errorf("apocalypse warp target has no unique COS destination")
	}
	found := false
	for i, token := range act.Cells {
		if token.Type != 10 || i == 0 || act.Cells[i-1].Type != 3 || act.Cells[i-1].Text != "[COMPARE VAR]" {
			continue
		}
		formula := resolve(int(token.Value))
		if !strings.HasPrefix(formula, "isApocalypseShortcutOpened(") || !strings.HasSuffix(formula, ")") {
			continue
		}
		value := strings.TrimSuffix(strings.TrimPrefix(formula, "isApocalypseShortcutOpened("), ")")
		slot, err := strconv.ParseUint(value, 10, 8)
		if err != nil || slot >= 4 || found && d.Slot != byte(slot) {
			return d, fmt.Errorf("apocalypse warp has invalid or ambiguous shortcut owner")
		}
		d.Slot, found = byte(slot), true
	}
	if !found {
		return d, fmt.Errorf("apocalypse warp lacks its source shortcut check")
	}
	return d, nil
}
