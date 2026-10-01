package adventure

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

func ImportRules(a *pvf.Archive, index catalog.ItemIndex) (*Rules, error) {
	if a == nil || index.Source.Checksum == "" || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("adventure PVF/index source mismatch")
	}
	const sourcePath = "etc/adventurersystem/adventurersystem2018.etc"
	script, err := catalog.ReadScript(a, sourcePath)
	if err != nil {
		return nil, err
	}
	r := Rules{SourceChecksum: a.Snapshot().Checksum, Experience: map[uint32]uint64{}, Shops: map[byte]Shop{}, Items: map[uint32]Item{}, Sources: map[string]string{sourcePath: script.SHA256}}
	r.MaxLevel, err = ruleUint(script.Cells, "[max adventure level]")
	if err != nil {
		return nil, err
	}
	rate := ruleSection(script.Cells, "[adventure exp rate]")
	if len(rate) != 1 || rate[0].Type != 2 {
		return nil, fmt.Errorf("invalid adventure experience rate cell")
	}
	r.ExpRate = math.Float32frombits(uint32(rate[0].Value))
	if math.IsNaN(float64(r.ExpRate)) || math.IsInf(float64(r.ExpRate), 0) {
		return nil, fmt.Errorf("non-finite adventure rate")
	}
	exp, err := ruleNumbers(script.Cells, "[adventure exp table]")
	if err != nil {
		return nil, err
	}
	if len(exp) == 0 || len(exp)%2 != 0 {
		return nil, fmt.Errorf("invalid adventure experience pairs")
	}
	for i := 0; i < len(exp); i += 2 {
		if exp[i] <= 0 || exp[i] > math.MaxUint32 || exp[i+1] < 0 {
			return nil, fmt.Errorf("invalid adventure experience row")
		}
		id := uint32(exp[i])
		if _, ok := r.Experience[id]; ok {
			return nil, fmt.Errorf("duplicate adventure level %d", id)
		}
		r.Experience[id] = uint64(exp[i+1])
	}
	for _, block := range ruleBlocks(script.Cells, "[shop]") {
		category, err := ruleUint(block, "[index]")
		if err != nil {
			return nil, err
		}
		if category > math.MaxUint8 {
			return nil, fmt.Errorf("invalid adventure shop index")
		}
		if _, ok := r.Shops[byte(category)]; ok {
			return nil, fmt.Errorf("duplicate adventure shop %d", category)
		}
		var shop Shop
		shop.MaxPoints, err = ruleUint(block, "[max shop point]")
		if err != nil {
			return nil, err
		}
		points, err := ruleNumbers(block, "[exp to get shop point]")
		if err != nil {
			return nil, err
		}
		shop.ExpPoints = []uint64{}
		for _, v := range points {
			if v < 0 {
				return nil, fmt.Errorf("negative adventure shop experience")
			}
			shop.ExpPoints = append(shop.ExpPoints, uint64(v))
		}
		for _, t := range block {
			shop.ResetPoints = shop.ResetPoints || t.Text == "[reset point]"
		}
		rows, err := ruleNumbers(block, "[adventurer shop purchase info]")
		if err != nil {
			return nil, err
		}
		if len(rows)%5 != 0 {
			return nil, fmt.Errorf("unpaired adventure shop rows")
		}
		for i := 0; i < len(rows); i += 5 {
			for j := 0; j < 4; j++ {
				if rows[i+j] < 0 || rows[i+j] > math.MaxUint32 {
					return nil, fmt.Errorf("invalid adventure shop number")
				}
			}
			item := ShopItem{Template: uint32(rows[i]), Level: uint32(rows[i+1]), Price: uint32(rows[i+2]), Limit: uint32(rows[i+3]), Reset: int(rows[i+4])}
			shop.Items = append(shop.Items, item)
			if _, ok := r.Items[item.Template]; !ok {
				def, err := readRuleItem(a, index, item.Template)
				if err != nil {
					return nil, err
				}
				r.Items[item.Template] = def
			}
		}
		r.Shops[byte(category)] = shop
	}
	return NewRules(r)
}
