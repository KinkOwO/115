package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type FameItemImport struct{ rules *FameRules }

func NewFameItemImport(a *pvf.Archive) (*FameItemImport, error) {
	if a == nil {
		return nil, fmt.Errorf("fame import requires PVF")
	}
	r, err := importFameRules(a, catalog.ItemIndex{Source: a.Snapshot()}, false)
	if err != nil {
		return nil, err
	}
	return &FameItemImport{rules: r}, nil
}

func (b *FameItemImport) Consume(s catalog.ItemScript) error {
	if s.Item.Kind != "stackable" {
		return nil
	}
	if !s.Exact {
		return fmt.Errorf("missing fame source item %d", s.Item.ID)
	}
	fields := fameSourceSections(s.Cells)
	direct, table, additional := fameSourceField(fields, "[fame value]"), fameSourceField(fields, "[fame table]"), fameSourceField(fields, "[add fame value]")
	if len(direct) == 0 && len(table) == 0 && len(additional) == 0 {
		return nil
	}
	var item fameSourceValue
	if len(direct) > 0 {
		values, err := fameSourceNumbers(direct)
		if err != nil {
			return err
		}
		item.Value = values[0]
	}
	if len(additional) > 0 {
		values, err := fameSourceNumbers(additional)
		if err != nil {
			return err
		}
		item.Additional = values[0]
	}
	if len(table) > 0 {
		if len(table) != 2 || table[0].Type != 6 || table[1].Type != 0 {
			return fmt.Errorf("invalid item fame table %d", s.Item.ID)
		}
		item.Table, item.Index = table[0].Text, int(table[1].Value)
	}
	b.rules.Items[s.Item.ID] = item
	b.rules.Sources[s.Item.Path] = s.SHA256()
	return nil
}

func (b *FameItemImport) Finish() (*FameRules, error) { return NewFameRules(*b.rules) }
