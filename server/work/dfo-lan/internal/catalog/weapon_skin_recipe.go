package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"strings"
)

// WeaponSkinRecipe is the native forge recipe, not an alternate item table.
type WeaponSkinRecipe struct {
	Mode          uint32
	Material      uint32
	Count         uint32
	Product       uint32
	Attach        string
	StackableType string
}

func ImportWeaponSkinRecipes(a *pvf.Archive) (map[uint32]WeaponSkinRecipe, error) {
	if a == nil {
		return nil, fmt.Errorf("weapon shape recipes require PVF")
	}
	cells, err := a.Tokens("skin/weaponskincontainer.etc")
	if err != nil {
		return nil, err
	}
	index, e := ResolveScript(a, "list/stackable.lst")
	if e != nil {
		return nil, e
	}
	rows, e := ParseIndex(index.Cells)
	if e != nil {
		return nil, e
	}
	paths := map[uint32]string{}
	for _, row := range rows {
		name := row.Path
		if !strings.HasPrefix(name, "stackable/") {
			name = path.Join("stackable", name)
		}
		paths[row.ID] = name
	}
	readItem := func(id uint32) ([]pvf.Token, error) {
		name, ok := paths[id]
		if !ok {
			return nil, fmt.Errorf("weapon shape recipe item %d missing", id)
		}
		return scriptTokens(a, name)
	}
	out, e := parseWeaponSkinRecipes(cells, readItem)
	if e != nil {
		return nil, e
	}
	skins, e := skinList(a)
	if e != nil {
		return nil, e
	}
	for _, r := range out {
		product, e := readItem(r.Product)
		if e != nil {
			return nil, e
		}
		skin, e := scriptTokens(a, skins[skinStorageActionParam(product)])
		if e != nil {
			return nil, e
		}
		if skinTypeName(skin) != "weapon skin" {
			return nil, fmt.Errorf("recipe product is not a weapon shape")
		}
		material, e := readItem(r.Material)
		if e != nil {
			return nil, e
		}
		kind := sectionCells(material, "[stackable type]")
		if len(kind) == 0 || kind[0].Text != "[material]" {
			return nil, fmt.Errorf("weapon shape recipe material is not a material")
		}
	}
	return out, nil
}

func parseWeaponSkinRecipes(cells []pvf.Token, item func(uint32) ([]pvf.Token, error)) (map[uint32]WeaponSkinRecipe, error) {
	out := map[uint32]WeaponSkinRecipe{}
	inside := false
	for i := 0; i < len(cells); i++ {
		t := cells[i]
		if t.Type != 3 {
			continue
		}
		if t.Text == "[make enchanted skin]" {
			inside = true
			continue
		}
		if t.Text == "[/make enchanted skin]" {
			inside = false
			continue
		}
		if !inside || t.Text != "[recipe]" {
			continue
		}
		if i+1 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value <= 0 {
			return nil, fmt.Errorf("invalid weapon shape recipe mode")
		}
		r := WeaponSkinRecipe{Mode: uint32(cells[i+1].Value)}
		i += 2
		closed := false
		for ; i < len(cells); i++ {
			if cells[i].Type != 3 {
				return nil, fmt.Errorf("unexpected weapon shape recipe token")
			}
			switch cells[i].Text {
			case "[/recipe]":
				closed = true
			case "[enchanted result]":
				if r.Product != 0 || i+1 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value <= 0 {
					return nil, fmt.Errorf("invalid weapon shape result")
				}
				i++
				r.Product = uint32(cells[i].Value)
			case "[material]":
				if r.Material != 0 || i+4 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value <= 0 || cells[i+2].Text != "fixed" || cells[i+3].Type != 0 || cells[i+3].Value <= 0 || cells[i+4].Text != "[/material]" {
					return nil, fmt.Errorf("unsupported weapon shape material rule")
				}
				r.Material, r.Count = uint32(cells[i+1].Value), uint32(cells[i+3].Value)
				i += 4
			default:
				return nil, fmt.Errorf("unsupported weapon shape recipe field %q", cells[i].Text)
			}
			if closed {
				break
			}
		}
		if !closed || r.Product == 0 || r.Material == 0 || out[r.Mode].Mode != 0 {
			return nil, fmt.Errorf("incomplete or repeated weapon shape recipe")
		}
		product, e := item(r.Product)
		if e != nil {
			return nil, e
		}
		if skinStorageActionParam(product) == 0 {
			return nil, fmt.Errorf("weapon shape product has no registration action")
		}
		for _, c := range sectionCells(product, "[attach type]") {
			if c.Type == 6 {
				r.Attach = c.Text
				break
			}
		}
		for _, c := range sectionCells(product, "[stackable type]") {
			if c.Type == 6 {
				r.StackableType = c.Text
				break
			}
		}
		if r.Attach != "[free]" && r.Attach != "[trade]" && r.Attach != "[account]" && r.Attach != "[character]" {
			return nil, fmt.Errorf("unsupported weapon shape product attachment")
		}
		if r.StackableType == "" {
			return nil, fmt.Errorf("weapon shape product has no bag type")
		}
		out[r.Mode] = r
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("weapon shape recipes are empty")
	}
	return out, nil
}
