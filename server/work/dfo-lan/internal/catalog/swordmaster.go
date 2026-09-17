package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strings"
)

// Swordmaster is growtype index 1; the .chr sections are one-based.
// Only numeric attributes already understood by the entry encoder are projected.
func SwordmasterGrowth(tokens []pvf.Token, attributes map[string]float32) (map[string]float32, error) {
	return ProfessionGrowth(tokens, attributes, 1)
}

func ProfessionGrowth(tokens []pvf.Token, attributes map[string]float32, advancement byte) (map[string]float32, error) {
	result := map[string]float32{}
	active, section := false, ""
	for _, t := range tokens {
		if t.Type == 3 {
			section = strings.ToLower(t.Text)
			if strings.HasPrefix(section, "[growtype ") {
				active = section == fmt.Sprintf("[growtype %d]", int(advancement)+1)
			}
			if strings.HasPrefix(section, "[awakening") {
				active = false
			}
			continue
		}
		if _, ok := attributes[section]; !active || !ok {
			continue
		}
		if _, duplicate := result[section]; duplicate {
			return nil, fmt.Errorf("duplicate swordmaster growth %s", section)
		}
		var v float32
		switch t.Type {
		case 0:
			v = float32(t.Value)
		case 2:
			v = t.Number
		default:
			return nil, fmt.Errorf("invalid swordmaster growth %s", section)
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 {
			return nil, fmt.Errorf("invalid swordmaster growth value")
		}
		result[section] = v
	}
	if result["[hp max]"] <= 0 || result["[mp max]"] <= 0 {
		return nil, fmt.Errorf("missing swordmaster source growth")
	}
	return result, nil
}
