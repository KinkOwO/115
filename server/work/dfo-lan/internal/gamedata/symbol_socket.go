package gamedata

import (
	"dfolan/internal/inventory"
	"fmt"
)

// SymbolSocketLayout uses the original source XUI. Importers must include it
// alongside the private client presentation; no counts are exported to JSON.
func (s *Source) SymbolSocketLayout() (*inventory.SymbolSocketLayout, error) {
	if s == nil || s.archive == nil {
		return nil, fmt.Errorf("symbol socket layout requires PVF")
	}
	if _, exists := s.archive.FindFile(inventory.SymbolEmblemSourceDirectory + "ui/equipemblemwindow.xui"); !exists {
		return nil, nil // Older sources have no independent symbol inventory.
	}
	raw, e := s.archive.ReadRaw(inventory.SymbolEmblemSourceDirectory + "ui/equipemblemwindow.xui")
	if e != nil {
		return nil, e
	}
	layout, e := inventory.ParseSymbolSocketLayout(s.archive.Snapshot().Checksum, raw)
	if e != nil {
		return nil, e
	}
	const mapping = "local/20261008/emblem/symbol_option_map.etc"
	if _, ok := s.archive.FindFile(mapping); !ok {
		return layout, nil
	}
	layout.Rules, e = inventory.ImportSymbolEmblemRules(s.archive)
	if e != nil {
		return nil, e
	}
	layout.FixedTemplates = map[uint32][2]uint32{}
	layout.ResonanceTemplates = map[uint32][2]uint32{}
	tokens, e := s.archive.Tokens(mapping)
	if e != nil {
		return nil, e
	}
	if len(tokens)%4 != 0 {
		return nil, fmt.Errorf("invalid symbol option map")
	}
	for i := 0; i < len(tokens); i += 4 {
		if tokens[i].Type != 3 || tokens[i+1].Type != 0 || tokens[i+2].Type != 0 || tokens[i+3].Type != 0 || tokens[i+1].Value < 0 || tokens[i+2].Value < 2 || tokens[i+3].Value < 2 {
			return nil, fmt.Errorf("invalid symbol option row")
		}
		m := layout.FixedTemplates
		if tokens[i].Text == "[resonance]" {
			m = layout.ResonanceTemplates
		} else if tokens[i].Text != "[fixed]" {
			return nil, fmt.Errorf("unknown symbol option category")
		}
		key := uint32(tokens[i+1].Value)
		if _, ok := m[key]; ok {
			return nil, fmt.Errorf("duplicate symbol option row")
		}
		m[key] = [2]uint32{uint32(tokens[i+2].Value), uint32(tokens[i+3].Value)}
	}
	for id := range layout.Rules.Fixed {
		if layout.FixedTemplates[id][0] == 0 {
			return nil, fmt.Errorf("missing native fixed option")
		}
	}
	for id := range layout.Rules.Resonance {
		if layout.ResonanceTemplates[id][0] == 0 {
			return nil, fmt.Errorf("missing native resonance option")
		}
	}
	return layout, nil
}
